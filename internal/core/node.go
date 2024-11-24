package core

type (
	ServerID uint64

	Node struct {
		ID      ServerID
		Address string
	}
)
