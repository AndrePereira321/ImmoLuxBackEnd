package database

import (
	"context"

	"immo-lux/internal/server_error"
)

// ownershipVerdict explains why an owner-scoped statement matched nothing: the
// row either does not exist (notFoundErr) or belongs to someone else
// (forbiddenErr). Every store's ownership check funnels through here so the
// two-step classification exists exactly once.
func ownershipVerdict(ctx context.Context, exists func(context.Context) (bool, error), queryCode string, notFoundErr, forbiddenErr *server_error.ServerError) error {
	found, err := exists(ctx)
	if err != nil {
		return server_error.Wrap(queryCode, "failed to check existence", err)
	}
	if !found {
		return notFoundErr
	}
	return forbiddenErr
}
