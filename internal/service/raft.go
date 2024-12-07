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

func (r *Raft) processRPC(req interface{}) {
	switch req := req.(type) {
	case *appendEntriesRequest:
		res, err := r.appendEntries(req.ctx, req.req)
		req.reply(res, err)
	case *requestVoteRequest:
		res, err := r.requestVote(req.ctx, req.req)
		req.reply(res, err)
	case *replicateRequest:
		err := r.replicate(req.ctx, req.data)
		req.reply(err)
	default:
		logger.ErrorKV(r.ctx, "unknown request type", "req", req)
	}
}

func (r *Raft) appendEntries(ctx context.Context, req *desc.AppendEntriesRequest) (*desc.AppendEntriesResponse, error) {
	var (
		term     = r.state.GetTerm()
		response = &desc.AppendEntriesResponse{
			Term:    term,
			Success: false,
		}

		reqTerm         = req.GetTerm()
		reqPrevLogTerm  = req.GetPrevLogTerm()
		reqPrevLogIndex = req.GetPrevLogIndex()
	)

	defer r.state.UpdateLastContact()

	lastLogIndex, _ := r.state.GetLastLog()
	response.LastLogIndex = lastLogIndex

	// Reply false if term < currentTerm
	if reqTerm < term {
		logger.InfoKV(ctx, "append entries term < current term", "term", term, "req_term", reqTerm, "raft_id", req.GetLeaderId())
		return response, nil
	}

	if reqTerm > term {
		r.state.SetTerm(reqTerm)
		r.state.SetLeaderID(req.GetLeaderId())
		r.state.SetState(FollowerState)
	}

	entry, err := r.entryStore.At(ctx, reqPrevLogIndex)
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
		logger.InfoKV(ctx, "append entries entry not found", "index", reqPrevLogIndex, "raft_id", req.GetLeaderId())
		return response, nil
	}

	// Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
	if entry.Term != reqPrevLogTerm {
		logger.InfoKV(ctx, "append entries term mismatch",
			"index", reqPrevLogIndex,
			"raft_id", req.GetLeaderId(),
			"entry_term", entry.Term,
			"req_term", reqPrevLogTerm,
		)
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

	err = r.entryStore.AppendEntries(ctx, entries...)
	if err != nil {
		logger.ErrorKV(ctx, "append entries", "error", err)
		return nil, err
	}

	logChanged := len(entries) > 0

	if logChanged {
		lastEntry := entries[len(entries)-1]
		r.state.SetLastLog(lastEntry.Index, lastEntry.Term) // update last log
	}

	if req.GetLeaderCommit() > r.state.GetCommitIndex() {
		commitIndex := req.GetLeaderCommit()

		if logChanged {
			commitIndex = min(commitIndex, entries[len(entries)-1].Index)
		}

		r.state.SetCommitIndex(commitIndex)
	}

	state := r.state.GetState()
	if state == LeaderState || state == CandidateState {
		logger.WarnKV(ctx, "append entries resign to follower", "raft_id", req.GetLeaderId(), "state", state)
		r.state.SetState(FollowerState)
	}

	response.Success = true
	return response, nil
}

func (r *Raft) requestVote(ctx context.Context, req *desc.RequestVoteRequest) (*desc.RequestVoteResponse, error) {
	var (
		term = r.state.GetTerm()

		response = &desc.RequestVoteResponse{
			Term:        term,
			VoteGranted: false,
		}

		reqTerm         = req.GetTerm()
		reqLastLogTerm  = req.GetLastLogTerm()
		reqLastLogIndex = req.GetLastLogIndex()
	)

	defer r.state.UpdateLastContact()

	if reqTerm < term {
		logger.InfoKV(ctx, "request vote term < current term",
			"term", term,
			"req_term", reqTerm,
			"raft_id", req.GetCandidateId(),
		)

		return response, nil
	}

	if reqTerm > term {
		r.state.SetTerm(reqTerm)
	}

	lastLogIndex, lastLogTerm := r.state.GetLastLog()
	if reqLastLogTerm > lastLogTerm || (reqLastLogTerm == lastLogTerm && reqLastLogIndex >= lastLogIndex) {
		votedFor := r.state.GetVotedFor()

		// Give vote only if the term is higher (i.e a new election) or we haven't voted yet
		if reqTerm > term || votedFor == 0 {
			r.state.SetState(FollowerState)
			r.state.SetVotedFor(req.GetCandidateId())
			r.state.SetLeaderID(req.GetCandidateId())
			response.VoteGranted = true
		}
	}

	if response.VoteGranted {
		logger.WarnKV(ctx, "vote given",
			"raft_id", req.GetCandidateId(),
			"term", term,
			"req_term", reqTerm,
			"req_last_entry_index", reqLastLogIndex,
			"req_last_entry_term", reqLastLogTerm,
			"last_entry_index", lastLogIndex,
			"last_entry_term", lastLogTerm,
		)
	} else {
		logger.WarnKV(ctx, "vote not given",
			"raft_id", req.GetCandidateId(),
			"term", term,
			"req_term", reqTerm,
			"req_last_entry_index", reqLastLogIndex,
			"req_last_entry_term", reqLastLogTerm,
			"last_entry_index", lastLogIndex,
			"last_entry_term", lastLogTerm,
		)
	}

	return response, nil
}

func (r *Raft) replicate(ctx context.Context, data []string) error {
	var (
		term        = r.state.GetTerm()
		commitIndex = r.state.GetCommitIndex()
	)

	lastLogIndex, lastLogTerm := r.state.GetLastLog()
	entry := &core.Entry{
		Term:  term,
		Index: lastLogIndex + 1,
		Data:  data,
	}

	// Append the entry to the log
	err := r.entryStore.AppendEntries(ctx, entry)
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
		responseChan = make(chan *replicateResponse, len(r.servers))
	)

	// Replicate the entry to the followers
	for raftID, server := range r.servers {
		errG.Go(func() error {
			sendCtx, cancel := context.WithTimeout(ctx, r.config.GetAppendEntriesTimeout())
			defer cancel()

			response, err := server.AppendEntries(sendCtx, &desc.AppendEntriesRequest{
				Term:         term,
				LeaderId:     uint64(r.raftID),
				PrevLogIndex: lastLogIndex,
				PrevLogTerm:  lastLogTerm,
				Entries:      []*desc.Entry{descEntry},
				LeaderCommit: commitIndex,
			})
			if err != nil {
				logger.ErrorKV(ctx, "replicate entry", "error", err, "raft_id", raftID)
				return err
			}

			responseChan <- &replicateResponse{
				raftID: raftID,
				res:    response,
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
		if resp.res.Success {
			success++

			// update the nextIndex and matchIndex for the follower (prevent re-sending the same entry)
			r.leader.nextIndex.Store(resp.raftID, entry.Index+1)
			r.leader.matchIndex.Store(resp.raftID, entry.Index)

			if success == r.quorum {
				r.state.SetLastLog(entry.Index, entry.Term)
				r.state.SetCommitIndex(entry.Index)
				logger.WarnKV(ctx, "replication done", "entry_index", entry.Index)
				break
			}
		} else {
			resTerm := resp.res.GetTerm()
			if resTerm > term {
				r.state.SetTerm(resTerm)
			}
		}
	}

	if success != r.quorum {
		return core.ErrReplicateQuorumUnreachable
	}

	return nil
}

func (r *Raft) commit() {
	defer r.wg.Done()

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		timer.Reset(r.config.GetCommitPeriod())

		select {
		case <-timer.C:
			r.tryCommit()
		case <-r.shutdownChan:
			logger.WarnKV(r.ctx, "commit shutdown")
			return
		}
	}
}

func (r *Raft) tryCommit() {
	lastApplied := r.state.GetLastApplied()
	commitIndex := r.state.GetCommitIndex()

	if lastApplied == commitIndex {
		return // nothing to commit
	}

	for lastApplied < commitIndex {
		index := lastApplied + 1

		entry, err := r.entryStore.At(r.ctx, index)
		if err != nil {
			if !errors.Is(err, core.ErrNotFound) {
				logger.ErrorKV(r.ctx, "commit at", "error", err, "index", index)
			}
			break
		}

		err = r.applyEntry(entry)
		if err != nil {
			logger.ErrorKV(r.ctx, "commit apply entry", "error", err, "index", index)
			break
		}

		r.state.SetLastApplied(index)
		lastApplied++
	}
}

func (r *Raft) applyEntry(entry *core.Entry) error {
	entryType := entry.Data[0]
	switch entryType {
	case core.Set.String():
		return r.kvStore.Set(r.ctx, entry.Data[1], entry.Data[2])
	case core.Del.String():
		return r.kvStore.Del(r.ctx, entry.Data[1])
	default:
		return core.ErrUnknownEntryType
	}
}
