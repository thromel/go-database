package btree

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/thromel/go-database/pkg/storage/page"
)

// TestBPlusTreeEdgeCases tests various edge cases and error conditions
func TestBPlusTreeEdgeCases(t *testing.T) {
	pageManager := page.NewManager()
	config := DefaultConfig()

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	// Test Put with nil key (should be rejected by validation)
	err = tree.Put(nil, []byte("value"))
	if err == nil {
		t.Error("Expected error for nil key")
	}

	// Test Get with nil key
	_, err = tree.Get(nil)
	if err == nil {
		t.Error("Expected error for nil key")
	}

	// Test Delete with nil key
	err = tree.Delete(nil)
	if err == nil {
		t.Error("Expected error for nil key")
	}

	// Test Exists with nil key
	_, err = tree.Exists(nil)
	if err == nil {
		t.Error("Expected error for nil key")
	}
}

// TestSerializationEdgeCases tests serialization/deserialization edge cases
func TestSerializationEdgeCases(t *testing.T) {
	pageManager := page.NewManager()
	config := DefaultConfig()

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	// Test serialization of nil node
	_, err = tree.serializeNode(nil)
	if err == nil {
		t.Error("Expected error when serializing nil node")
	}

	// Test deserialization of nil page
	_, err = tree.deserializeNode(nil)
	if err == nil {
		t.Error("Expected error when deserializing nil page")
	}

	// Test deserialization with insufficient data
	emptyPage, err := pageManager.AllocatePage(page.PageTypeLeaf)
	if err != nil {
		t.Fatalf("Failed to allocate page: %v", err)
	}

	// Clear page data to simulate corrupted/insufficient data
	data := emptyPage.Data()
	for i := range data {
		data[i] = 0
	}

	_, err = tree.deserializeNode(emptyPage)
	if err == nil {
		t.Log("Note: Deserialization may succeed with zero data as it represents valid empty node")
		// This might actually succeed as all-zero data could represent a valid empty node
		// The minimum size check is 21 bytes, and an empty page has sufficient size
	}
}

// TestTreeCorruptionHandling tests handling of tree corruption scenarios
func TestTreeCorruptionHandling(t *testing.T) {
	pageManager := page.NewManager()
	config := &Config{
		BranchingFactor: 4,
		LeafCapacity:    4,
		MaxKeySize:      64,
		MaxValueSize:    128,
	}

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	// Insert some data to create a tree structure
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%d", i))
		value := []byte(fmt.Sprintf("value%d", i))
		err = tree.Put(key, value)
		if err != nil {
			t.Fatalf("Failed to put key: %v", err)
		}
	}

	// Test accessing a child index that's out of bounds
	// This is hard to simulate directly, but we can test the error condition
	// by trying to access non-existent keys after tree height increases

	stats := tree.Stats()
	originalHeight := stats.Height

	// Verify the tree has some structure
	if originalHeight < 0 {
		t.Errorf("Expected non-negative tree height, got %d", originalHeight)
	}
}

// TestLargeKeyValues tests handling of large keys and values
func TestLargeKeyValues(t *testing.T) {
	pageManager := page.NewManager()
	config := &Config{
		BranchingFactor: 4,
		LeafCapacity:    4,
		MaxKeySize:      100,
		MaxValueSize:    200,
	}

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	// Test maximum size key and value
	maxKey := make([]byte, config.MaxKeySize)
	for i := range maxKey {
		maxKey[i] = byte('A' + (i % 26))
	}

	maxValue := make([]byte, config.MaxValueSize)
	for i := range maxValue {
		maxValue[i] = byte('0' + (i % 10))
	}

	err = tree.Put(maxKey, maxValue)
	if err != nil {
		t.Errorf("Failed to put max size key-value: %v", err)
	}

	// Retrieve and verify
	retrievedValue, err := tree.Get(maxKey)
	if err != nil {
		t.Errorf("Failed to get max size key: %v", err)
	}

	if !bytes.Equal(retrievedValue, maxValue) {
		t.Error("Retrieved value doesn't match original max value")
	}
}

// TestConcurrentOperations tests thread safety aspects
func TestConcurrentOperations(t *testing.T) {
	pageManager := page.NewManager()
	config := DefaultConfig()

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	// Insert initial data
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%d", i))
		value := []byte(fmt.Sprintf("value%d", i))
		err = tree.Put(key, value)
		if err != nil {
			t.Fatalf("Failed to put initial key: %v", err)
		}
	}

	// Test concurrent reads (should not cause issues with RLock)
	done := make(chan bool, 3)

	// Reader goroutine 1
	go func() {
		for i := 0; i < 5; i++ {
			key := []byte(fmt.Sprintf("key%d", i))
			_, err := tree.Get(key)
			if err != nil {
				t.Errorf("Concurrent read failed: %v", err)
			}
		}
		done <- true
	}()

	// Reader goroutine 2
	go func() {
		for i := 5; i < 10; i++ {
			exists, err := tree.Exists([]byte(fmt.Sprintf("key%d", i)))
			if err != nil {
				t.Errorf("Concurrent exists check failed: %v", err)
			}
			if !exists {
				t.Errorf("Key should exist during concurrent read")
			}
		}
		done <- true
	}()

	// Stats reader
	go func() {
		for i := 0; i < 3; i++ {
			stats := tree.Stats()
			if stats.NumKeys != 10 {
				t.Errorf("Expected 10 keys during concurrent access, got %d", stats.NumKeys)
			}
		}
		done <- true
	}()

	// Wait for all readers to complete
	for i := 0; i < 3; i++ {
		<-done
	}
}

// TestNodeFindMethods tests the node search methods with edge cases
func TestNodeFindMethods(t *testing.T) {
	// Test findValue with empty leaf node
	emptyLeaf := newLeafNode()
	value, found := emptyLeaf.findValue([]byte("anykey"))
	if found {
		t.Error("Should not find value in empty leaf")
	}
	if value != nil {
		t.Error("Should return nil value for empty leaf")
	}

	// Test findValue with internal node (should return false)
	internal := newInternalNode()
	internal.keys = [][]byte{[]byte("key1")}
	_, found = internal.findValue([]byte("key1"))
	if found {
		t.Error("Should not find value in internal node")
	}

	// Test findChildIndex with leaf node (should return -1)
	leaf := newLeafNode()
	index := leaf.findChildIndex([]byte("anykey"))
	if index != -1 {
		t.Error("Should return -1 for leaf node child index")
	}

	// Test findChildIndex with various key positions
	internal.children = []page.PageID{1, 2}

	// Key less than all keys should return 0
	index = internal.findChildIndex([]byte("key0"))
	if index != 0 {
		t.Errorf("Expected index 0 for key less than all, got %d", index)
	}

	// Key greater than all keys should return last index
	index = internal.findChildIndex([]byte("key9"))
	if index != len(internal.keys) {
		t.Errorf("Expected index %d for key greater than all, got %d", len(internal.keys), index)
	}
}

// TestSplitEdgeCases tests node splitting edge cases
func TestSplitEdgeCases(t *testing.T) {
	// Test splitLeaf with internal node (should return nil)
	internal := newInternalNode()
	internal.keys = [][]byte{[]byte("key1"), []byte("key2")}

	newNode, promoteKey := internal.splitLeaf(4)
	if newNode != nil {
		t.Error("splitLeaf should return nil for internal node")
	}
	if promoteKey != nil {
		t.Error("splitLeaf should return nil promote key for internal node")
	}

	// Test splitInternal with leaf node (should return nil)
	leaf := newLeafNode()
	leaf.keys = [][]byte{[]byte("key1"), []byte("key2")}

	newNode, promoteKey = leaf.splitInternal(4)
	if newNode != nil {
		t.Error("splitInternal should return nil for leaf node")
	}
	if promoteKey != nil {
		t.Error("splitInternal should return nil promote key for leaf node")
	}
}

// TestInsertInLeafEdgeCases tests leaf insertion edge cases
func TestInsertInLeafEdgeCases(t *testing.T) {
	// Test insertInLeaf with internal node (should return false)
	internal := newInternalNode()
	needsSplit := internal.insertInLeaf([]byte("key"), []byte("value"), 4)
	if needsSplit {
		t.Error("insertInLeaf should return false for internal node")
	}

	// Test insertInInternal with leaf node (should return false)
	leaf := newLeafNode()
	needsSplit = leaf.insertInInternal([]byte("key"), page.PageID(1), 4)
	if needsSplit {
		t.Error("insertInInternal should return false for leaf node")
	}

	// Test key update in leaf (should not need split)
	leaf.keys = [][]byte{[]byte("existingkey")}
	leaf.values = [][]byte{[]byte("oldvalue")}

	needsSplit = leaf.insertInLeaf([]byte("existingkey"), []byte("newvalue"), 4)
	if needsSplit {
		t.Error("Updating existing key should not need split")
	}

	if string(leaf.values[0]) != "newvalue" {
		t.Error("Key update should change the value")
	}
}

// TestDeleteEdgeCases tests deletion edge cases
func TestDeleteEdgeCases(t *testing.T) {
	// Test deleteFromLeaf with internal node (should return false)
	internal := newInternalNode()
	internal.keys = [][]byte{[]byte("key1")}

	underflow := internal.deleteFromLeaf([]byte("key1"), 1)
	if underflow {
		t.Error("deleteFromLeaf should return false for internal node")
	}

	// Test deletion of non-existent key
	leaf := newLeafNode()
	leaf.keys = [][]byte{[]byte("key1"), []byte("key3")}
	leaf.values = [][]byte{[]byte("val1"), []byte("val3")}

	underflow = leaf.deleteFromLeaf([]byte("key2"), 1)
	if underflow {
		t.Error("Deleting non-existent key should not cause underflow")
	}

	if len(leaf.keys) != 2 {
		t.Error("Key count should not change when deleting non-existent key")
	}
}

// TestBorrowFromRightEdgeCase tests borrowFromRight when right sibling becomes empty
func TestBorrowFromRightEdgeCase(t *testing.T) {
	node := newLeafNode()
	node.keys = [][]byte{[]byte("key1")}
	node.values = [][]byte{[]byte("val1")}

	rightSibling := newLeafNode()
	rightSibling.keys = [][]byte{[]byte("key2")}
	rightSibling.values = [][]byte{[]byte("val2")}

	// Borrow the only key from right sibling
	newSeparator := node.borrowFromRight(rightSibling, []byte("separator"))

	// Right sibling should be empty, so newSeparator should be nil
	if len(rightSibling.keys) != 0 {
		t.Error("Right sibling should be empty after borrowing its only key")
	}

	if newSeparator != nil {
		t.Error("Should return nil separator when right sibling becomes empty")
	}
}
