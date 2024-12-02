package service

import (
	"context"
	"time"

	"github.com/catalystgo/logger/logger"
	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func (rf *RaftState) processRaftRPC() {
	defer rf.wg.Done()

	for {
		select {
		case req := <-rf.appendEntriesChan:
			rf.resetElectionTimer()
			res, err := rf.appendEntries(req.ctx, req.req)
			if err != nil {
				req.err <- err
				continue
			}
			req.res <- res
		case req := <-rf.requestVoteChan:
			rf.resetElectionTimer()
			res, err := rf.requestVote(req.ctx, req.req)
			if err != nil {
				req.err <- err
				continue
			}
			req.res <- res
		case <-rf.ctx.Done():
			return
		}
	}
}

func (rf *RaftState) appendEntries(ctx context.Context, req *desc.AppendEntriesRequest) (*desc.AppendEntriesResponse, error) {
	var (
		term     = rf.state.GetTerm()
		response = &desc.AppendEntriesResponse{
			Term:    term,
			Success: false,
		}

		reqTerm         = req.GetTerm()
		reqPrevLogTerm  = req.GetPrevLogTerm()
		reqPrevLogIndex = req.GetPrevLogIndex()
	)

	lastEntry, err := rf.getLastEntry(ctx)
	if err != nil {
		logger.ErrorKV(ctx, "append entries last", "error", err)
		return nil, err
	}

	response.LastLogIndex = lastEntry.Index

	// Reply false if term < currentTerm
	if reqTerm < term {
		return response, nil
	}

	if reqTerm > term {
		rf.sendStateUpdate(core.StateUpdate{
			Type: core.StateUpdateTypeTerm,
			Term: reqTerm,
		})
	}

	entry, err := rf.entryStore.At(ctx, reqPrevLogIndex)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(ctx, "append entries at", "error", err)
			return nil, err
		}

		// If entry is not found check if it's the initial entry
		if reqPrevLogIndex == 0 {
			entry = &core.Entry{}
		}
	}

	// Reply false if log doesn’t contain an entry at prevLogIndex
	if entry == nil {
		return response, nil
	}

	// Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
	if entry.Term != reqPrevLogTerm {
		return response, nil
	}

	entries := make([]*core.Entry, len(req.GetEntries()))
	for i, e := range req.GetEntries() {
		entries[i] = &core.Entry{
			Term:  e.GetTerm(),
			Index: e.GetIndex(),
			Data:  e.GetData(),
		}
	}

	err = rf.entryStore.AppendEntries(ctx, entries...)
	if err != nil {
		logger.ErrorKV(ctx, "append entries", "error", err)
		return nil, err
	}

	if req.GetLeaderCommit() > rf.state.GetCommitIndex() {
		commitIndex := req.GetLeaderCommit()

		// Entries might be empty if the leader is sending a heartbeat only
		if len(entries) > 0 {
			commitIndex = min(commitIndex, entries[len(entries)-1].Index)
		}

		rf.sendStateUpdate(core.StateUpdate{
			Type:        core.StateUpdateTypeCommitIndex,
			CommitIndex: commitIndex,
		})
	}

	state := rf.state.GetState()
	if state == core.Leader || state == core.Candidate {
		rf.sendStateUpdate(core.StateUpdate{
			Type:     core.StateUpdateTypeState,
			State:    core.Follower,
			LeaderID: req.GetLeaderId(),
		})
	}

	response.Success = true
	return response, nil
}

func (rf *RaftState) requestVote(ctx context.Context, req *desc.RequestVoteRequest) (*desc.RequestVoteResponse, error) {
	var (
		term     = rf.state.GetTerm()
		response = &desc.RequestVoteResponse{
			Term:        term,
			VoteGranted: false,
		}

		reqTerm         = req.GetTerm()
		reqLastLogTerm  = req.GetLastLogTerm()
		reqLastLogIndex = req.GetLastLogIndex()
	)

	rf.resetElectionTimer()

	if reqTerm < term {
		return response, nil
	}

	if reqTerm > term {
		rf.sendStateUpdate(core.StateUpdate{
			Type: core.StateUpdateTypeTerm,
			Term: reqTerm,
		})
	}

	lastEntry, err := rf.getLastEntry(ctx)
	if err != nil {
		logger.ErrorKV(ctx, "request vote last", "error", err)
		return nil, err
	}

	if reqLastLogTerm > lastEntry.Term || (reqLastLogTerm == lastEntry.Term && reqLastLogIndex >= lastEntry.Index) {
		votedFor := rf.state.GetVotedFor()

		// Give vote only if the term is higher (i.e a new election) or we haven't voted yet
		if reqTerm > term || votedFor == 0 {
			rf.sendStateUpdate(core.StateUpdate{
				Type:     core.StateUpdateTypeState,
				State:    core.Follower,
				VotedFor: req.GetCandidateId(),
			})

			response.VoteGranted = true
		}
	}

	if response.VoteGranted {
		logger.WarnKV(ctx, "vote given",
			"candidate_id", req.GetCandidateId(),
			"term", term,
			"req_term", reqTerm,
			"req_last_entry_index", reqLastLogIndex,
			"req_last_entry_term", reqLastLogTerm,
			"last_entry_index", lastEntry.Index,
			"last_entry_term", lastEntry.Term,
		)
	} else {
		logger.WarnKV(ctx, "vote denied",
			"candidate_id", req.GetCandidateId(),
			"term", term,
			"req_term", reqTerm,
			"req_last_entry_index", reqLastLogIndex,
			"req_last_entry_term", reqLastLogTerm,
			"last_entry_index", lastEntry.Index,
			"last_entry_term", lastEntry.Term,
		)
	}

	return response, nil
}

func (rf *RaftState) processReplicate() {
	defer rf.wg.Done()

	for {
		select {
		case req := <-rf.replicateChan:
			err := rf.replicate(req.ctx, req.data)
			req.err <- err
		case <-rf.ctx.Done():
			return
		}
	}
}

func (rf *RaftState) replicate(ctx context.Context, data []string) error {
	term := rf.state.GetTerm()

	lastEntry, err := rf.getLastEntry(ctx)
	if err != nil {
		logger.ErrorKV(ctx, "replicate last entry", "error", err)
		return err
	}

	entry := &core.Entry{
		Term:  term,
		Index: lastEntry.Index + 1,
		Data:  data,
	}

	// Append the entry to the log
	err = rf.entryStore.AppendEntries(ctx, entry)
	if err != nil {
		return err
	}

	var (
		descEntry = &desc.Entry{
			Term:  entry.Term,
			Index: entry.Index,
			Data:  entry.Data,
		}

		errG         = errgroup.Group{}
		responseChan = make(chan *desc.AppendEntriesResponse, rf.quorum)

		// Close the done channel when the replication is done at least on
		// the majority of the servers, i.e. we can ignore the rest of the responses
		done = make(chan struct{})
	)

	// Replicate the entry to the followers
	for serverID, server := range rf.servers {
		errG.Go(func() error {
			response, err := server.AppendEntries(ctx, &desc.AppendEntriesRequest{
				Term:         term,
				LeaderId:     uint64(rf.raftID),
				PrevLogIndex: lastEntry.Index,
				PrevLogTerm:  lastEntry.Term,
				Entries:      []*desc.Entry{descEntry},
				LeaderCommit: rf.state.GetCommitIndex(),
			})
			if err != nil {
				logger.ErrorKV(ctx, "replicate entry", "error", err, "raft_id", serverID)
				return err
			}
			select {
			case responseChan <- response:
			case <-done: // ignore the rest of the responses
			}
			return nil
		})
	}

	go func() {
		_ = errG.Wait()
		close(responseChan)
	}()

	success := uint32(1) // count the leader as a vote
	for resp := range responseChan {
		if resp.Success {
			success++

			// TODO: on success request update nextIndex in leader for heartbeat
			// 	currently this leads to double replication of the entry

			if success == rf.quorum {
				rf.sendStateUpdate(core.StateUpdate{
					Type:        core.StateUpdateTypeCommitIndex,
					CommitIndex: entry.Index,
				})
				close(done)
				logger.WarnKV(ctx, "replication done", "entry_index", entry.Index)
				break
			}
		}

		// If the term is higher than the current term, update the term and continue collecting votes
		if resp.GetTerm() > term {
			rf.sendStateUpdate(core.StateUpdate{
				Type: core.StateUpdateTypeTerm,
				Term: resp.GetTerm(),
			})
		}
	}

	if success != rf.quorum {
		return core.ErrReplicateQuorumUnreachable
	}

	return nil
}

func (rf *RaftState) processCommit() {
	defer rf.wg.Done()

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		timer.Reset(rf.config.GetCommitPeriod())

		select {
		case <-timer.C:
			rf.commit(rf.ctx)
		case <-rf.ctx.Done():
			return
		}
	}
}

func (rf *RaftState) commit(ctx context.Context) {
	lastApplied := rf.state.GetLastApplied()
	commitIndex := rf.state.GetCommitIndex()

	if lastApplied == commitIndex {
		return
	}

	for lastApplied < commitIndex {
		entry, err := rf.entryStore.At(ctx, lastApplied+1)
		if err != nil {
			if !errors.Is(err, core.ErrNotFound) {
				logger.ErrorKV(ctx, "commit at", "error", err)
			}
			break
		}

		err = rf.applyEntry(ctx, entry)
		if err != nil {
			logger.ErrorKV(ctx, "commit apply entry", "error", err)
			break
		}

		rf.sendStateUpdate(core.StateUpdate{
			Type:        core.StateUpdateTypeLastApplied,
			LastApplied: entry.Index,
		})

		lastApplied++
	}
}

func (rf *RaftState) applyEntry(ctx context.Context, entry *core.Entry) error {
	var err error

	entryType := entry.Data[0]
	switch entryType {
	case core.Set.String():
		err = rf.kvStore.Set(ctx, entry.Data[1], entry.Data[2])
	case core.Del.String():
		err = rf.kvStore.Del(ctx, entry.Data[1])
	default:
		err = core.ErrUnknownEntryType
	}

	if err != nil {
		return err
	}

	return nil
}

func (rf *RaftState) processElection() {
	defer rf.wg.Done()

	timer := time.NewTimer(rf.config.GetElectionDelayPeriod())
	defer timer.Stop()

	<-timer.C // wait for the delay before starting the election

	for {
		timer.Reset(rf.config.GetElectionTimeoutPeriod())

		select {
		case <-timer.C:
		case <-rf.heartbeat:
			// If we receive a heartbeat, we should reset the timer
			continue
		case <-rf.ctx.Done():
			// If the context is done then app is shutting down
			// so we should stop the loop
			return
		}

		if !rf.state.IsLeader() {
			logger.WarnKV(rf.ctx, "starting election")
			rf.startElection(rf.ctx)
		}
	}
}

func (rf *RaftState) startElection(ctx context.Context) {
	rf.sendStateUpdate(core.StateUpdate{
		Type:  core.StateUpdateTypeState,
		State: core.Candidate,
	})

	lastEntry, err := rf.getLastEntry(ctx)
	if err != nil {
		logger.ErrorKV(ctx, "election last entry", "error", err)
		return
	}

	var (
		term = rf.state.GetTerm()

		errG         = errgroup.Group{}
		responseChan = make(chan struct{})

		// Close the done channel when the vote is collected at least from
		// the majority of the servers, i.e. we can ignore the rest of the responses
		done = make(chan struct{})
	)

	logger.WarnKV(ctx, "election info",
		"term", term,
		"last_log_index", lastEntry.Index,
		"last_log_term", lastEntry.Term,
	)

	for raftID, server := range rf.servers {
		errG.Go(func() error {
			sendCtx, cancel := context.WithTimeout(ctx, 1*time.Second) // TODO: make this configurable
			defer cancel()

			res, err := server.RequestVote(sendCtx, &desc.RequestVoteRequest{
				Term:         term,
				CandidateId:  uint64(rf.raftID),
				LastLogIndex: lastEntry.Index,
				LastLogTerm:  lastEntry.Term,
			})

			if err != nil {
				logger.ErrorKV(ctx, "election request vote", "error", err, "voter", raftID)
				return nil
			}

			if res.GetTerm() > term {
				rf.sendStateUpdate(core.StateUpdate{
					Type: core.StateUpdateTypeTerm,
					Term: res.GetTerm(),
				})
			}

			// log request result
			defer func() {
				result := "vote not granted"
				if res.VoteGranted {
					result = "vote granted"
				}

				logger.WarnKV(ctx, result,
					"voter", raftID,
					"term", term,
					"res_term", res.GetTerm(),
					"last_entry_index", lastEntry.Index,
					"last_entry_term", lastEntry.Term,
				)
			}()

			if res.VoteGranted {
				select {
				case responseChan <- struct{}{}:
				case <-done: // ignore the rest of the responses
				}
			}

			return nil
		})
	}

	go func() {
		_ = errG.Wait()
		close(responseChan)
	}()

	votes := uint32(1)
	for range responseChan {
		votes++

		// If candidate state has changed, this means a new leader has sent an AppendEntries or
		// another candidate has as collected our vote
		if rf.state.GetState() != core.Candidate {
			logger.WarnKV(ctx, "stop election => candidate state has changed since start")
			close(done)
			break
		}

		if votes == rf.quorum {
			rf.sendStateUpdate(core.StateUpdate{
				Type:  core.StateUpdateTypeState,
				State: core.Leader,
			})

			logger.WarnKV(ctx, "election won")

			close(done)
			break
		}
	}
}

func (rf *RaftState) getLastEntry(ctx context.Context) (*core.Entry, error) {
	lastEntry, err := rf.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			return nil, err
		}
		lastEntry = &core.Entry{} // no entries yet (i.e. initial state)
	}
	return lastEntry, nil
}
