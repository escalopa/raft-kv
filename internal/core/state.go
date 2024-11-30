package core

type State string

func (s State) String() string {
	return string(s)
}

const (
	Follower  State = "follower"
	Candidate State = "candidate"
	Leader    State = "leader"
)

type StateUpdateType string

const (
	// StateUpdateTypeTerm is used to update the term of the cluster
	StateUpdateTypeTerm StateUpdateType = "Term"

	// StateUpdateTypeVotedFor is used to update the voted for on election process
	StateUpdateTypeVotedFor StateUpdateType = "VotedFor"

	// StateUpdateTypeCommitIndex is used to update the commit index on the server storage
	StateUpdateTypeCommitIndex StateUpdateType = "CommitIndex"

	// StateUpdateTypeLastApplied is used to update the last applied index on the state machine
	StateUpdateTypeLastApplied StateUpdateType = "LastApplied"

	// StateUpdateTypeState is used to update the state of the server (Follower, Candidate, Leader)
	StateUpdateTypeState StateUpdateType = "State"

	// StateUpdateTypeLeaderID is used to update the leader ID
	StateUpdateTypeLeaderID StateUpdateType = "LeaderID"
)

type StateUpdate struct {
	Type        StateUpdateType
	Term        uint64
	VotedFor    uint64
	CommitIndex uint64
	LastApplied uint64
	LeaderID    uint64
	State       State

	// Done is used to signal that the state update has been processed.
	Done chan struct{}
}
