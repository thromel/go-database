package storage

import (
	"github.com/romel/go-database/pkg/utils"
)

// MemoryIterator implements the Iterator interface for in-memory storage.
type MemoryIterator struct {
	// keys holds the sorted keys for iteration
	keys []string

	// data holds a snapshot of the key-value pairs
	data map[string][]byte

	// position tracks the current position in the keys slice
	position int

	// closed indicates if the iterator has been closed
	closed bool

	// err holds any error encountered during iteration
	err error
}

// Valid returns true if the iterator is positioned at a valid key-value pair.
func (it *MemoryIterator) Valid() bool {
	if it.closed || it.err != nil {
		return false
	}

	return it.position >= 0 && it.position < len(it.keys)
}

// Next advances the iterator to the next key-value pair.
func (it *MemoryIterator) Next() bool {
	if it.closed {
		it.err = utils.ErrIteratorClosed
		return false
	}

	it.position++
	return it.Valid()
}

// Key returns the current key.
func (it *MemoryIterator) Key() []byte {
	if !it.Valid() {
		return nil
	}

	return []byte(it.keys[it.position])
}

// Value returns the current value.
func (it *MemoryIterator) Value() []byte {
	if !it.Valid() {
		return nil
	}

	key := it.keys[it.position]
	value, exists := it.data[key]
	if !exists {
		it.err = utils.ErrKeyNotFound
		return nil
	}

	// Return a copy to prevent external modification
	result := make([]byte, len(value))
	copy(result, value)
	return result
}

// Seek positions the iterator at the first key that is >= target.
func (it *MemoryIterator) Seek(target []byte) {
	if it.closed {
		it.err = utils.ErrIteratorClosed
		return
	}

	targetStr := string(target)

	// Binary search for the first key >= target
	left, right := 0, len(it.keys)
	for left < right {
		mid := (left + right) / 2
		if it.keys[mid] < targetStr {
			left = mid + 1
		} else {
			right = mid
		}
	}

	it.position = left
	if it.position >= len(it.keys) {
		// No key found >= target, position at end
		it.position = len(it.keys)
	}
}

// SeekToFirst positions the iterator at the first key-value pair.
func (it *MemoryIterator) SeekToFirst() {
	if it.closed {
		it.err = utils.ErrIteratorClosed
		return
	}

	it.position = 0
	if len(it.keys) == 0 {
		it.position = -1
	}
}

// SeekToLast positions the iterator at the last key-value pair.
func (it *MemoryIterator) SeekToLast() {
	if it.closed {
		it.err = utils.ErrIteratorClosed
		return
	}

	it.position = len(it.keys) - 1
}

// Error returns any error encountered during iteration.
func (it *MemoryIterator) Error() error {
	return it.err
}

// Close releases resources associated with the iterator.
func (it *MemoryIterator) Close() error {
	if it.closed {
		return utils.ErrIteratorClosed
	}

	it.closed = true
	it.keys = nil
	it.data = nil
	it.err = nil

	return nil
}
