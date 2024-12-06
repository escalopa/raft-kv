package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/escalopa/raft-kv/internal/core"
	"github.com/pkg/errors"
)

func parseClusterConfig(_ context.Context, raftID uint64, cluster []string) ([]core.Node, error) {
	var (
		nodes     = make([]core.Node, 0, len(cluster))
		uniqueIDs = make(map[uint64]struct{})

		id      uint64
		address string

		err error
	)

	for i, node := range cluster {
		if node == "" {
			return nil, errors.Errorf("empty node at RAFT_CLUSTER[%d]", i)
		}

		// Parse node ServerID and address
		_, err = fmt.Fscanf(strings.NewReader(node), "%d@%s", &id, &address)
		if err != nil {
			return nil, errors.Errorf("parse node address at RAFT_CLUSTER[%d](%s): %v", i, node, err)
		}

		// Validate ServerID
		if id == raftID {
			continue // skip self node
		}

		if id == 0 {
			return nil, errors.Errorf("node ServerID must be greater than 0 at RAFT_CLUSTER[%d](%s)", i, node)
		}
		// Check for duplicate node ServerID
		if _, ok := uniqueIDs[id]; ok {
			return nil, errors.Errorf("duplicate node ServerID at RAFT_CLUSTER[%d](%s)", i, node)
		}
		uniqueIDs[id] = struct{}{}

		// Validate address
		if address == "" {
			return nil, errors.Errorf("empty node address at RAFT_CLUSTER[%d](%s)", i, node)
		}

		// TODO: validate address format (e.g. IP:PORT)

		// Append node
		nodes = append(nodes, core.Node{
			ID:      core.ServerID(id),
			Address: address,
		})
	}

	return nodes, nil
}
