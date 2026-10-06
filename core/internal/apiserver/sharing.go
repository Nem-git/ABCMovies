package apiserver

import (
	"context"
	"fmt"

	"github.com/nem-git/abcmovies/core/internal/accounts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// checkMembers verifies that every user named in a sharing roster exists,
// so a typo fails loudly and names the user instead of silently granting the
// share to nobody. When no UserDirectory is armed the check is skipped — a
// minimal embedding that never arms it keeps the older behaviour.
func (s *Server) checkMembers(ctx context.Context, members []string) error {
	if s.userDirectory == nil {
		return nil
	}
	for _, m := range members {
		ok, err := s.userDirectory.HasUser(m)
		if err != nil {
			return status.Error(codes.Internal, "failed to verify member "+m)
		}
		if !ok {
			return status.Error(codes.InvalidArgument, "no such user: "+m)
		}
	}
	return nil
}

// validateSharing checks a sharing choice the same way for both entry points
// — link and update — so the two paths can never drift: the shape rules live
// in one place, and the member-existence check lives here.
func (s *Server) validateSharing(ctx context.Context, v accounts.Visibility, members []string) error {
	switch v {
	case accounts.VisibilityPrivate, accounts.VisibilityPublic:
		if len(members) != 0 {
			return status.Error(codes.InvalidArgument, "shared_with is only meaningful with shared visibility")
		}
	case accounts.VisibilityShared:
		if len(members) == 0 {
			return status.Error(codes.InvalidArgument, "shared visibility requires shared_with users")
		}
		if err := s.checkMembers(ctx, members); err != nil {
			return err
		}
	default:
		return status.Error(codes.InvalidArgument, fmt.Sprintf("unknown visibility %q", v))
	}
	return nil
}
