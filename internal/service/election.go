package service

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/catalystgo/logger/logger"
	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func (rf *RaftState) electionTimeout() {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		timeout := time.Duration(150+rand.IntN(150)) * time.Millisecond
		timer.Reset(timeout)

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
			rf.startElection(rf.ctx)
		}
	}
}

func (rf *RaftState) startElection(ctx context.Context) {
	rf.updateState(core.StateUpdate{
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
		votesChan = make(chan struct{})
		errG      = errgroup.Group{}
	)

	for serverID, server := range rf.servers {
		errG.Go(func() error {
			res, err := server.RequestVote(ctx, &desc.RequestVoteRequest{
				Term:         rf.state.GetTerm(),
				CandidateId:  uint64(rf.raftID),
				LastLogIndex: entry.Index,
				LastLogTerm:  entry.Term,
			})

			if err != nil {
				logger.ErrorKV(ctx, "election request vote", "error", err, "server_id", serverID)
				return nil
			}

			if res.Term > rf.state.GetTerm() {
				rf.updateState(core.StateUpdate{
					Type: core.StateUpdateTypeTerm,
					Term: res.Term,
				})
				return nil
			}

			if res.VoteGranted {
				votesChan <- struct{}{}
			}

			return nil
		})
	}

	go func() {
		_ = errG.Wait()
		close(votesChan)
	}()

	votes := uint32(1)
	for range votesChan {
		votes++
		if votes >= rf.quorum {
			rf.updateState(core.StateUpdate{
				Type:  core.StateUpdateTypeState,
				State: core.Leader,
			})
			return
		}
	}
}
