package core

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrNotFound                   = fmt.Errorf("not found")
	ErrNotLeader                  = fmt.Errorf("not leader")
	ErrUnknownEntryType           = fmt.Errorf("unknown entry type")
	ErrReplicateQuorumUnreachable = fmt.Errorf("replicate quorum unreachable")
)

func ToGrpcError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case errors.Is(err, ErrNotLeader):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, ErrReplicateQuorumUnreachable):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
