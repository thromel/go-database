// Package storage provides persistent storage engine implementation that integrates
// page management, B+ trees, buffer pool, and file storage for durable data persistence.
package storage

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/thromel/go-database/pkg/storage/btree"
	"github.com/thromel/go-database/pkg/storage/buffer"
	"github.com/thromel/go-database/pkg/storage/file"
	"github.com/thromel/go-database/pkg/storage/page"
	"github.com/thromel/go-database/pkg/utils"
)

// PersistentEngine implements the StorageEngine interface with persistent disk storage.
// It integrates B+ trees, buffer pool, page manager, and file storage for ACID compliance.
type PersistentEngine struct {
	// Configuration
	config *PersistentConfig

	// Core components
	fileManager *file.FileManager
	pageManager *page.Manager
	bufferPool  *buffer.BufferPool
	btree       *btree.BPlusTree

	// State management
	mu     sync.RWMutex
	closed atomic.Bool

	// Statistics
	stats StorageStats
}

// PersistentConfig holds configuration for the persistent storage engine.
type PersistentConfig struct {
	// FilePath is the path to the database file
	FilePath string

	// BufferPoolSize is the number of pages to keep in memory
	BufferPoolSize int

	// BTreeConfig holds B+ tree configuration
	BTreeConfig *btree.Config

	// FileConfig holds file manager configuration
	FileConfig *file.Config

	// SyncOnWrite forces sync after each write operation
	SyncOnWrite bool

	// EnableIntegrityChecks performs startup integrity validation
	EnableIntegrityChecks bool
}

// DefaultPersistentConfig returns a default configuration for persistent storage.
func DefaultPersistentConfig() *PersistentConfig {
	return &PersistentConfig{
		FilePath:              "database.godb",
		BufferPoolSize:        1024,
		BTreeConfig:          btree.DefaultConfig(),
		FileConfig:           file.DefaultConfig(),
		SyncOnWrite:          true,
		EnableIntegrityChecks: true,
	}
}

// NewPersistentEngine creates a new persistent storage engine.
func NewPersistentEngine(config *PersistentConfig) (*PersistentEngine, error) {
	if config == nil {
		config = DefaultPersistentConfig()
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	engine := &PersistentEngine{
		config: config,
	}

	// Initialize components in dependency order
	if err := engine.initializeComponents(); err != nil {
		engine.cleanup()
		return nil, fmt.Errorf("failed to initialize storage engine: %w", err)
	}

	// Perform startup integrity checks if enabled
	if config.EnableIntegrityChecks {
		if err := engine.performStartupChecks(); err != nil {
			engine.cleanup()
			return nil, fmt.Errorf("startup integrity check failed: %w", err)
		}
	}

	return engine, nil
}

// validateConfig validates the persistent storage configuration.
func validateConfig(config *PersistentConfig) error {
	if config.FilePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	if config.BufferPoolSize <= 0 {
		return fmt.Errorf("buffer pool size must be positive, got %d", config.BufferPoolSize)
	}

	if config.BTreeConfig == nil {
		return fmt.Errorf("B+ tree configuration cannot be nil")
	}

	if config.FileConfig == nil {
		return fmt.Errorf("file configuration cannot be nil")
	}

	return nil
}

// initializeComponents initializes all storage components in the correct order.
func (pe *PersistentEngine) initializeComponents() error {
	var err error

	// 1. Initialize file manager
	pe.fileManager, err = file.NewFileManager(pe.config.FilePath, pe.config.FileConfig)
	if err != nil {
		return fmt.Errorf("failed to create file manager: %w", err)
	}

	// 2. Initialize page manager
	pe.pageManager = page.NewManager()

	// 3. Initialize buffer pool
	pe.bufferPool = buffer.NewBufferPool(pe.config.BufferPoolSize, pe.pageManager)

	// 4. Initialize B+ tree
	pe.btree, err = btree.NewBPlusTree(pe.pageManager, pe.config.BTreeConfig)
	if err != nil {
		return fmt.Errorf("failed to create B+ tree: %w", err)
	}

	return nil
}

// performStartupChecks performs integrity validation during startup.
func (pe *PersistentEngine) performStartupChecks() error {
	// Check file integrity
	if err := pe.fileManager.CheckIntegrity(); err != nil {
		return fmt.Errorf("file integrity check failed: %w", err)
	}

	// Additional checks could include:
	// - B+ tree structure validation
	// - Cross-reference file pages with page manager
	// - Verify buffer pool initialization

	return nil
}

// cleanup releases resources during initialization failures.
func (pe *PersistentEngine) cleanup() {
	if pe.bufferPool != nil {
		_ = pe.bufferPool.Close()
	}
	if pe.fileManager != nil {
		_ = pe.fileManager.Close()
	}
}

// Get retrieves the value associated with the given key.
func (pe *PersistentEngine) Get(key []byte) ([]byte, error) {
	if pe.closed.Load() {
		return nil, utils.ErrDatabaseClosed
	}

	if len(key) == 0 {
		return nil, utils.ErrInvalidKey
	}

	pe.mu.RLock()
	defer pe.mu.RUnlock()

	// Get from B+ tree
	value, err := pe.btree.Get(key)
	if err != nil {
		// Convert B+ tree's ErrKeyNotFound to utils.ErrKeyNotFound
		if err.Error() == "key not found" {
			atomic.AddInt64(&pe.stats.CacheMissCount, 1)
			return nil, utils.ErrKeyNotFound
		}
		return nil, err
	}

	// Update statistics
	atomic.AddInt64(&pe.stats.ReadCount, 1)
	atomic.AddInt64(&pe.stats.BytesRead, int64(len(value)))
	atomic.AddInt64(&pe.stats.CacheHitCount, 1)

	return value, nil
}

// Put stores a key-value pair in the storage engine.
func (pe *PersistentEngine) Put(key []byte, value []byte) error {
	if pe.closed.Load() {
		return utils.ErrDatabaseClosed
	}

	if len(key) == 0 {
		return utils.ErrInvalidKey
	}

	pe.mu.Lock()
	defer pe.mu.Unlock()

	// Store in B+ tree
	if err := pe.btree.Put(key, value); err != nil {
		return err
	}

	// Sync to disk if enabled
	if pe.config.SyncOnWrite {
		if err := pe.syncInternal(); err != nil {
			return fmt.Errorf("failed to sync after write: %w", err)
		}
	}

	// Update statistics
	atomic.AddInt64(&pe.stats.WriteCount, 1)
	atomic.AddInt64(&pe.stats.BytesWritten, int64(len(key)+len(value)))

	return nil
}

// Delete removes the key-value pair from the storage engine.
func (pe *PersistentEngine) Delete(key []byte) error {
	if pe.closed.Load() {
		return utils.ErrDatabaseClosed
	}

	if len(key) == 0 {
		return utils.ErrInvalidKey
	}

	pe.mu.Lock()
	defer pe.mu.Unlock()

	// Delete from B+ tree
	if err := pe.btree.Delete(key); err != nil {
		// Convert B+ tree's ErrKeyNotFound to utils.ErrKeyNotFound
		if err.Error() == "key not found" {
			return utils.ErrKeyNotFound
		}
		return err
	}

	// Sync to disk if enabled
	if pe.config.SyncOnWrite {
		if err := pe.syncInternal(); err != nil {
			return fmt.Errorf("failed to sync after delete: %w", err)
		}
	}

	// Update statistics
	atomic.AddInt64(&pe.stats.DeleteCount, 1)

	return nil
}

// Exists checks if a key exists in the storage without retrieving its value.
func (pe *PersistentEngine) Exists(key []byte) (bool, error) {
	if pe.closed.Load() {
		return false, utils.ErrDatabaseClosed
	}

	if len(key) == 0 {
		return false, utils.ErrInvalidKey
	}

	pe.mu.RLock()
	defer pe.mu.RUnlock()

	return pe.btree.Exists(key)
}

// NewIterator creates a new iterator for traversing key-value pairs.
func (pe *PersistentEngine) NewIterator(start, end []byte) Iterator {
	if pe.closed.Load() {
		return &ErrorIterator{err: utils.ErrDatabaseClosed}
	}

	pe.mu.RLock()
	defer pe.mu.RUnlock()

	// Create B+ tree iterator
	// Note: This is a simplified implementation. A full implementation would
	// need to integrate with the buffer pool for efficient page loading.
	// For now, return an error iterator to indicate this feature is not yet implemented.
	return &ErrorIterator{err: fmt.Errorf("iterator not yet implemented for persistent storage")}
}

// Size returns the approximate number of key-value pairs in the storage.
func (pe *PersistentEngine) Size() (int64, error) {
	if pe.closed.Load() {
		return 0, utils.ErrDatabaseClosed
	}

	pe.mu.RLock()
	defer pe.mu.RUnlock()

	stats := pe.btree.Stats()
	return int64(stats.NumKeys), nil
}

// Sync ensures all pending writes are flushed to stable storage.
func (pe *PersistentEngine) Sync() error {
	if pe.closed.Load() {
		return utils.ErrDatabaseClosed
	}

	pe.mu.Lock()
	defer pe.mu.Unlock()

	return pe.syncInternal()
}

// syncInternal performs the actual sync operation (assumes lock is held).
func (pe *PersistentEngine) syncInternal() error {
	// 1. Flush buffer pool dirty pages
	if err := pe.bufferPool.FlushAllPages(); err != nil {
		return fmt.Errorf("failed to flush buffer pool: %w", err)
	}

	// 2. Sync file manager to disk
	if err := pe.fileManager.Sync(); err != nil {
		return fmt.Errorf("failed to sync file manager: %w", err)
	}

	return nil
}

// Close releases all resources held by the storage engine.
func (pe *PersistentEngine) Close() error {
	if pe.closed.Swap(true) {
		return nil // Already closed
	}

	pe.mu.Lock()
	defer pe.mu.Unlock()

	var errs []error

	// Perform final sync
	if err := pe.syncInternal(); err != nil {
		errs = append(errs, fmt.Errorf("final sync failed: %w", err))
	}

	// Close components in reverse dependency order
	if pe.bufferPool != nil {
		if err := pe.bufferPool.Close(); err != nil {
			errs = append(errs, fmt.Errorf("buffer pool close failed: %w", err))
		}
	}

	if pe.fileManager != nil {
		if err := pe.fileManager.Close(); err != nil {
			errs = append(errs, fmt.Errorf("file manager close failed: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	return nil
}

// GetStats returns storage engine statistics.
func (pe *PersistentEngine) GetStats() StorageStats {
	stats := StorageStats{
		ReadCount:      atomic.LoadInt64(&pe.stats.ReadCount),
		WriteCount:     atomic.LoadInt64(&pe.stats.WriteCount),
		DeleteCount:    atomic.LoadInt64(&pe.stats.DeleteCount),
		BytesRead:      atomic.LoadInt64(&pe.stats.BytesRead),
		BytesWritten:   atomic.LoadInt64(&pe.stats.BytesWritten),
		CacheHitCount:  atomic.LoadInt64(&pe.stats.CacheHitCount),
		CacheMissCount: atomic.LoadInt64(&pe.stats.CacheMissCount),
	}

	// Add buffer pool statistics
	if pe.bufferPool != nil {
		bufferStats := pe.bufferPool.GetStatistics()
		stats.CacheHitCount += bufferStats.CacheHits
		stats.CacheMissCount += bufferStats.CacheMisses
	}

	return stats
}

// GetFileManager returns the file manager (for testing/debugging).
func (pe *PersistentEngine) GetFileManager() *file.FileManager {
	return pe.fileManager
}

// GetBufferPool returns the buffer pool (for testing/debugging).
func (pe *PersistentEngine) GetBufferPool() *buffer.BufferPool {
	return pe.bufferPool
}

// GetBTree returns the B+ tree (for testing/debugging).
func (pe *PersistentEngine) GetBTree() *btree.BPlusTree {
	return pe.btree
}

// ErrorIterator is a simple iterator that always returns an error.
type ErrorIterator struct {
	err error
}

func (ei *ErrorIterator) Valid() bool                { return false }
func (ei *ErrorIterator) Next() bool                 { return false }
func (ei *ErrorIterator) Key() []byte                { return nil }
func (ei *ErrorIterator) Value() []byte              { return nil }
func (ei *ErrorIterator) Seek(target []byte)         {}
func (ei *ErrorIterator) SeekToFirst()               {}
func (ei *ErrorIterator) SeekToLast()                {}
func (ei *ErrorIterator) Error() error               { return ei.err }
func (ei *ErrorIterator) Close() error               { return nil }