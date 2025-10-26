package txmanager

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
)

type mockCommitter struct {
	commitErr  error
	rolledBack atomic.Bool
}

func (m *mockCommitter) CommitRollback(_ pgx.Tx, _ context.Context, r interface{}, operationErr error) error {
	if r != nil || operationErr != nil {
		m.rolledBack.Store(true)
	}
	return m.commitErr
}

func TestTxManager_CommitAndExecuteCallbacks(t *testing.T) {
	m := New()
	db := &mockCommitter{}
	var tx pgx.Tx

	var called atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)

	m.StoreCommitCallback(tx, func() error {
		defer wg.Done()
		called.Store(true)
		return nil
	})

	if err := m.CommitAndExecuteCallbacks(context.Background(), db, tx, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wg.Wait()

	if !called.Load() {
		t.Fatalf("callback not called")
	}
}

func TestTxManager_CallbackPanic(t *testing.T) {
	m := New()
	db := &mockCommitter{}
	var tx pgx.Tx

	var wg sync.WaitGroup
	wg.Add(1)

	m.StoreCommitCallback(tx, func() error {
		defer wg.Done()
		panic("simulated panic in callback")
	})

	if err := m.CommitAndExecuteCallbacks(context.Background(), db, tx, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wg.Wait()
}

func TestTxManager_OperationError(t *testing.T) {
	m := New()
	db := &mockCommitter{}
	var tx pgx.Tx

	var called atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)

	m.StoreCommitCallback(tx, func() error {
		defer wg.Done()
		called.Store(true)
		return nil
	})

	err := m.CommitAndExecuteCallbacks(context.Background(), db, tx, nil, errOperation)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		t.Fatalf("callback executed unexpectedly")
	default:
	}
}

var errOperation = &operationError{}

type operationError struct{}

func (e *operationError) Error() string {
	return "simulated operation error"
}
