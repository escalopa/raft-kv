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

func (rf *RaftState) processAppendEntries() {
	defer rf.wg.Done()

	for {
		select {
		case req := <-rf.appendEntriesChan:
			res, err := rf.appendEntries(req.ctx, req.req)
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

	defer rf.resetElectionTimer()

	lastEntry, err := rf.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(ctx, "append entries last", "error", err)
			return nil, err
		}
		// no entry yet in log
		lastEntry = &core.Entry{}
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

func (rf *RaftState) processRequestVote() {
	defer rf.wg.Done()

	for {
		select {
		case req := <-rf.requestVoteChan:
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

	defer rf.resetElectionTimer()

	if reqTerm < term {
		return response, nil
	}

	if reqTerm > term {
		rf.sendStateUpdate(core.StateUpdate{
			Type: core.StateUpdateTypeTerm,
			Term: reqTerm,
		})
	}

	lastEntry, err := rf.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(ctx, "request vote last", "error", err)
			return nil, err
		}
		lastEntry = &core.Entry{}
	}

	if reqLastLogTerm > lastEntry.Term || (reqLastLogTerm == lastEntry.Term && reqLastLogIndex >= lastEntry.Index) {
		// If we have already voted for a node, give a vote only if the term is higher

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
		logger.WarnKV(ctx, "vote given", "candidate_id", req.GetCandidateId(), "term", term, "req_term", reqTerm)
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

	lastEntry, err := rf.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(ctx, "replicate last entry", "error", err)
			return err
		}
		lastEntry = &core.Entry{}
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
		}

		// If the term is higher than the current term, update the term and continue collecting votes
		if resp.Term > term {
			rf.sendStateUpdate(core.StateUpdate{
				Type: core.StateUpdateTypeTerm,
				Term: resp.Term,
			})
			continue // skip
		}

		// TODO: on success request update nextIndex in leader for heartbeat

		if success == rf.quorum {
			rf.sendStateUpdate(core.StateUpdate{
				Type:        core.StateUpdateTypeCommitIndex,
				CommitIndex: entry.Index,
			})
			close(done)
			break
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

	timer := time.NewTimer(rf.config.GetElectionDelay())
	defer timer.Stop()

	<-timer.C // wait for the delay before starting the election

	for {
		timer.Reset(rf.config.GetElectionTimeout())

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

	entry, err := rf.entryStore.Last(ctx)
	if err != nil {
		// If the error is not found, then we can ignore it
		// because it means that there are no entries in the log yet (i.e. first run)
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(ctx, "election last entry", "error", err)
			return
		}
		entry = &core.Entry{}
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
		"last_log_index", entry.Index,
		"last_log_term", entry.Term,
	)

	for raftID, server := range rf.servers {
		errG.Go(func() error {
			res, err := server.RequestVote(ctx, &desc.RequestVoteRequest{
				Term:         term,
				CandidateId:  uint64(rf.raftID),
				LastLogIndex: entry.Index,
				LastLogTerm:  entry.Term,
			})

			if err != nil {
				logger.ErrorKV(ctx, "election request vote", "error", err, "voter", raftID)
				return nil
			}

			if res.Term > term {
				rf.sendStateUpdate(core.StateUpdate{
					Type: core.StateUpdateTypeTerm,
					Term: res.Term,
				})
				return nil
			}

			if res.VoteGranted {
				select {
				case responseChan <- struct{}{}:
					logger.WarnKV(ctx, "vote granted", "voter", raftID, "term", term)
				case <-done: // ignore the rest of the responses
				}
				return nil
			}

			logger.WarnKV(ctx, "vote not granted", "voter", raftID, "term", term, "res_term", res.GetTerm())

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

		if votes == rf.quorum {
			logger.WarnKV(ctx, "election won")

			rf.sendStateUpdate(core.StateUpdate{
				Type:  core.StateUpdateTypeState,
				State: core.Leader,
			})

			close(done)
			break
		}
	}
}
