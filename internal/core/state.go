package core

type State string

const (
	Follower  State = "Follower"
	Candidate State = "Candidate"
	Leader    State = "Leader"
)

type StateUpdateType string

const (
	StateUpdateTypeTerm        StateUpdateType = "Term"
	StateUpdateTypeVotedFor    StateUpdateType = "VotedFor"
	StateUpdateTypeCommitIndex StateUpdateType = "CommitIndex"
	StateUpdateTypeLastApplied StateUpdateType = "LastApplied"
	StateUpdateTypeState       StateUpdateType = "State"
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
