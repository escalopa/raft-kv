package service

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/catalystgo/logger/logger"
	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
	"github.com/pkg/errors"
)

func (rf *RaftState) processAppendEntries() {
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
	)

	// Reply false if term < currentTerm
	if req.Term < term {
		return response, nil
	}

	// TODO: Check if the term is higher, the request might be from a new leader

	entry, err := rf.entryStore.At(ctx, req.PrevLogIndex)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(rf.ctx, "append entries at", "error", err)
			return nil, err
		}
		// Reply false if log doesn’t contain an entry at prevLogIndex
		return response, nil
	}

	// Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
	if entry.Term != req.PrevLogTerm {
		return response, nil
	}

	entries := make([]*core.Entry, len(req.Entries))
	for i, e := range req.Entries {
		entries[i] = &core.Entry{
			Term:  e.Term,
			Index: e.Index,
			Data:  e.Data,
		}
	}

	err = rf.entryStore.AppendEntries(ctx, entries...)
	if err != nil {
		logger.ErrorKV(rf.ctx, "append entries", "error", err)
		return nil, err
	}

	if req.LeaderCommit > rf.state.GetCommitIndex() {
		rf.updateState(core.StateUpdate{
			Type:        core.StateUpdateTypeCommitIndex,
			CommitIndex: min(req.LeaderCommit, entries[len(entries)-1].Index),
		})
	}

	// TODO:
	//  - reset election timer
	//  - convert to follower if necessary

	response.Success = true
	return response, nil
}

func (rf *RaftState) processRequestVote() {
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
	)

	if req.Term < term {
		return response, nil
	}

	if req.Term > term {
		rf.updateState(core.StateUpdate{
			Type: core.StateUpdateTypeTerm,
			Term: req.Term,
		})
	}

	if rf.state.GetVotedFor() == req.CandidateId { // already voted for this candidate
		response.VoteGranted = true
		return response, nil
	}

	lastEntry, err := rf.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(rf.ctx, "request vote last", "error", err)
			return nil, err
		}
		lastEntry = &core.Entry{}
	}

	if req.LastLogTerm > lastEntry.Term || (req.LastLogTerm == lastEntry.Term && req.LastLogIndex >= lastEntry.Index) {
		rf.updateState(core.StateUpdate{
			Type:     core.StateUpdateTypeVotedFor,
			VotedFor: req.CandidateId,
		})

		response.VoteGranted = true
		return response, nil
	}

	return response, nil
}

func (rf *RaftState) processReplicate() {
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
	lastEntry, err := rf.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(ctx, "replicate last entry", "error", err)
			return err
		}
		lastEntry = &core.Entry{}
	}

	entry := &core.Entry{
		Term:  rf.state.GetTerm(),
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

		responseChan = make(chan *desc.AppendEntriesResponse, rf.quorum)
		errG         = errgroup.Group{}
	)

	// Replicate the entry to the followers
	for serverID, server := range rf.servers {
		errG.Go(func() error {
			response, err := server.AppendEntries(ctx, &desc.AppendEntriesRequest{
				Term:         rf.state.GetTerm(),
				LeaderId:     uint64(rf.raftID),
				PrevLogIndex: lastEntry.Index,
				PrevLogTerm:  lastEntry.Term,
				Entries:      []*desc.Entry{descEntry},
				LeaderCommit: rf.state.GetCommitIndex(),
			})
			if err != nil {
				logger.ErrorKV(rf.ctx, "replicate entry", "error", err, "server_id", serverID)
				return err
			}
			responseChan <- response
			return nil
		})
	}

	go func() {
		_ = errG.Wait()
		close(responseChan)
	}()

	var success uint32
	for resp := range responseChan {
		if resp.Success {
			success++
		}
		// TODO: if the term is higher than the current term then become a follower
		// TODO: check all response values
		if success >= rf.quorum {
			rf.updateState(core.StateUpdate{
				Type:        core.StateUpdateTypeCommitIndex,
				CommitIndex: entry.Index,
			})
			break
		}
	}

	return nil
}

func (rf *RaftState) processCommit() {

}
