package service

import (
	"context"
	"sync"
	"time"

	"github.com/catalystgo/logger/logger"
	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func (r *Raft) runFollower() {
	electionTimeout := r.config.GetElectionTimeout()

	heartbeatTimer := time.NewTimer(electionTimeout)
	defer heartbeatTimer.Stop()

	for r.state.GetState() == FollowerState {
		select {
		case req := <-r.requestChan:
			r.processRPC(req)
		case <-heartbeatTimer.C:
			if time.Since(r.state.GetLastContact()) >= electionTimeout {
				logger.WarnKV(r.ctx, "heartbeat timeout")
				r.state.SetState(CandidateState)
				return
			}
			// reset the heartbeat timer
			electionTimeout = r.config.GetElectionTimeout()
			heartbeatTimer.Reset(electionTimeout)
		case <-r.shutdownChan:
			logger.WarnKV(r.ctx, "follower shutdown signal received")
			return
		}
	}
}

func (r *Raft) runCandidate() {
	var (
		responseChan    = r.startElection()
		electionTimeout = r.config.GetElectionTimeout()
	)

	heartbeatTimer := time.NewTimer(electionTimeout)
	defer heartbeatTimer.Stop()

	votes := uint32(1) // vote for self

	for r.state.GetState() == CandidateState {
		select {
		case req := <-r.requestChan:
			r.processRPC(req)
		case res := <-responseChan:
			if res.GetVoteGranted() {
				votes++
				if votes == r.quorum {
					logger.WarnKV(r.ctx, "election won")
					r.state.SetState(LeaderState)
					return
				}
			}
		case <-heartbeatTimer.C:
			logger.WarnKV(r.ctx, "restart election due to timeout")
			return
		case <-r.shutdownChan:
			logger.WarnKV(r.ctx, "candidate shutdown signal received")
			return
		}
	}
}

func (r *Raft) startElection() <-chan *desc.RequestVoteResponse {
	r.state.IncrementTerm()
	r.state.SetVotedFor(uint64(r.raftID))

	lastLogIndex, lastLogTerm := r.state.GetLastLog()

	var (
		term = r.state.GetTerm()

		errG         = errgroup.Group{}
		responseChan = make(chan *desc.RequestVoteResponse, len(r.servers))
	)

	logger.WarnKV(r.ctx, "election start info",
		"term", term,
		"last_log_index", lastLogIndex,
		"last_log_term", lastLogTerm,
	)

	for raftID, server := range r.servers {
		errG.Go(func() error {
			sendCtx, cancel := context.WithTimeout(r.ctx, r.config.GetRequestVoteTimeout())
			defer cancel()

			res, err := server.RequestVote(sendCtx, &desc.RequestVoteRequest{
				Term:         term,
				CandidateId:  uint64(r.raftID),
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			})

			if err != nil {
				logger.ErrorKV(r.ctx, "election request vote", "error", err, "voter", raftID)
				return nil
			}

			if res.GetTerm() > term {
				r.state.SetTerm(res.GetTerm())
			}

			// log request result
			{
				result := "vote not granted"
				if res.GetVoteGranted() {
					result = "vote granted"
				}

				logger.WarnKV(r.ctx, result,
					"voter", raftID,
					"term", term,
					"res_term", res.GetTerm(),
					"last_entry_index", lastLogIndex,
					"last_entry_term", lastLogTerm,
				)
			}

			responseChan <- res

			return nil
		})
	}

	go func() {
		_ = errG.Wait()
		close(responseChan)
	}()

	return responseChan
}

func (r *Raft) runLeader() {
	var (
		stopChan   = make(chan struct{}) // channel to stop the leader's goroutines
		resignChan = make(chan struct{}) // channel to signal leader to resign
	)

	r.leader = newLeaderState()

	lastLogIndex, _ := r.state.GetLastLog()
	for raftID := range r.servers {
		r.leader.nextIndex.Store(raftID, lastLogIndex+1)
		r.leader.matchIndex.Store(raftID, 0)
		r.leader.lastHeartbeat.Store(raftID, time.Now())
	}

	wg := &sync.WaitGroup{}
	for raftID := range r.servers {
		r.goLoop(wg, func() { r.sendHeartbeat(raftID) }, r.config.GetLeaderHeartbeatPeriod, stopChan)
	}
	r.goLoop(wg, func() { r.checkResignState(resignChan) }, r.config.GetLeaderStalePeriod, stopChan)

	defer func() {
		tryClose(stopChan)
		tryClose(resignChan)

		wg.Wait()

		r.leader = nil // cleanup leader state

		logger.WarnKV(r.ctx, "leader stop")
	}()

	logger.WarnKV(r.ctx, "leader start")

	for r.state.GetState() == LeaderState {
		select {
		case req := <-r.requestChan:
			r.processRPC(req)
		case <-resignChan:
			logger.WarnKV(r.ctx, "leader resign")
			r.state.SetState(FollowerState)
			return
		case <-r.shutdownChan:
			logger.WarnKV(r.ctx, "leader shutdown signal received")
			return
		}
	}
}

func (r *Raft) sendHeartbeat(raftID core.ServerID) {
	var (
		term            = r.state.GetTerm()
		lastLogIndex, _ = r.state.GetLastLog()
	)

	var (
		lowerBound, _ = r.leader.nextIndex.Load(raftID)
		upperBound    = min(lastLogIndex, lowerBound+r.config.GetLeaderHeartbeatBatchSize())
	)

	entries, err := r.entryStore.Range(r.ctx, lowerBound, upperBound)
	if err != nil {
		logger.ErrorKV(r.ctx, "heartbeat range", "error", err, "raft_id", raftID)
		return
	}

	entriesDesc := make([]*desc.Entry, len(entries))
	for i, e := range entries {
		entriesDesc[i] = &desc.Entry{
			Term:  e.Term,
			Index: e.Index,
			Data:  e.Data,
		}
	}

	// Get the previous entry to send the correct prevLogIndex and prevLogTerm
	prevEntry, err := r.entryStore.At(r.ctx, lowerBound-1)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(r.ctx, "heartbeat at", "error", err, "raft_id", raftID)
			return
		}

		// no previous entry (first entry)
		if lowerBound == 1 {
			prevEntry = &core.Entry{}
		}
	}

	if prevEntry == nil {
		logger.ErrorKV(r.ctx, "load prev log entry on heartbeat send",
			"raft_id", raftID,
			"lower_bound", lowerBound,
		)
		return
	}

	server := r.servers[raftID]

	sendCtx, cancel := context.WithTimeout(r.ctx, r.config.GetAppendEntriesTimeout())
	defer cancel()

	res, err := server.AppendEntries(sendCtx, &desc.AppendEntriesRequest{
		Term:         term,
		LeaderId:     uint64(r.raftID),
		PrevLogIndex: prevEntry.Index,
		PrevLogTerm:  prevEntry.Term,
		LeaderCommit: r.state.GetCommitIndex(),
		Entries:      entriesDesc,
	})
	if err != nil {
		// TODO: add backoff and retry
		logger.ErrorKV(r.ctx, "heartbeat append entries", "error", err, "raft_id", raftID)
		return
	}

	// If we got a response from the follower, we can update the last heartbeat
	// even if the request was not successful (i.e. the follower is not up to date)
	defer r.leader.lastHeartbeat.Store(raftID, time.Now())

	if res.Term > term {
		logger.InfoKV(r.ctx, "leader update term", "old_term", term, "res_term", res.Term, "raft_id", raftID)
		r.state.SetTerm(res.Term)
	}

	// If the follower is up to date, then we can update the nextIndex and matchIndex
	if res.Success {
		r.leader.nextIndex.Store(raftID, upperBound+1)
		r.leader.matchIndex.Store(raftID, upperBound)
		return
	}

	// On the next heartbeat send from the latest log + 1
	nextIndex := res.LastLogIndex + 1

	// If prevLogIndex is less than the latest log on the follower, that means the follower has some
	// inconsistent data that need to be cleaned therefore we decrement the log by -1
	if res.LastLogIndex >= prevEntry.Index {
		nextIndex = prevEntry.Index - 1
	}

	r.leader.nextIndex.Store(raftID, nextIndex)
}

// checkResignState checks if the leader can contact the majority of the servers in the cluster
// if not it will signal the leader to resign
func (r *Raft) checkResignState(resignChan chan struct{}) {
	contacted := 1 // leader is counted as contacted
	for raftID := range r.servers {
		lastHeartbeat, ok := r.leader.lastHeartbeat.Load(raftID)
		if !ok {
			continue
		}

		if time.Since(lastHeartbeat) < r.config.GetLeaderStalePeriod() {
			contacted++
		}
	}

	canContactMajority := contacted > (len(r.servers)+1)/2
	if !canContactMajority {
		logger.WarnKV(r.ctx, "leader cannot contact majority of the nodes in cluster", "contacted", contacted)
		tryClose(resignChan)
	}
}

// repeat runs the given function f every freq until done is closed
func (r *Raft) goLoop(wg *sync.WaitGroup, f func(), freq func() time.Duration, stopChan <-chan struct{}) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		timer := time.NewTimer(0)
		defer timer.Stop()

		for {
			timer.Reset(freq())

			select {
			case <-stopChan:
				return
			case <-timer.C:
				f()
			}
		}
	}()
}
