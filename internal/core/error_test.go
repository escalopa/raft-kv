package core

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToGrpcError(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
		wantCode codes.Code
	}{
		{"nil error", nil, codes.OK},
		{"context canceled", context.Canceled, codes.Canceled},
		{"context deadline exceeded", context.DeadlineExceeded, codes.DeadlineExceeded},
		{"empty key error", ErrEmptyKey, codes.InvalidArgument},
		{"not leader error", ErrNotLeader, codes.FailedPrecondition},
		{"not found error", ErrNotFound, codes.NotFound},
		{"replicate quorum unreachable", ErrReplicateQuorumUnreachable, codes.Unavailable},
		{"unknown error", errors.New("generic error"), codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := ToGrpcError(tt.inputErr)

			if tt.inputErr == nil {
				require.Nil(t, gotErr, "expected nil error but got %v", gotErr)
				return
			}

			gotStatus, _ := status.FromError(gotErr)
			require.Equal(t, tt.wantCode, gotStatus.Code(), "unexpected gRPC code")
		})
	}
}

func FuzzToGrpcError(f *testing.F) {
	f.Add("random error")
	f.Add("context canceled")
	f.Add("context deadline exceeded")
	f.Add("empty key")
	f.Add("not leader")
	f.Add("not found")
	f.Add("unknown entry type")
	f.Add("replicate quorum unreachable")

	f.Fuzz(func(t *testing.T, input string) {
		inputErr := errors.New(input)
		gotErr := ToGrpcError(inputErr)

		if inputErr == nil {
			require.Nil(t, gotErr, "expected nil error but got %v", gotErr)
			return
		}

		gotStatus, _ := status.FromError(gotErr)
		require.NotNil(t, gotStatus, "status should never be nil")
		require.NotEqual(t, codes.OK, gotStatus.Code(), "unexpected gRPC code")
	})
}
