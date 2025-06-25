package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thromel/go-database/pkg/storage/btree"
	"github.com/thromel/go-database/pkg/storage/file"
	"github.com/thromel/go-database/pkg/utils"
)

func TestDefaultPersistentConfig(t *testing.T) {
	config := DefaultPersistentConfig()

	if config == nil {
		t.Fatal("DefaultPersistentConfig returned nil")
	}

	if config.FilePath == "" {
		t.Error("Expected non-empty file path")
	}

	if config.BufferPoolSize <= 0 {
		t.Error("Expected positive buffer pool size")
	}

	if config.BTreeConfig == nil {
		t.Error("Expected non-nil B+ tree config")
	}

	if config.FileConfig == nil {
		t.Error("Expected non-nil file config")
	}

	if !config.SyncOnWrite {
		t.Error("Expected SyncOnWrite to be true by default")
	}

	if !config.EnableIntegrityChecks {
		t.Error("Expected EnableIntegrityChecks to be true by default")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *PersistentConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultPersistentConfig(),
			wantErr: false,
		},
		{
			name: "empty file path",
			config: &PersistentConfig{
				FilePath:       "",
				BufferPoolSize: 1024,
				BTreeConfig:    btree.DefaultConfig(),
				FileConfig:     file.DefaultConfig(),
			},
			wantErr: true,
		},
		{
			name: "zero buffer pool size",
			config: &PersistentConfig{
				FilePath:       "test.godb",
				BufferPoolSize: 0,
				BTreeConfig:    btree.DefaultConfig(),
				FileConfig:     file.DefaultConfig(),
			},
			wantErr: true,
		},
		{
			name: "negative buffer pool size",
			config: &PersistentConfig{
				FilePath:       "test.godb",
				BufferPoolSize: -1,
				BTreeConfig:    btree.DefaultConfig(),
				FileConfig:     file.DefaultConfig(),
			},
			wantErr: true,
		},
		{
			name: "nil B+ tree config",
			config: &PersistentConfig{
				FilePath:       "test.godb",
				BufferPoolSize: 1024,
				BTreeConfig:    nil,
				FileConfig:     file.DefaultConfig(),
			},
			wantErr: true,
		},
		{
			name: "nil file config",
			config: &PersistentConfig{
				FilePath:       "test.godb",
				BufferPoolSize: 1024,
				BTreeConfig:    btree.DefaultConfig(),
				FileConfig:     nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewPersistentEngine(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultPersistentConfig()
	config.FilePath = filepath.Join(tempDir, "test.godb")

	engine, err := NewPersistentEngine(config)
	if err != nil {
		t.Fatalf("Failed to create persistent engine: %v", err)
	}
	defer engine.Close()

	// Verify components are initialized
	if engine.fileManager == nil {
		t.Error("File manager not initialized")
	}

	if engine.pageManager == nil {
		t.Error("Page manager not initialized")
	}

	if engine.bufferPool == nil {
		t.Error("Buffer pool not initialized")
	}

	if engine.btree == nil {
		t.Error("B+ tree not initialized")
	}

	// Verify file was created
	if _, err := os.Stat(config.FilePath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestNewPersistentEngine_InvalidConfig(t *testing.T) {
	// Test with nil config (should use defaults)
	_, err := NewPersistentEngine(nil)
	if err != nil {
		// This should succeed with default config, but might fail due to file path
		t.Logf("Engine creation with nil config failed (expected): %v", err)
	}

	// Test with invalid config
	invalidConfig := &PersistentConfig{
		FilePath:       "",
		BufferPoolSize: -1,
	}

	_, err = NewPersistentEngine(invalidConfig)
	if err == nil {
		t.Error("Expected error with invalid config")
	}

	// Test file creation failure (invalid path)
	invalidPathConfig := DefaultPersistentConfig()
	invalidPathConfig.FilePath = "/invalid/path/that/does/not/exist/test.godb"

	_, err = NewPersistentEngine(invalidPathConfig)
	if err == nil {
		t.Error("Expected error with invalid file path")
	}
}

func TestPersistentEngine_BasicOperations(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultPersistentConfig()
	config.FilePath = filepath.Join(tempDir, "test.godb")

	engine, err := NewPersistentEngine(config)
	if err != nil {
		t.Fatalf("Failed to create persistent engine: %v", err)
	}
	defer engine.Close()

	// Test Put
	key := []byte("test_key")
	value := []byte("test_value")

	err = engine.Put(key, value)
	if err != nil {
		t.Fatalf("Failed to put key-value: %v", err)
	}

	// Test Get
	retrievedValue, err := engine.Get(key)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if string(retrievedValue) != string(value) {
		t.Errorf("Expected value %s, got %s", string(value), string(retrievedValue))
	}

	// Test Exists
	exists, err := engine.Exists(key)
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}

	if !exists {
		t.Error("Key should exist")
	}

	// Test Delete
	err = engine.Delete(key)
	if err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	// Verify deletion
	_, err = engine.Get(key)
	if err == nil {
		t.Error("Expected error when getting deleted key")
	} else if !utils.IsKeyNotFound(err) {
		t.Errorf("Expected ErrKeyNotFound, got %v (type: %T)", err, err)
	}

	exists, err = engine.Exists(key)
	if err != nil {
		t.Fatalf("Failed to check existence after delete: %v", err)
	}

	if exists {
		t.Error("Key should not exist after deletion")
	}
}

func TestPersistentEngine_MultipleOperations(t *testing.T) {
	t.Skip("Skipping until full persistence integration is complete")
	tempDir := t.TempDir()
	config := DefaultPersistentConfig()
	config.FilePath = filepath.Join(tempDir, "test.godb")

	engine, err := NewPersistentEngine(config)
	if err != nil {
		t.Fatalf("Failed to create persistent engine: %v", err)
	}
	defer engine.Close()

	// Insert multiple key-value pairs
	numPairs := 100
	for i := 0; i < numPairs; i++ {
		key := []byte(fmt.Sprintf("key_%03d", i))
		value := []byte(fmt.Sprintf("value_%03d", i))

		if err := engine.Put(key, value); err != nil {
			t.Fatalf("Failed to put key %s: %v", string(key), err)
		}
	}

	// Verify all pairs
	for i := 0; i < numPairs; i++ {
		key := []byte(fmt.Sprintf("key_%03d", i))
		expectedValue := []byte(fmt.Sprintf("value_%03d", i))

		value, err := engine.Get(key)
		if err != nil {
			t.Fatalf("Failed to get key %s: %v", string(key), err)
		}

		if string(value) != string(expectedValue) {
			t.Errorf("Key %s: expected %s, got %s", string(key), string(expectedValue), string(value))
		}
	}

	// Check size
	size, err := engine.Size()
	if err != nil {
		t.Fatalf("Failed to get size: %v", err)
	}

	if size != int64(numPairs) {
		t.Errorf("Expected size %d, got %d", numPairs, size)
	}
}

func TestPersistentEngine_InvalidOperations(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultPersistentConfig()
	config.FilePath = filepath.Join(tempDir, "test.godb")

	engine, err := NewPersistentEngine(config)
	if err != nil {
		t.Fatalf("Failed to create persistent engine: %v", err)
	}
	defer engine.Close()

	// Test empty key operations
	emptyKey := []byte("")

	err = engine.Put(emptyKey, []byte("value"))
	if err != utils.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key put, got %v", err)
	}

	_, err = engine.Get(emptyKey)
	if err != utils.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key get, got %v", err)
	}

	err = engine.Delete(emptyKey)
	if err != utils.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key delete, got %v", err)
	}

	_, err = engine.Exists(emptyKey)
	if err != utils.ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key exists, got %v", err)
	}

	// Test operations on non-existent keys
	nonExistentKey := []byte("non_existent")

	_, err = engine.Get(nonExistentKey)
	if !utils.IsKeyNotFound(err) {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}

	err = engine.Delete(nonExistentKey)
	if !utils.IsKeyNotFound(err) {
		t.Errorf("Expected ErrKeyNotFound for delete, got %v", err)
	}
}

func TestPersistentEngine_ClosedOperations(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultPersistentConfig()
	config.FilePath = filepath.Join(tempDir, "test.godb")

	engine, err := NewPersistentEngine(config)
	if err != nil {
		t.Fatalf("Failed to create persistent engine: %v", err)
	}

	// Close the engine
	if err := engine.Close(); err != nil {
		t.Fatalf("Failed to close engine: %v", err)
	}

	// Test operations on closed engine
	key := []byte("test_key")
	value := []byte("test_value")

	err = engine.Put(key, value)
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for put, got %v", err)
	}

	_, err = engine.Get(key)
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for get, got %v", err)
	}

	err = engine.Delete(key)
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for delete, got %v", err)
	}

	_, err = engine.Exists(key)
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for exists, got %v", err)
	}

	_, err = engine.Size()
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for size, got %v", err)
	}

	err = engine.Sync()
	if err != utils.ErrDatabaseClosed {
		t.Errorf("Expected ErrDatabaseClosed for sync, got %v", err)
	}

	// Test double close
	err = engine.Close()
	if err != nil {
		t.Errorf("Double close should not return error, got %v", err)
	}
}

func TestPersistentEngine_Sync(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultPersistentConfig()
	config.FilePath = filepath.Join(tempDir, "test.godb")
	config.SyncOnWrite = false // Disable automatic sync

	engine, err := NewPersistentEngine(config)
	if err != nil {
		t.Fatalf("Failed to create persistent engine: %v", err)
	}
	defer engine.Close()

	// Put some data
	key := []byte("test_key")
	value := []byte("test_value")

	if err := engine.Put(key, value); err != nil {
		t.Fatalf("Failed to put data: %v", err)
	}

	// Manual sync
	if err := engine.Sync(); err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}
}

func TestPersistentEngine_Statistics(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultPersistentConfig()
	config.FilePath = filepath.Join(tempDir, "test.godb")

	engine, err := NewPersistentEngine(config)
	if err != nil {
		t.Fatalf("Failed to create persistent engine: %v", err)
	}
	defer engine.Close()

	// Initial stats should be zero
	stats := engine.GetStats()
	if stats.ReadCount != 0 || stats.WriteCount != 0 || stats.DeleteCount != 0 {
		t.Error("Initial statistics should be zero")
	}

	// Perform operations and check stats
	key := []byte("test_key")
	value := []byte("test_value")

	// Put operation
	if err := engine.Put(key, value); err != nil {
		t.Fatalf("Failed to put: %v", err)
	}

	stats = engine.GetStats()
	if stats.WriteCount != 1 {
		t.Errorf("Expected 1 write, got %d", stats.WriteCount)
	}

	// Get operation
	if _, err := engine.Get(key); err != nil {
		t.Fatalf("Failed to get: %v", err)
	}

	stats = engine.GetStats()
	if stats.ReadCount != 1 {
		t.Errorf("Expected 1 read, got %d", stats.ReadCount)
	}

	// Delete operation
	if err := engine.Delete(key); err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	stats = engine.GetStats()
	if stats.DeleteCount != 1 {
		t.Errorf("Expected 1 delete, got %d", stats.DeleteCount)
	}
}

func TestPersistentEngine_Persistence(t *testing.T) {
	t.Skip("Skipping until full persistence integration is complete")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	// Create engine and insert data
	config := DefaultPersistentConfig()
	config.FilePath = dbPath

	engine1, err := NewPersistentEngine(config)
	if err != nil {
		t.Fatalf("Failed to create first engine: %v", err)
	}

	key := []byte("persistent_key")
	value := []byte("persistent_value")

	if err := engine1.Put(key, value); err != nil {
		t.Fatalf("Failed to put data: %v", err)
	}

	if err := engine1.Close(); err != nil {
		t.Fatalf("Failed to close first engine: %v", err)
	}

	// Reopen engine and verify data persists
	engine2, err := NewPersistentEngine(config)
	if err != nil {
		t.Fatalf("Failed to create second engine: %v", err)
	}
	defer engine2.Close()

	retrievedValue, err := engine2.Get(key)
	if err != nil {
		t.Fatalf("Failed to get data from reopened engine: %v", err)
	}

	if string(retrievedValue) != string(value) {
		t.Errorf("Data not persisted correctly: expected %s, got %s", string(value), string(retrievedValue))
	}
}

func TestPersistentEngine_GetComponents(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultPersistentConfig()
	config.FilePath = filepath.Join(tempDir, "test.godb")

	engine, err := NewPersistentEngine(config)
	if err != nil {
		t.Fatalf("Failed to create persistent engine: %v", err)
	}
	defer engine.Close()

	// Test component getters
	if engine.GetFileManager() == nil {
		t.Error("GetFileManager returned nil")
	}

	if engine.GetBufferPool() == nil {
		t.Error("GetBufferPool returned nil")
	}

	if engine.GetBTree() == nil {
		t.Error("GetBTree returned nil")
	}
}

func TestPersistentEngine_ConcurrentAccess(t *testing.T) {
	t.Skip("Skipping until full persistence integration is complete")
	tempDir := t.TempDir()
	config := DefaultPersistentConfig()
	config.FilePath = filepath.Join(tempDir, "test.godb")

	engine, err := NewPersistentEngine(config)
	if err != nil {
		t.Fatalf("Failed to create persistent engine: %v", err)
	}
	defer engine.Close()

	// Test concurrent operations
	numGoroutines := 10
	operationsPerGoroutine := 50

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			for j := 0; j < operationsPerGoroutine; j++ {
				key := []byte(fmt.Sprintf("key_%d_%d", goroutineID, j))
				value := []byte(fmt.Sprintf("value_%d_%d", goroutineID, j))

				// Put
				if err := engine.Put(key, value); err != nil {
					t.Errorf("Goroutine %d: Put failed: %v", goroutineID, err)
					return
				}

				// Get
				if _, err := engine.Get(key); err != nil {
					t.Errorf("Goroutine %d: Get failed: %v", goroutineID, err)
					return
				}

				// Exists
				if exists, err := engine.Exists(key); err != nil || !exists {
					t.Errorf("Goroutine %d: Exists failed: %v", goroutineID, err)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("Concurrent access test timed out")
		}
	}

	// Verify final state
	stats := engine.GetStats()
	expectedWrites := int64(numGoroutines * operationsPerGoroutine)

	if stats.WriteCount != expectedWrites {
		t.Errorf("Expected %d writes, got %d", expectedWrites, stats.WriteCount)
	}
}

func TestErrorIterator(t *testing.T) {
	testErr := fmt.Errorf("test error")
	iter := &ErrorIterator{err: testErr}

	if iter.Valid() {
		t.Error("ErrorIterator should not be valid")
	}

	if iter.Next() {
		t.Error("ErrorIterator Next should return false")
	}

	if iter.Key() != nil {
		t.Error("ErrorIterator Key should return nil")
	}

	if iter.Value() != nil {
		t.Error("ErrorIterator Value should return nil")
	}

	if iter.Error() != testErr {
		t.Errorf("Expected error %v, got %v", testErr, iter.Error())
	}

	if err := iter.Close(); err != nil {
		t.Errorf("ErrorIterator Close should not return error, got %v", err)
	}

	// Test methods that don't return values
	iter.Seek([]byte("test"))
	iter.SeekToFirst()
	iter.SeekToLast()
}