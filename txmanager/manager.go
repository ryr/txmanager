package txmanager

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Committer interface {
	CommitRollback(tx pgx.Tx, ctx context.Context, r interface{}, operationErr error) error
}

type Manager interface {
	StoreCommitCallback(tx pgx.Tx, callback func() error)
	CommitAndExecuteCallbacks(ctx context.Context, db Committer, tx pgx.Tx, recoverResult any, operationErr error) error
}

var _ Manager = &TxManager{}
