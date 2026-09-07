package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrImpersonationNotAllowed = infraerrors.Forbidden("IMPERSONATION_NOT_ALLOWED", "only active administrators can switch to active regular users")

// ImpersonateUser creates a separate user session without changing either account.
func (s *AuthService) ImpersonateUser(ctx context.Context, actorID, targetID int64) (*TokenPair, *User, error) {
	actor, err := s.userRepo.GetByID(ctx, actorID)
	if err != nil {
		return nil, nil, err
	}
	if actor == nil || !actor.IsActive() || !actor.IsAdmin() || actorID == targetID {
		return nil, nil, ErrImpersonationNotAllowed
	}

	user, err := s.userRepo.GetByID(ctx, targetID)
	if err != nil {
		return nil, nil, err
	}
	if user == nil || !user.IsActive() || user.Role != RoleUser {
		return nil, nil, ErrImpersonationNotAllowed
	}

	pair, err := s.GenerateTokenPair(ctx, user, "")
	if err != nil {
		return nil, nil, err
	}
	return pair, user, nil
}
