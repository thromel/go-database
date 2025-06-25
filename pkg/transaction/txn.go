// Package transaction provides ACID transaction management for the database engine.
// It includes support for different isolation levels, deadlock detection, and
// automatic retry policies for handling transaction conflicts.
package transaction

import (
	"context"
	"time"
)

// Transaction represents a database transaction that provides ACID guarantees.
// All operations within a transaction are atomic - they either all succeed or all fail.
type Transaction interface {
	// Put stores a key-value pair within the transaction context.
	// The change is not visible to other transactions until Commit() is called.
	Put(key []byte, value []byte) error

	// Get retrieves the value associated with the given key within the transaction.
	// This will return the most recent value set within this transaction,
	// or the committed value if no changes have been made to this key.
	Get(key []byte) ([]byte, error)

	// Delete removes the key-value pair within the transaction context.
	// The deletion is not visible to other transactions until Commit() is called.
	Delete(key []byte) error

	// Exists checks if a key exists within the transaction context.
	Exists(key []byte) (bool, error)

	// Commit applies all changes made within this transaction to the database.
	// If the commit fails, the transaction is automatically rolled back.
	Commit() error

	// Rollback discards all changes made within this transaction.
	// After rollback, the transaction cannot be used for further operations.
	Rollback() error

	// ID returns the unique identifier for this transaction.
	ID() ID

	// IsReadOnly returns true if this transaction only performs read operations.
	IsReadOnly() bool

	// Context returns the context associated with this transaction.
	Context() context.Context

	// SetDeadline sets a deadline for the transaction.
	// The transaction will be automatically rolled back if it exceeds this deadline.
	SetDeadline(deadline time.Time) error
}

// ID represents a unique identifier for a transaction.
type ID uint64

// Manager handles the lifecycle and coordination of transactions.
type Manager interface {
	// Begin starts a new transaction with default options.
	Begin() (Transaction, error)

	// BeginWithContext starts a new transaction with the given context.
	BeginWithContext(ctx context.Context) (Transaction, error)

	// BeginWithOptions starts a new transaction with specific options.
	BeginWithOptions(opts *TransactionOptions) (Transaction, error)

	// GetTransaction retrieves an active transaction by its ID.
	GetTransaction(id ID) (Transaction, error)

	// ActiveTransactions returns the number of currently active transactions.
	ActiveTransactions() int64

	// Close shuts down the transaction manager and rolls back any active transactions.
	Close() error
}

// TransactionOptions configures how a transaction should behave.
type TransactionOptions struct {
	// ReadOnly indicates whether this transaction will only perform read operations.
	// Read-only transactions can have better performance characteristics.
	ReadOnly bool

	// IsolationLevel specifies the isolation level for this transaction.
	IsolationLevel IsolationLevel

	// Timeout specifies the maximum duration this transaction can remain active.
	Timeout time.Duration

	// RetryPolicy specifies how transaction conflicts should be handled.
	RetryPolicy *RetryPolicy
}

// IsolationLevel defines the isolation level for transactions.
type IsolationLevel int

const (
	// ReadUncommitted allows dirty reads (lowest isolation, highest performance)
	ReadUncommitted IsolationLevel = iota

	// ReadCommitted prevents dirty reads but allows non-repeatable reads
	ReadCommitted

	// RepeatableRead prevents dirty reads and non-repeatable reads
	RepeatableRead

	// Serializable provides the highest isolation level (highest consistency, lowest performance)
	Serializable
)

// RetryPolicy defines how transaction conflicts should be handled.
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// InitialDelay is the initial delay between retry attempts
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retry attempts
	MaxDelay time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64
}

// TransactionStats contains statistics about transaction performance.
type TransactionStats struct {
	// TotalTransactions is the total number of transactions started
	TotalTransactions int64

	// CommittedTransactions is the number of successfully committed transactions
	CommittedTransactions int64

	// RolledBackTransactions is the number of rolled back transactions
	RolledBackTransactions int64

	// ActiveTransactions is the current number of active transactions
	ActiveTransactions int64

	// AverageTransactionDuration is the average duration of committed transactions
	AverageTransactionDuration time.Duration

	// ConflictCount is the number of transaction conflicts encountered
	ConflictCount int64

	// DeadlockCount is the number of deadlocks detected and resolved
	DeadlockCount int64
}
