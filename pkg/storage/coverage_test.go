package storage

import (
	"testing"

	"github.com/thromel/go-database/pkg/utils"
)

// TestMemoryEngine_SyncAndStats tests Sync and Stats methods
func TestMemoryEngine_SyncAndStats(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Test Sync on open engine
	err := engine.Sync()
	if err != nil {
		t.Errorf("Sync should not fail on open engine, got: %v", err)
	}

	// Test Stats on open engine
	stats := engine.Stats()
	// Stats should return empty struct for memory engine
	if stats.ReadCount != 0 || stats.WriteCount != 0 {
		t.Logf("Stats: %+v", stats) // Log but don't fail, as it's implementation-specific
	}

	// Close engine
	engine.Close()

	// Test Sync on closed engine
	err = engine.Sync()
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for Sync on closed engine, got: %v", err)
	}
}

// TestMemoryIterator_SeekToLast tests the SeekToLast method
func TestMemoryIterator_SeekToLast(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Test with empty engine
	iter := engine.NewIterator(nil, nil)
	defer iter.Close()

	iter.SeekToLast()
	if iter.Valid() {
		t.Error("SeekToLast should be invalid on empty engine")
	}

	// Add test data
	keys := []string{"a", "b", "c", "d", "e"}
	for _, k := range keys {
		err := engine.Put([]byte(k), []byte("value-"+k))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// Test SeekToLast with data
	iter2 := engine.NewIterator(nil, nil)
	defer iter2.Close()

	iter2.SeekToLast()
	if !iter2.Valid() {
		t.Error("SeekToLast should be valid with data")
	}
	if string(iter2.Key()) != "e" {
		t.Errorf("Expected last key 'e', got '%s'", iter2.Key())
	}
	if string(iter2.Value()) != "value-e" {
		t.Errorf("Expected last value 'value-e', got '%s'", iter2.Value())
	}
}

// TestMemoryIterator_ClosedIterator tests operations on closed iterators
func TestMemoryIterator_ClosedIterator(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Add test data
	_ = engine.Put([]byte("key"), []byte("value"))

	iter := engine.NewIterator(nil, nil)

	// Close the iterator
	err := iter.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Test operations on closed iterator
	if iter.Valid() {
		t.Error("Closed iterator should not be valid")
	}

	if iter.Next() {
		t.Error("Next should return false on closed iterator")
	}

	if iter.Error() != utils.ErrIteratorClosed {
		t.Errorf("Expected ErrIteratorClosed, got: %v", iter.Error())
	}

	iter.Seek([]byte("key"))
	if iter.Error() != utils.ErrIteratorClosed {
		t.Errorf("Expected ErrIteratorClosed after Seek, got: %v", iter.Error())
	}

	iter.SeekToFirst()
	if iter.Error() != utils.ErrIteratorClosed {
		t.Errorf("Expected ErrIteratorClosed after SeekToFirst, got: %v", iter.Error())
	}

	iter.SeekToLast()
	if iter.Error() != utils.ErrIteratorClosed {
		t.Errorf("Expected ErrIteratorClosed after SeekToLast, got: %v", iter.Error())
	}

	// Test double close
	err = iter.Close()
	if err != utils.ErrIteratorClosed {
		t.Errorf("Expected ErrIteratorClosed on double close, got: %v", err)
	}
}

// TestMemoryIterator_EdgeCases tests iterator edge cases
func TestMemoryIterator_EdgeCases(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Add test data
	_ = engine.Put([]byte("key1"), []byte("value1"))
	_ = engine.Put([]byte("key2"), []byte("value2"))

	iter := engine.NewIterator(nil, nil)
	defer iter.Close()

	// Test Key() and Value() on invalid iterator
	if iter.Key() != nil {
		t.Error("Key() should return nil for invalid iterator")
	}
	if iter.Value() != nil {
		t.Error("Value() should return nil for invalid iterator")
	}

	// Position at valid key
	iter.SeekToFirst()
	if !iter.Valid() {
		t.Fatal("Iterator should be valid after SeekToFirst")
	}

	// Test Next() advancing past end
	iter.Next() // Should be at key2
	if !iter.Valid() {
		t.Error("Iterator should still be valid at key2")
	}

	iter.Next() // Should be past end
	if iter.Valid() {
		t.Error("Iterator should be invalid past end")
	}

	// Test Key() and Value() when invalid
	if iter.Key() != nil {
		t.Error("Key() should return nil when iterator is invalid")
	}
	if iter.Value() != nil {
		t.Error("Value() should return nil when iterator is invalid")
	}
}

// TestMemoryEngine_OperationsOnClosed tests operations on closed engine
func TestMemoryEngine_OperationsOnClosed(t *testing.T) {
	engine := NewMemoryEngine()

	// Close the engine
	engine.Close()

	// Test all operations on closed engine
	_, err := engine.Get([]byte("key"))
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for Get, got: %v", err)
	}

	err = engine.Put([]byte("key"), []byte("value"))
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for Put, got: %v", err)
	}

	err = engine.Delete([]byte("key"))
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for Delete, got: %v", err)
	}

	_, err = engine.Exists([]byte("key"))
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for Exists, got: %v", err)
	}

	_, err = engine.Size()
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for Size, got: %v", err)
	}

	iter := engine.NewIterator(nil, nil)
	if iter != nil {
		t.Error("NewIterator should return nil on closed engine")
	}
}

// TestMemoryIterator_ValueNotFound tests edge case where key exists in keys but not in data
func TestMemoryIterator_ValueNotFound(t *testing.T) {
	// This is a synthetic test for an edge case that shouldn't happen in normal operation
	// but we need to test the error path in Value()
	iter := &MemoryIterator{
		keys:     []string{"key1"},
		data:     map[string][]byte{}, // Empty data map
		position: 0,
		closed:   false,
	}

	// This should trigger the "key not found" error path in Value()
	value := iter.Value()
	if value != nil {
		t.Error("Value() should return nil when key not found in data")
	}
	if iter.Error() != utils.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got: %v", iter.Error())
	}
}

// TestMemoryEngine_EmptyValueHandling tests handling of empty (not nil) values
func TestMemoryEngine_EmptyValueHandling(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	key := []byte("key")
	emptyValue := []byte{} // Empty but not nil

	// Empty values should be allowed
	err := engine.Put(key, emptyValue)
	if err != nil {
		t.Fatalf("Put with empty value should succeed, got: %v", err)
	}

	// Should be able to retrieve empty value
	value, err := engine.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(value) != 0 {
		t.Errorf("Expected empty value, got length %d", len(value))
	}
}
