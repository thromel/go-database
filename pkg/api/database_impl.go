package api

import (
	"context"
	"sync"

	"github.com/romel/go-database/pkg/storage"
	"github.com/romel/go-database/pkg/transaction"
	"github.com/romel/go-database/pkg/utils"
)

// DatabaseImpl implements the Database interface.
type DatabaseImpl struct {
	// config holds the database configuration
	config *Config

	// path is the database file path
	path string

	// storage is the underlying storage engine
	storage storage.StorageEngine

	// txnManager handles transaction lifecycle
	txnManager transaction.TransactionManager

	// mu protects concurrent access to database state
	mu sync.RWMutex

	// closed indicates if the database is closed
	closed bool

	// stats tracks database statistics
	stats DatabaseStats
}

// Open initializes a database connection with the given path and configuration.
func Open(path string, config *Config) (Database, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Set the path in config if not already set
	if config.Path == "" {
		config.Path = path
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, utils.NewDatabaseErrorWithPath("open", path, err)
	}

	db := &DatabaseImpl{
		config: config,
		path:   path,
		closed: false,
	}

	// Initialize storage engine (for now, use memory engine)
	// TODO: In future sprints, add disk-based storage
	db.storage = storage.NewMemoryEngine()

	// TODO: Initialize transaction manager in future sprints
	// db.txnManager = transaction.NewTransactionManager(db.storage)

	return db, nil
}

// Open implements the Database interface Open method.
func (db *DatabaseImpl) Open(path string, config *Config) error {
	// This method is for interface compatibility
	// The actual opening is done by the package-level Open function
	return utils.NewDatabaseError("open", utils.ErrInvalidConfig)
}

// Close gracefully shuts down the database.
func (db *DatabaseImpl) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return utils.ErrDatabaseClosed
	}

	var lastErr error

	// Close transaction manager if initialized
	if db.txnManager != nil {
		if err := db.txnManager.Close(); err != nil {
			lastErr = err
		}
	}

	// Close storage engine
	if db.storage != nil {
		if err := db.storage.Close(); err != nil {
			lastErr = err
		}
	}

	db.closed = true

	if lastErr != nil {
		return utils.NewDatabaseErrorWithPath("close", db.path, lastErr)
	}

	return nil
}

// Begin starts a new transaction.
func (db *DatabaseImpl) Begin() (transaction.Transaction, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, utils.ErrDatabaseClosed
	}

	// TODO: Implement transaction manager in future sprints
	// For now, return an error indicating transactions are not yet implemented
	return nil, utils.NewDatabaseError("begin", utils.ErrTransactionNotFound)
}

// BeginWithContext starts a new transaction with context.
func (db *DatabaseImpl) BeginWithContext(ctx context.Context) (transaction.Transaction, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, utils.ErrDatabaseClosed
	}

	// TODO: Implement transaction manager with context support in future sprints
	return nil, utils.NewDatabaseError("begin_with_context", utils.ErrTransactionNotFound)
}

// Put stores a key-value pair in the database.
func (db *DatabaseImpl) Put(key []byte, value []byte) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return utils.ErrDatabaseClosed
	}

	err := db.storage.Put(key, value)
	if err != nil {
		return utils.NewDatabaseErrorWithKey("put", key, err)
	}

	// Update stats
	db.stats.KeyCount++

	return nil
}

// Get retrieves the value associated with the given key.
func (db *DatabaseImpl) Get(key []byte) ([]byte, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, utils.ErrDatabaseClosed
	}

	value, err := db.storage.Get(key)
	if err != nil {
		return nil, utils.NewDatabaseErrorWithKey("get", key, err)
	}

	return value, nil
}

// Delete removes the key-value pair from the database.
func (db *DatabaseImpl) Delete(key []byte) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return utils.ErrDatabaseClosed
	}

	err := db.storage.Delete(key)
	if err != nil {
		return utils.NewDatabaseErrorWithKey("delete", key, err)
	}

	// Update stats
	db.stats.KeyCount--

	return nil
}

// Exists checks if a key exists in the database.
func (db *DatabaseImpl) Exists(key []byte) (bool, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return false, utils.ErrDatabaseClosed
	}

	exists, err := db.storage.Exists(key)
	if err != nil {
		return false, utils.NewDatabaseErrorWithKey("exists", key, err)
	}

	return exists, nil
}

// Stats returns database statistics.
func (db *DatabaseImpl) Stats() (*DatabaseStats, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, utils.ErrDatabaseClosed
	}

	// Get current key count from storage
	keyCount, err := db.storage.Size()
	if err != nil {
		return nil, utils.NewDatabaseErrorWithPath("stats", db.path, err)
	}

	// Create a copy of stats to return
	stats := db.stats
	stats.KeyCount = keyCount
	stats.TransactionCount = 0 // TODO: Get from transaction manager

	return &stats, nil
}

// GetStorageEngine returns the underlying storage engine (for testing).
func (db *DatabaseImpl) GetStorageEngine() storage.StorageEngine {
	return db.storage
}

// GetConfig returns the database configuration (for testing).
func (db *DatabaseImpl) GetConfig() *Config {
	return db.config
}

// IsClosed returns true if the database is closed (for testing).
func (db *DatabaseImpl) IsClosed() bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.closed
}