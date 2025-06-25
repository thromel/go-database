package storage

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/romel/go-database/pkg/utils"
)

func TestMemoryEngine_BasicOperations(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Test Put
	key := []byte("test-key")
	value := []byte("test-value")
	
	err := engine.Put(key, value)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Test Get
	result, err := engine.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	
	if !bytes.Equal(result, value) {
		t.Errorf("Expected %s, got %s", value, result)
	}

	// Test Exists
	exists, err := engine.Exists(key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	
	if !exists {
		t.Error("Key should exist")
	}

	// Test Delete
	err = engine.Delete(key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Test Get after delete
	_, err = engine.Get(key)
	if err != utils.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}

	// Test Exists after delete
	exists, err = engine.Exists(key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	
	if exists {
		t.Error("Key should not exist after delete")
	}
}

func TestMemoryEngine_KeyValidation(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Test nil key
	err := engine.Put(nil, []byte("value"))
	if err != utils.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for nil key, got %v", err)
	}

	// Test empty key
	err = engine.Put([]byte{}, []byte("value"))
	if err != utils.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key, got %v", err)
	}

	// Test key too large
	largeKey := make([]byte, 65537) // 64KB + 1
	err = engine.Put(largeKey, []byte("value"))
	if err != utils.ErrKeyTooLarge {
		t.Errorf("Expected ErrKeyTooLarge, got %v", err)
	}
}

func TestMemoryEngine_ValueValidation(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Test nil value
	err := engine.Put([]byte("key"), nil)
	if err != utils.ErrInvalidValue {
		t.Errorf("Expected ErrInvalidValue for nil value, got %v", err)
	}

	// Test value too large
	largeValue := make([]byte, 16*1024*1024+1) // 16MB + 1
	err = engine.Put([]byte("key"), largeValue)
	if err != utils.ErrValueTooLarge {
		t.Errorf("Expected ErrValueTooLarge, got %v", err)
	}
}

func TestMemoryEngine_Iterator(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Add test data
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
	}

	for k, v := range testData {
		err := engine.Put([]byte(k), []byte(v))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// Test full iteration
	iter := engine.NewIterator(nil, nil)
	defer iter.Close()

	count := 0
	for iter.SeekToFirst(); iter.Valid(); iter.Next() {
		key := string(iter.Key())
		value := string(iter.Value())
		
		expectedValue, exists := testData[key]
		if !exists {
			t.Errorf("Unexpected key: %s", key)
		}
		
		if value != expectedValue {
			t.Errorf("Expected value %s for key %s, got %s", expectedValue, key, value)
		}
		
		count++
	}

	if count != len(testData) {
		t.Errorf("Expected %d items, got %d", len(testData), count)
	}

	if iter.Error() != nil {
		t.Errorf("Iterator error: %v", iter.Error())
	}
}

func TestMemoryEngine_IteratorRange(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Add test data
	keys := []string{"a", "b", "c", "d", "e"}
	for _, k := range keys {
		err := engine.Put([]byte(k), []byte("value-"+k))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// Test range iteration [b, d)
	iter := engine.NewIterator([]byte("b"), []byte("d"))
	defer iter.Close()

	expectedKeys := []string{"b", "c"}
	actualKeys := []string{}

	for iter.SeekToFirst(); iter.Valid(); iter.Next() {
		actualKeys = append(actualKeys, string(iter.Key()))
	}

	if len(actualKeys) != len(expectedKeys) {
		t.Errorf("Expected %d keys, got %d", len(expectedKeys), len(actualKeys))
	}

	for i, key := range actualKeys {
		if key != expectedKeys[i] {
			t.Errorf("Expected key %s at position %d, got %s", expectedKeys[i], i, key)
		}
	}
}

func TestMemoryEngine_IteratorSeek(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Add test data
	keys := []string{"a", "c", "e", "g", "i"}
	for _, k := range keys {
		err := engine.Put([]byte(k), []byte("value-"+k))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	iter := engine.NewIterator(nil, nil)
	defer iter.Close()

	// Test seek to existing key
	iter.Seek([]byte("c"))
	if !iter.Valid() {
		t.Error("Iterator should be valid after seek to existing key")
	}
	if string(iter.Key()) != "c" {
		t.Errorf("Expected key 'c', got %s", iter.Key())
	}

	// Test seek to non-existing key (should find next larger key)
	iter.Seek([]byte("d"))
	if !iter.Valid() {
		t.Error("Iterator should be valid after seek to non-existing key")
	}
	if string(iter.Key()) != "e" {
		t.Errorf("Expected key 'e', got %s", iter.Key())
	}

	// Test seek past all keys
	iter.Seek([]byte("z"))
	if iter.Valid() {
		t.Error("Iterator should be invalid after seek past all keys")
	}
}

func TestMemoryEngine_ThreadSafety(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Test concurrent reads and writes
	const numGoroutines = 10
	const numOperations = 100

	// Start concurrent goroutines
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			for j := 0; j < numOperations; j++ {
				key := []byte(fmt.Sprintf("key-%d-%d", id, j))
				value := []byte(fmt.Sprintf("value-%d-%d", id, j))
				
				// Put
				err := engine.Put(key, value)
				if err != nil {
					t.Errorf("Put failed: %v", err)
					return
				}
				
				// Get
				result, err := engine.Get(key)
				if err != nil {
					t.Errorf("Get failed: %v", err)
					return
				}
				
				if !bytes.Equal(result, value) {
					t.Errorf("Expected %s, got %s", value, result)
					return
				}
				
				// Delete
				err = engine.Delete(key)
				if err != nil {
					t.Errorf("Delete failed: %v", err)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestMemoryEngine_Size(t *testing.T) {
	engine := NewMemoryEngine()
	defer engine.Close()

	// Initial size should be 0
	size, err := engine.Size()
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != 0 {
		t.Errorf("Expected size 0, got %d", size)
	}

	// Add some data
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key-%d", i))
		value := []byte(fmt.Sprintf("value-%d", i))
		err := engine.Put(key, value)
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// Size should be 10
	size, err = engine.Size()
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != 10 {
		t.Errorf("Expected size 10, got %d", size)
	}
}

func TestMemoryEngine_Close(t *testing.T) {
	engine := NewMemoryEngine()

	// Add some data
	err := engine.Put([]byte("key"), []byte("value"))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Close the engine
	err = engine.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations should fail after close
	_, err = engine.Get([]byte("key"))
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed, got %v", err)
	}

	err = engine.Put([]byte("key2"), []byte("value2"))
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed, got %v", err)
	}
}

func BenchmarkMemoryEngine_Put(b *testing.B) {
	engine := NewMemoryEngine()
	defer engine.Close()

	key := []byte("benchmark-key")
	value := []byte("benchmark-value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := engine.Put(key, value)
		if err != nil {
			b.Fatalf("Put failed: %v", err)
		}
	}
}

func BenchmarkMemoryEngine_Get(b *testing.B) {
	engine := NewMemoryEngine()
	defer engine.Close()

	key := []byte("benchmark-key")
	value := []byte("benchmark-value")
	
	// Setup
	err := engine.Put(key, value)
	if err != nil {
		b.Fatalf("Put failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.Get(key)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}