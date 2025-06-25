package storage

// StorageEngine defines the interface for the underlying storage mechanism.
// This abstraction allows for different storage backends (memory, disk, etc.)
// while maintaining a consistent API for the database layer.
type StorageEngine interface {
	// Get retrieves the value associated with the given key.
	// Returns ErrKeyNotFound if the key does not exist.
	Get(key []byte) ([]byte, error)

	// Put stores a key-value pair in the storage engine.
	// If the key already exists, its value will be updated.
	Put(key []byte, value []byte) error

	// Delete removes the key-value pair from the storage engine.
	// Returns ErrKeyNotFound if the key does not exist.
	Delete(key []byte) error

	// Exists checks if a key exists in the storage without retrieving its value.
	// This can be more efficient than Get when only existence needs to be checked.
	Exists(key []byte) (bool, error)

	// NewIterator creates a new iterator for traversing key-value pairs.
	// The iterator will include keys in the range [start, end).
	// If start is nil, iteration begins from the first key.
	// If end is nil, iteration continues to the last key.
	NewIterator(start, end []byte) Iterator

	// Size returns the approximate number of key-value pairs in the storage.
	Size() (int64, error)

	// Close releases any resources held by the storage engine.
	Close() error

	// Sync ensures all pending writes are flushed to stable storage.
	Sync() error
}

// Iterator provides sequential access to key-value pairs in the storage engine.
// Iterators maintain their position and can be used to traverse data efficiently.
type Iterator interface {
	// Valid returns true if the iterator is positioned at a valid key-value pair.
	Valid() bool

	// Next advances the iterator to the next key-value pair.
	// Returns false if there are no more pairs to iterate over.
	Next() bool

	// Key returns the current key. Only valid when Valid() returns true.
	Key() []byte

	// Value returns the current value. Only valid when Valid() returns true.
	Value() []byte

	// Seek positions the iterator at the first key that is >= target.
	// If no such key exists, the iterator becomes invalid.
	Seek(target []byte)

	// SeekToFirst positions the iterator at the first key-value pair.
	SeekToFirst()

	// SeekToLast positions the iterator at the last key-value pair.
	SeekToLast()

	// Error returns any error encountered during iteration.
	Error() error

	// Close releases resources associated with the iterator.
	Close() error
}

// StorageStats contains statistics about the storage engine performance and state.
type StorageStats struct {
	// ReadCount is the total number of read operations performed
	ReadCount int64

	// WriteCount is the total number of write operations performed
	WriteCount int64

	// DeleteCount is the total number of delete operations performed
	DeleteCount int64

	// BytesRead is the total number of bytes read
	BytesRead int64

	// BytesWritten is the total number of bytes written
	BytesWritten int64

	// CacheHitCount is the number of cache hits (if applicable)
	CacheHitCount int64

	// CacheMissCount is the number of cache misses (if applicable)
	CacheMissCount int64
}
