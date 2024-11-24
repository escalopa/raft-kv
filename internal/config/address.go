package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/catalystgo/logger/logger"

	"github.com/pkg/errors"
)

func parseClusterConfig(ctx context.Context, raftID uint64, cluster []string) ([]Node, error) {
	var (
		nodes     = make([]Node, 0, len(cluster))
		uniqueIDs = make(map[uint64]struct{})

		id      uint64
		address string

		err error
	)

	for i, node := range cluster {
		if node == "" {
			return nil, errors.Errorf("empty node at RAFT_CLUSTER[%d]", i)
		}

		// Parse node ID and address
		_, err = fmt.Fscanf(strings.NewReader(node), "%d@%s", &id, &address)
		if err != nil {
			return nil, errors.Errorf("parse node address at RAFT_CLUSTER[%d](%s): %v", i, node, err)
		}

		// Validate ID
		if id == raftID {
			logger.Warnf(ctx, "skip self node at RAFT_CLUSTER[%d](%s)", i, node)
			continue
		}
		if id == 0 {
			return nil, errors.Errorf("node ID must be greater than 0 at RAFT_CLUSTER[%d](%s)", i, node)
		}
		// Check for duplicate node ID
		if _, ok := uniqueIDs[id]; ok {
			return nil, errors.Errorf("duplicate node ID at RAFT_CLUSTER[%d](%s)", i, node)
		}
		uniqueIDs[id] = struct{}{}

		// Validate address
		if address == "" {
			return nil, errors.Errorf("empty node address at RAFT_CLUSTER[%d](%s)", i, node)
		}

		// Append node
		nodes[i] = Node{
			ID:      id,
			Address: address,
		}
	}

	return nodes, nil
}
