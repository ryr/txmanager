package txmanager

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sync"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

type TxManager struct {
	callbacks sync.Map
}

func New() *TxManager {
	return &TxManager{}
}

func (t *TxManager) StoreCommitCallback(tx pgx.Tx, callback func() error) {
	if callback == nil {
		return
	}

	val, _ := t.callbacks.LoadOrStore(tx, &[]func() error{})
	callbacks := val.(*[]func() error)

	newSlice := append([]func() error{}, *callbacks...)
	newSlice = append(newSlice, callback)
	t.callbacks.Store(tx, &newSlice)
}

func (t *TxManager) CommitAndExecuteCallbacks(ctx context.Context, db Committer, tx pgx.Tx, operationErr error) error {
	r := recover()

	err := db.CommitRollback(tx, ctx, r, operationErr)
	if err == nil && operationErr == nil && r == nil {
		err = t.executeCallbacks(tx, ctx)
	}
	t.callbacks.Delete(tx)

	if operationErr != nil {
		return operationErr
	}

	return err
}

func (t *TxManager) executeCallbacks(tx pgx.Tx, ctx context.Context) error {
	val, ok := t.callbacks.Load(tx)
	if !ok {
		return nil
	}
	t.callbacks.Delete(tx)

	callbacks := *val.(*[]func() error)
	if len(callbacks) == 0 {
		return nil
	}

	g, _ := errgroup.WithContext(ctx)
	for i, callback := range callbacks {
		g.Go(func() error {
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					wrapped := fmt.Errorf("callback #%d panic: %v\n%s", i, r, stack)
					log.Printf("[txmanager] %v", wrapped)
				}
			}()

			if err := callback(); err != nil {
				wrapped := fmt.Errorf("callback #%d error: %w", i, err)
				log.Printf("[txmanager] %v", wrapped)

				return wrapped
			}

			return nil
		})
	}

	return g.Wait()
}
