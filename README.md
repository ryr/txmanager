# txmanager  
**Safe Transaction Manager with Post-Commit Callbacks for Go**

![Go](https://img.shields.io/badge/go-1.24+-blue.svg)
![Tests](https://github.com/ryr/txmanager/actions/workflows/tests.yml/badge.svg)

## 📘 Overview
`txmanager` is a lightweight, concurrency-safe Go package that simplifies transaction management and post-commit logic execution.

It provides a clean abstraction for handling **commit**, **rollback**, and **post-commit callbacks**, ensuring that registered functions execute **only after a successful commit** — never after a rollback or panic.

## 🚀 Features
- ✅ Thread-safe with `sync.Map` and atomic operations  
- 🧩 Works seamlessly with `pgx.Tx` (or any compatible interface)  
- ⚡ Supports panic-safe commit/rollback logic  
- 🧠 Clean separation between transaction and post-commit behavior  
- 🧪 Race-tested with `go test -race`  

## 📦 Installation
```bash
go get github.com/ryr/txmanager
```

## 💡 Usage Example
```go
package main

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5"
    "github.com/ryr/txmanager"
    "go.uber.org/zap"
)

type CommitterImpl struct{}

func (CommitterImpl) CommitRollback(tx pgx.Tx, ctx context.Context, r interface{}, opErr error) error {
    if r != nil || opErr != nil {
        fmt.Println("rollback transaction")
        return tx.Rollback(ctx)
    }
    fmt.Println("commit transaction")
    return tx.Commit(ctx)
}

func main() {
    logger, _ := zap.NewDevelopment()
    manager := txmanager.New()

    var tx pgx.Tx // Assume it's initialized

    manager.StoreCommitCallback(tx, func() error {
        fmt.Println("post-commit callback executed!")
        return nil
    })

    db := CommitterImpl{}
    if err := manager.CommitAndExecuteCallbacks(context.Background(), db, tx, nil); err != nil {
        logger.Error("commit failed", zap.Error(err))
    }
}
```

## 🧩 API Overview

| Function | Description |
|-----------|--------------|
| `StoreCommitCallback(tx pgx.Tx, callback func() error)` | Registers a callback to be executed **after a successful commit**. |
| `CommitAndExecuteCallbacks(ctx, committer, tx, opErr)` | Executes commit/rollback and triggers stored callbacks. |
| `Committer` interface | Abstracts commit/rollback logic so the manager can be used with different database drivers. |

## 🧠 Why txmanager?
`txmanager` ensures post-commit side effects (cache invalidation, event publishing, etc.) run **only after a successful transaction** — never before or on rollback.

## ⚠️ Common Mistakes
| Mistake | Explanation |
|----------|--------------|
| ❌ Executing callbacks directly after `Commit()` | Use `StoreCommitCallback` instead — it ensures proper ordering. |
| ❌ Forgetting to clean up callbacks per transaction | `txmanager` automatically handles cleanup once callbacks are executed. |
| ❌ Using the same transaction across goroutines unsafely | Always keep `pgx.Tx` usage confined to one goroutine. |

## 💬 When to Use
✅ Suitable for:
- Event-driven architectures (outbox pattern, cache invalidation)  
- Systems where transactional side effects must be deterministic  
- Complex commit/rollback logic

❌ Not ideal for:
- Non-transactional operations  
- Long-running background jobs where transaction context is lost
