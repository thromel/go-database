package api

import (
	"bytes"
	"context"
	"testing"

	"github.com/romel/go-database/pkg/utils"
)

const testDBPath = "test.db"

func TestDatabase_Open(t *testing.T) {
	config := DefaultConfig()
	config.Path = testDBPath

	db, err := Open(testDBPath, config)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Verify database is not closed
	impl := db.(*DatabaseImpl)
	if impl.IsClosed() {
		t.Error("Database should not be closed after open")
	}

	// Verify config is set
	if impl.GetConfig().Path != testDBPath {
		t.Errorf("Expected path %s, got %s", testDBPath, impl.GetConfig().Path)
	}
}

func TestDatabase_OpenWithNilConfig(t *testing.T) {
	db, err := Open(testDBPath, nil)
	if err != nil {
		t.Fatalf("Open with nil config failed: %v", err)
	}
	defer db.Close()

	// Should use default config
	impl := db.(*DatabaseImpl)
	config := impl.GetConfig()
	if config == nil {
		t.Error("Config should not be nil")
		return
	}
	if config.Path != testDBPath {
		t.Errorf("Expected path %s, got %s", testDBPath, config.Path)
	}
}

func TestDatabase_OpenWithInvalidConfig(t *testing.T) {
	config := &Config{
		Path: "", // Invalid: empty path
	}

	_, err := Open("", config)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

func TestDatabase_BasicOperations(t *testing.T) {
	db, err := Open("test.db", DefaultConfig())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Test Put
	key := []byte("test-key")
	value := []byte("test-value")

	err = db.Put(key, value)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Test Get
	result, err := db.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !bytes.Equal(result, value) {
		t.Errorf("Expected %s, got %s", value, result)
	}

	// Test Exists
	exists, err := db.Exists(key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}

	if !exists {
		t.Error("Key should exist")
	}

	// Test Delete
	err = db.Delete(key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Test Get after delete
	_, err = db.Get(key)
	if !utils.IsKeyNotFound(err) {
		t.Errorf("Expected key not found error, got %v", err)
	}

	// Test Exists after delete
	exists, err = db.Exists(key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}

	if exists {
		t.Error("Key should not exist after delete")
	}
}

func TestDatabase_Stats(t *testing.T) {
	db, err := Open("test.db", DefaultConfig())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Initial stats
	stats, err := db.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.KeyCount != 0 {
		t.Errorf("Expected 0 keys, got %d", stats.KeyCount)
	}

	// Add some data
	for i := 0; i < 5; i++ {
		key := []byte("key" + string(rune('0'+i)))
		value := []byte("value" + string(rune('0'+i)))
		err := db.Put(key, value)
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// Check stats after adding data
	stats, err = db.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.KeyCount != 5 {
		t.Errorf("Expected 5 keys, got %d", stats.KeyCount)
	}
}

func TestDatabase_Close(t *testing.T) {
	db, err := Open("test.db", DefaultConfig())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Add some data
	err = db.Put([]byte("key"), []byte("value"))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Close the database
	err = db.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify database is closed
	impl := db.(*DatabaseImpl)
	if !impl.IsClosed() {
		t.Error("Database should be closed")
	}

	// Operations should fail after close
	_, err = db.Get([]byte("key"))
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed, got %v", err)
	}

	err = db.Put([]byte("key2"), []byte("value2"))
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed, got %v", err)
	}

	err = db.Delete([]byte("key"))
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed, got %v", err)
	}

	_, err = db.Exists([]byte("key"))
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed, got %v", err)
	}

	_, err = db.Stats()
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed, got %v", err)
	}

	// Double close should return error
	err = db.Close()
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed on double close, got %v", err)
	}
}

func TestDatabase_TransactionsNotImplemented(t *testing.T) {
	db, err := Open("test.db", DefaultConfig())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Transactions should not be implemented yet
	_, err = db.Begin()
	if err == nil {
		t.Error("Expected error for Begin() - transactions not implemented yet")
	}

	_, err = db.BeginWithContext(context.TODO())
	if err == nil {
		t.Error("Expected error for BeginWithContext() - transactions not implemented yet")
	}
}

func TestDatabase_InvalidOperations(t *testing.T) {
	db, err := Open("test.db", DefaultConfig())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Test invalid key operations
	_, err = db.Get(nil)
	if err == nil {
		t.Error("Expected error for nil key")
	}

	err = db.Put(nil, []byte("value"))
	if err == nil {
		t.Error("Expected error for nil key")
	}

	err = db.Put([]byte("key"), nil)
	if err == nil {
		t.Error("Expected error for nil value")
	}

	err = db.Delete(nil)
	if err == nil {
		t.Error("Expected error for nil key")
	}

	_, err = db.Exists(nil)
	if err == nil {
		t.Error("Expected error for nil key")
	}
}

func TestDatabase_ConcurrentOperations(t *testing.T) {
	db, err := Open("test.db", DefaultConfig())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	const numGoroutines = 10
	const numOperations = 100

	// Start concurrent goroutines
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				key := []byte("key-" + string(rune('0'+id)) + "-" + string(rune('0'+j)))
				value := []byte("value-" + string(rune('0'+id)) + "-" + string(rune('0'+j)))

				// Put
				err := db.Put(key, value)
				if err != nil {
					t.Errorf("Put failed: %v", err)
					return
				}

				// Get
				result, err := db.Get(key)
				if err != nil {
					t.Errorf("Get failed: %v", err)
					return
				}

				if !bytes.Equal(result, value) {
					t.Errorf("Expected %s, got %s", value, result)
					return
				}

				// Delete
				err = db.Delete(key)
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

func BenchmarkDatabase_Put(b *testing.B) {
	db, err := Open("bench.db", DefaultConfig())
	if err != nil {
		b.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	key := []byte("benchmark-key")
	value := []byte("benchmark-value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := db.Put(key, value)
		if err != nil {
			b.Fatalf("Put failed: %v", err)
		}
	}
}

func BenchmarkDatabase_Get(b *testing.B) {
	db, err := Open("bench.db", DefaultConfig())
	if err != nil {
		b.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	key := []byte("benchmark-key")
	value := []byte("benchmark-value")

	// Setup
	err = db.Put(key, value)
	if err != nil {
		b.Fatalf("Put failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.Get(key)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}
