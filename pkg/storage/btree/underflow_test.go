package btree

import (
	"fmt"
	"testing"

	"github.com/thromel/go-database/pkg/storage/page"
)

func TestBPlusTreeDeleteWithUnderflow(t *testing.T) {
	pageManager := page.NewManager()

	// Use a smaller leaf capacity to trigger underflow more easily
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

	// Insert multiple keys
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	for _, key := range keys {
		err = tree.Put([]byte(key), []byte("value-"+key))
		if err != nil {
			t.Fatalf("Failed to put key %s: %v", key, err)
		}
	}

	// Verify all keys exist
	for _, key := range keys {
		exists, err := tree.Exists([]byte(key))
		if err != nil {
			t.Fatalf("Failed to check existence of key %s: %v", key, err)
		}
		if !exists {
			t.Errorf("Key %s should exist", key)
		}
	}

	// Delete keys one by one and check tree remains valid
	for _, key := range keys {
		err = tree.Delete([]byte(key))
		if err != nil {
			t.Fatalf("Failed to delete key %s: %v", key, err)
		}

		// Verify key no longer exists
		exists, err := tree.Exists([]byte(key))
		if err != nil {
			t.Fatalf("Failed to check existence after deletion of key %s: %v", key, err)
		}
		if exists {
			t.Errorf("Key %s should not exist after deletion", key)
		}

		// Note: For simplicity, we're not checking remaining keys in this test
		// since tracking which keys should still exist during deletion would complicate the test
		// The important thing is that the tree doesn't crash during underflow handling
	}

	// After deleting all keys, tree should be empty
	stats := tree.Stats()
	if stats.NumKeys != 0 {
		t.Errorf("Expected 0 keys after deleting all, got %d", stats.NumKeys)
	}
}

func TestBPlusTreeEmptyAfterDeletes(t *testing.T) {
	pageManager := page.NewManager()
	config := DefaultConfig()

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	// Insert a single key
	key := []byte("single-key")
	value := []byte("single-value")

	err = tree.Put(key, value)
	if err != nil {
		t.Fatalf("Failed to put key: %v", err)
	}

	// Verify it exists
	exists, err := tree.Exists(key)
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}
	if !exists {
		t.Error("Key should exist")
	}

	// Delete the key
	err = tree.Delete(key)
	if err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	// Verify it no longer exists
	exists, err = tree.Exists(key)
	if err != nil {
		t.Fatalf("Failed to check existence after deletion: %v", err)
	}
	if exists {
		t.Error("Key should not exist after deletion")
	}

	// Tree should be empty
	stats := tree.Stats()
	if stats.NumKeys != 0 {
		t.Errorf("Expected 0 keys after deletion, got %d", stats.NumKeys)
	}
}

func TestBPlusTreeMultipleDeletesAndInserts(t *testing.T) {
	pageManager := page.NewManager()
	config := DefaultConfig()

	tree, err := NewBPlusTree(pageManager, config)
	if err != nil {
		t.Fatalf("Failed to create B+ tree: %v", err)
	}

	// Perform multiple rounds of insertions and deletions
	for round := 0; round < 3; round++ {
		t.Logf("Round %d: Inserting keys", round+1)

		// Insert keys for this round
		for i := 0; i < 10; i++ {
			key := []byte(fmt.Sprintf("round%d-key%d", round, i))
			value := []byte(fmt.Sprintf("round%d-value%d", round, i))

			err = tree.Put(key, value)
			if err != nil {
				t.Fatalf("Round %d: Failed to put key%d: %v", round, i, err)
			}
		}

		t.Logf("Round %d: Verifying keys exist", round+1)

		// Verify all keys exist
		for i := 0; i < 10; i++ {
			key := []byte(fmt.Sprintf("round%d-key%d", round, i))

			exists, err := tree.Exists(key)
			if err != nil {
				t.Fatalf("Round %d: Failed to check existence of key%d: %v", round, i, err)
			}
			if !exists {
				t.Errorf("Round %d: Key%d should exist", round, i)
			}
		}

		t.Logf("Round %d: Deleting half the keys", round+1)

		// Delete half the keys
		for i := 0; i < 5; i++ {
			key := []byte(fmt.Sprintf("round%d-key%d", round, i))

			err = tree.Delete(key)
			if err != nil {
				t.Fatalf("Round %d: Failed to delete key%d: %v", round, i, err)
			}
		}

		t.Logf("Round %d: Verifying deleted keys don't exist", round+1)

		// Verify deleted keys don't exist
		for i := 0; i < 5; i++ {
			key := []byte(fmt.Sprintf("round%d-key%d", round, i))

			exists, err := tree.Exists(key)
			if err != nil {
				t.Fatalf("Round %d: Failed to check existence after deletion of key%d: %v", round, i, err)
			}
			if exists {
				t.Errorf("Round %d: Key%d should not exist after deletion", round, i)
			}
		}

		// Check tree stats
		stats := tree.Stats()
		expectedKeys := int64((round + 1) * 5) // 5 remaining keys per round
		if stats.NumKeys != expectedKeys {
			t.Errorf("Round %d: Expected %d keys, got %d", round, expectedKeys, stats.NumKeys)
		}
	}
}
