package storage

import (
	"bytes"
	"sort"
	"sync"

	"github.com/romel/go-database/pkg/utils"
)

// MemoryEngine implements the StorageEngine interface using an in-memory map.
// It provides thread-safe key-value operations with proper synchronization.
type MemoryEngine struct {
	// data stores the key-value pairs
	data map[string][]byte

	// mu protects concurrent access to the data map
	mu sync.RWMutex

	// closed indicates if the engine has been closed
	closed bool
}

// NewMemoryEngine creates a new in-memory storage engine.
func NewMemoryEngine() *MemoryEngine {
	return &MemoryEngine{
		data:   make(map[string][]byte),
		closed: false,
	}
}

// Get retrieves the value associated with the given key.
func (m *MemoryEngine) Get(key []byte) ([]byte, error) {
	if err := m.validateKey(key); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, utils.ErrDatabaseClosed
	}

	value, exists := m.data[string(key)]
	if !exists {
		return nil, utils.ErrKeyNotFound
	}

	// Return a copy to prevent external modification
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// Put stores a key-value pair in the storage engine.
func (m *MemoryEngine) Put(key []byte, value []byte) error {
	if err := m.validateKey(key); err != nil {
		return err
	}

	if err := m.validateValue(value); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return utils.ErrDatabaseClosed
	}

	// Store a copy to prevent external modification
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	m.data[string(key)] = valueCopy

	return nil
}

// Delete removes the key-value pair from the storage engine.
func (m *MemoryEngine) Delete(key []byte) error {
	if err := m.validateKey(key); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return utils.ErrDatabaseClosed
	}

	keyStr := string(key)
	if _, exists := m.data[keyStr]; !exists {
		return utils.ErrKeyNotFound
	}

	delete(m.data, keyStr)
	return nil
}

// Exists checks if a key exists in the storage.
func (m *MemoryEngine) Exists(key []byte) (bool, error) {
	if err := m.validateKey(key); err != nil {
		return false, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return false, utils.ErrDatabaseClosed
	}

	_, exists := m.data[string(key)]
	return exists, nil
}

// NewIterator creates a new iterator for traversing key-value pairs.
func (m *MemoryEngine) NewIterator(start, end []byte) Iterator {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a snapshot of keys for consistent iteration
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		// Filter keys based on range
		if start != nil && bytes.Compare([]byte(k), start) < 0 {
			continue
		}
		if end != nil && bytes.Compare([]byte(k), end) >= 0 {
			continue
		}
		keys = append(keys, k)
	}

	// Sort keys for consistent ordering
	sort.Strings(keys)

	// Create a snapshot of the data for the iterator
	snapshot := make(map[string][]byte)
	for _, k := range keys {
		value := m.data[k]
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)
		snapshot[k] = valueCopy
	}

	return &MemoryIterator{
		keys:     keys,
		data:     snapshot,
		position: -1,
		closed:   false,
	}
}

// Size returns the approximate number of key-value pairs in the storage.
func (m *MemoryEngine) Size() (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return 0, utils.ErrDatabaseClosed
	}

	return int64(len(m.data)), nil
}

// Close releases any resources held by the storage engine.
func (m *MemoryEngine) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return utils.ErrDatabaseClosed
	}

	// Clear the data map
	m.data = nil
	m.closed = true

	return nil
}

// Sync ensures all pending writes are flushed to stable storage.
// For in-memory storage, this is a no-op.
func (m *MemoryEngine) Sync() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return utils.ErrDatabaseClosed
	}

	// No-op for memory storage
	return nil
}

// Stats returns the current storage statistics.
func (m *MemoryEngine) Stats() StorageStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return basic stats without race-prone counters
	return StorageStats{
		// Note: ReadCount, WriteCount, DeleteCount, and BytesRead/Written
		// are not tracked to avoid race conditions in concurrent operations
	}
}

// validateKey checks if a key is valid.
func (m *MemoryEngine) validateKey(key []byte) error {
	if len(key) == 0 {
		return utils.ErrInvalidKey
	}

	// Check maximum key size (64KB limit)
	if len(key) > 65536 {
		return utils.ErrKeyTooLarge
	}

	return nil
}

// validateValue checks if a value is valid.
func (m *MemoryEngine) validateValue(value []byte) error {
	if value == nil {
		return utils.ErrInvalidValue
	}

	// Check maximum value size (16MB limit)
	if len(value) > 16*1024*1024 {
		return utils.ErrValueTooLarge
	}

	return nil
}
