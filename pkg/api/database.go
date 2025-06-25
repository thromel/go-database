package api

import (
	"context"

	"github.com/thromel/go-database/pkg/transaction"
)

// Database represents the main database interface providing CRUD operations
// and transaction management for the Go Database Engine.
type Database interface {
	// Open initializes a database connection with the given path and configuration.
	// Returns an error if the database cannot be opened or initialized.
	Open(path string, config *Config) error

	// Close gracefully shuts down the database, ensuring all pending operations
	// are completed and resources are properly released.
	Close() error

	// Begin starts a new transaction and returns a Transaction interface.
	// Transactions provide ACID guarantees for database operations.
	Begin() (transaction.Transaction, error)

	// BeginWithContext starts a new transaction with the given context.
	// The transaction will be cancelled if the context is cancelled.
	BeginWithContext(ctx context.Context) (transaction.Transaction, error)

	// Put stores a key-value pair in the database.
	// This operation is atomic and will be immediately visible to other operations.
	Put(key []byte, value []byte) error

	// Get retrieves the value associated with the given key.
	// Returns ErrKeyNotFound if the key does not exist.
	Get(key []byte) ([]byte, error)

	// Delete removes the key-value pair from the database.
	// Returns ErrKeyNotFound if the key does not exist.
	Delete(key []byte) error

	// Exists checks if a key exists in the database without retrieving its value.
	Exists(key []byte) (bool, error)

	// Stats returns database statistics including size, number of keys, etc.
	Stats() (*DatabaseStats, error)
}

// DatabaseStats contains various statistics about the database state.
type DatabaseStats struct {
	// KeyCount is the total number of keys in the database
	KeyCount int64

	// DataSize is the total size of stored data in bytes
	DataSize int64

	// IndexSize is the total size of indexes in bytes
	IndexSize int64

	// PageCount is the total number of pages allocated
	PageCount int64

	// FreePageCount is the number of free pages available for reuse
	FreePageCount int64

	// TransactionCount is the number of active transactions
	TransactionCount int64
}
