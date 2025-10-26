package txmanager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sync"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

type txCallbacks struct {
	mu        sync.Mutex
	callbacks []func() error
}

func (c *txCallbacks) append(fn func() error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = append(c.callbacks, fn)
}

func (c *txCallbacks) snapshot() []func() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]func() error, len(c.callbacks))
	copy(out, c.callbacks)

	return out
}

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

	key := txKey(tx)
	val, _ := t.callbacks.LoadOrStore(key, &txCallbacks{})
	tc := val.(*txCallbacks)
	tc.append(callback)
}

func (t *TxManager) CommitAndExecuteCallbacks(ctx context.Context, db Committer, tx pgx.Tx, recoverResult any, operationErr error) error {
	err := db.CommitRollback(tx, ctx, recoverResult, operationErr)
	if err == nil && operationErr == nil {
		err = t.executeCallbacks(tx, ctx)
	}

	return errors.Join(operationErr, err)
}

func (t *TxManager) executeCallbacks(tx pgx.Tx, ctx context.Context) error {
	key := txKey(tx)
	val, ok := t.callbacks.LoadAndDelete(key)
	if !ok {
		return nil
	}

	tc := val.(*txCallbacks)
	callbacks := tc.snapshot()
	if len(callbacks) == 0 {
		return nil
	}

	g, _ := errgroup.WithContext(ctx)
	for i, callback := range callbacks {
		i, callback := i, callback

		g.Go(func() error {
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					wrapped := fmt.Errorf("callback #%d panic: %v\n%s", i, r, stack)
					log.Printf("[txmanager] %v", wrapped)
					// do not return error from recover here; we log and allow errgroup to continue.
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

func txKey(tx pgx.Tx) string {
	if tx == nil {
		return "nil-tx"
	}

	return fmt.Sprintf("%p", tx)
}
