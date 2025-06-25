package btree

import (
	"testing"

	"github.com/thromel/go-database/pkg/storage/page"
)

func TestNewBPlusTree(t *testing.T) {
	pageManager := page.NewManager()
	config := DefaultConfig()

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	stats := tree.Stats()
	if stats.NumKeys != 0 {
		t.Errorf("Expected 0 keys, got %d", stats.NumKeys)
	}

	if stats.Height != 0 {
		t.Errorf("Expected height 0, got %d", stats.Height)
	}
}

func TestBPlusTreeBasicOperations(t *testing.T) {
	pageManager := page.NewManager()
	config := DefaultConfig()

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	// Test Put operation
	key := []byte("test-key")
	value := []byte("test-value")

	err = tree.Put(key, value)
	if err != nil {
		t.Fatalf("Failed to put key-value: %v", err)
	}

	// Test Get operation
	retrievedValue, err := tree.Get(key)
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}

	if string(retrievedValue) != string(value) {
		t.Errorf("Expected value %s, got %s", string(value), string(retrievedValue))
	}

	// Test Exists operation
	exists, err := tree.Exists(key)
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}

	if !exists {
		t.Error("Expected key to exist")
	}

	// Test stats after insertion
	stats := tree.Stats()
	if stats.NumKeys != 1 {
		t.Errorf("Expected 1 key, got %d", stats.NumKeys)
	}
}

func TestBPlusTreeGetNonExistentKey(t *testing.T) {
	pageManager := page.NewManager()
	config := DefaultConfig()

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	// Try to get a non-existent key
	_, err = tree.Get([]byte("non-existent"))
	if err != ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}

	// Test Exists for non-existent key
	exists, err := tree.Exists([]byte("non-existent"))
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}

	if exists {
		t.Error("Expected key to not exist")
	}
}

func TestBPlusTreeDelete(t *testing.T) {
	pageManager := page.NewManager()
	config := DefaultConfig()

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	// Insert a key-value pair
	key := []byte("test-key")
	value := []byte("test-value")

	err = tree.Put(key, value)
	if err != nil {
		t.Fatalf("Failed to put key-value: %v", err)
	}

	// Delete the key
	err = tree.Delete(key)
	if err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	// Verify key no longer exists
	_, err = tree.Get(key)
	if err != ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound after deletion, got %v", err)
	}

	// Test stats after deletion
	stats := tree.Stats()
	if stats.NumKeys != 0 {
		t.Errorf("Expected 0 keys after deletion, got %d", stats.NumKeys)
	}
}

func TestBPlusTreeInvalidOperations(t *testing.T) {
	pageManager := page.NewManager()
	config := DefaultConfig()

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	// Test with empty key
	err = tree.Put([]byte(""), []byte("value"))
	if err != ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey for empty key, got %v", err)
	}

	// Test with key too large
	largeKey := make([]byte, config.MaxKeySize+1)
	err = tree.Put(largeKey, []byte("value"))
	if err != ErrKeyTooLarge {
		t.Errorf("Expected ErrKeyTooLarge, got %v", err)
	}

	// Test with value too large
	largeValue := make([]byte, config.MaxValueSize+1)
	err = tree.Put([]byte("key"), largeValue)
	if err != ErrValueTooLarge {
		t.Errorf("Expected ErrValueTooLarge, got %v", err)
	}
}

func TestBPlusTreeUpdate(t *testing.T) {
	pageManager := page.NewManager()
	config := DefaultConfig()

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	key := []byte("test-key")
	originalValue := []byte("original-value")
	updatedValue := []byte("updated-value")

	// Insert original value
	err = tree.Put(key, originalValue)
	if err != nil {
		t.Fatalf("Failed to put original value: %v", err)
	}

	// Update with new value
	err = tree.Put(key, updatedValue)
	if err != nil {
		t.Fatalf("Failed to update value: %v", err)
	}

	// Retrieve and verify updated value
	retrievedValue, err := tree.Get(key)
	if err != nil {
		t.Fatalf("Failed to get updated value: %v", err)
	}

	if string(retrievedValue) != string(updatedValue) {
		t.Errorf("Expected updated value %s, got %s", string(updatedValue), string(retrievedValue))
	}

	// Stats should still show 1 key
	stats := tree.Stats()
	if stats.NumKeys != 1 {
		t.Errorf("Expected 1 key after update, got %d", stats.NumKeys)
	}
}

func TestBPlusTreeConfig(t *testing.T) {
	// Test default config
	config := DefaultConfig()
	if config.BranchingFactor != 64 {
		t.Errorf("Expected default branching factor 64, got %d", config.BranchingFactor)
	}

	if config.LeafCapacity != 32 {
		t.Errorf("Expected default leaf capacity 32, got %d", config.LeafCapacity)
	}

	// Test config validation
	pageManager := page.NewManager()

	// Test invalid branching factor
	invalidConfig := &Config{
		BranchingFactor: 2, // Too small
		LeafCapacity:    64,
		MaxKeySize:      1024,
		MaxValueSize:    4096,
	}

	_, err := NewBPlusTree(pageManager, invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid branching factor")
	}

	// Test invalid leaf capacity
	invalidConfig = &Config{
		BranchingFactor: 128,
		LeafCapacity:    1, // Too small
		MaxKeySize:      1024,
		MaxValueSize:    4096,
	}

	_, err = NewBPlusTree(pageManager, invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid leaf capacity")
	}
}

func TestBPlusTreeWithNilPageManager(t *testing.T) {
	config := DefaultConfig()

	_, err := NewBPlusTree(nil, config)
	if err == nil {
		t.Error("Expected error for nil page manager")
	}
}

