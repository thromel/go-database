// Package btree provides B+ Tree implementation for efficient key-value storage
// with support for range queries and ordered iteration. The B+ Tree maintains
// all data in leaf nodes and uses internal nodes for navigation only.
package btree

import (
	"errors"
	"sync"

	"github.com/thromel/go-database/pkg/storage/page"
)

// BPlusTree represents a B+ Tree index structure optimized for range queries.
// All data is stored in leaf nodes, and internal nodes contain only keys for navigation.
type BPlusTree struct {
	// Tree structure
	root   page.PageID // Root page ID
	height int         // Height of the tree (leaf level = 0)

	// Tree metadata
	numKeys         int64 // Total number of keys in the tree
	branchingFactor int   // Maximum number of children per internal node
	leafCapacity    int   // Maximum number of entries per leaf node

	// Page management
	pageManager *page.Manager // Page allocation and management

	// Concurrency control
	treeLatch sync.RWMutex // Protects tree structure modifications

	// Configuration
	maxKeySize   int // Maximum size of a key in bytes
	maxValueSize int // Maximum size of a value in bytes
}

// Config holds configuration options for B+ Tree creation.
type Config struct {
	BranchingFactor int // Number of children per internal node (default: 128)
	LeafCapacity    int // Number of entries per leaf node (default: 64)
	MaxKeySize      int // Maximum key size in bytes (default: 1024)
	MaxValueSize    int // Maximum value size in bytes (default: 4096)
}

// DefaultConfig returns the default B+ Tree configuration.
func DefaultConfig() *Config {
	return &Config{
		BranchingFactor: 64,  // Reasonable for 8KB pages
		LeafCapacity:    32,  // Conservative to ensure it fits
		MaxKeySize:      64,  // Smaller key size for testing
		MaxValueSize:    128, // Smaller value size for testing
	}
}

// NewBPlusTree creates a new B+ Tree with the given configuration.
func NewBPlusTree(pageManager *page.Manager, config *Config) (*BPlusTree, error) {
	if pageManager == nil {
		return nil, errors.New("page manager cannot be nil")
	}

	if config == nil {
		config = DefaultConfig()
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	tree := &BPlusTree{
		root:            0, // Will be set when first page is allocated
		height:          0, // Empty tree has height 0
		numKeys:         0,
		branchingFactor: config.BranchingFactor,
		leafCapacity:    config.LeafCapacity,
		pageManager:     pageManager,
		maxKeySize:      config.MaxKeySize,
		maxValueSize:    config.MaxValueSize,
	}

	// Create initial root leaf page
	if err := tree.initializeRoot(); err != nil {
		return nil, err
	}

	return tree, nil
}

// validateConfig validates the B+ Tree configuration parameters.
func validateConfig(config *Config) error {
	if config.BranchingFactor < 3 {
		return errors.New("branching factor must be at least 3")
	}
	if config.LeafCapacity < 2 {
		return errors.New("leaf capacity must be at least 2")
	}
	if config.MaxKeySize <= 0 {
		return errors.New("max key size must be positive")
	}
	if config.MaxValueSize <= 0 {
		return errors.New("max value size must be positive")
	}

	// Check if entries can fit in a page
	// Each leaf entry needs: key length (4) + key data + value length (4) + value data
	estimatedLeafEntrySize := 4 + config.MaxKeySize + 4 + config.MaxValueSize
	availableSpace := page.PageSize - page.PageHeaderSize - 100 // Reserve 100 bytes for node metadata
	if estimatedLeafEntrySize*config.LeafCapacity > availableSpace {
		return errors.New("leaf capacity too high for page size")
	}

	// Each internal entry needs: key length (4) + key data + child PageID (8)
	estimatedInternalEntrySize := 4 + config.MaxKeySize + 8
	if estimatedInternalEntrySize*config.BranchingFactor > availableSpace {
		return errors.New("branching factor too high for page size")
	}

	return nil
}

// initializeRoot creates the initial root leaf page for an empty tree.
func (bt *BPlusTree) initializeRoot() error {
	rootPage, err := bt.pageManager.AllocatePage(page.PageTypeLeaf)
	if err != nil {
		return err
	}

	// Create an empty leaf node and write it to the root page
	rootNode := newLeafNode()
	if err := bt.writeNodeToPage(rootNode, rootPage); err != nil {
		return err
	}

	bt.root = rootPage.ID()
	bt.height = 0

	return nil
}

// Get retrieves the value associated with the given key.
// Returns ErrKeyNotFound if the key doesn't exist.
func (bt *BPlusTree) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, ErrInvalidKey
	}
	if len(key) > bt.maxKeySize {
		return nil, ErrKeyTooLarge
	}

	bt.treeLatch.RLock()
	defer bt.treeLatch.RUnlock()

	// Find the leaf node containing the key
	leafPageID, err := bt.findLeafPage(key)
	if err != nil {
		return nil, err
	}

	// Search within the leaf node
	leafPage, err := bt.pageManager.GetPage(leafPageID)
	if err != nil {
		return nil, err
	}

	node, err := bt.deserializeNode(leafPage)
	if err != nil {
		return nil, err
	}

	// Binary search for the key in the leaf node
	value, found := node.findValue(key)
	if !found {
		return nil, ErrKeyNotFound
	}

	return value, nil
}

// Put inserts or updates a key-value pair in the B+ Tree.
func (bt *BPlusTree) Put(key []byte, value []byte) error {
	if len(key) == 0 {
		return ErrInvalidKey
	}
	if len(key) > bt.maxKeySize {
		return ErrKeyTooLarge
	}
	if len(value) > bt.maxValueSize {
		return ErrValueTooLarge
	}

	bt.treeLatch.Lock()
	defer bt.treeLatch.Unlock()

	// Check if key already exists
	existed, err := bt.existsUnlocked(key)
	if err != nil {
		return err
	}

	// Insert into the tree (may cause splits)
	newRoot, err := bt.insertRecursive(bt.root, key, value, bt.height)
	if err != nil {
		return err
	}

	// Update root if tree grew in height
	if newRoot != bt.root {
		bt.root = newRoot
		bt.height++
	}

	// Only increment key count if this is a new key
	if !existed {
		bt.numKeys++
	}

	return nil
}

// Delete removes a key-value pair from the B+ Tree.
func (bt *BPlusTree) Delete(key []byte) error {
	if len(key) == 0 {
		return ErrInvalidKey
	}

	bt.treeLatch.Lock()
	defer bt.treeLatch.Unlock()

	// Check if key exists before attempting deletion
	leafPageID, err := bt.findLeafPage(key)
	if err != nil {
		return err
	}

	leafPage, err := bt.pageManager.GetPage(leafPageID)
	if err != nil {
		return err
	}

	node, err := bt.deserializeNode(leafPage)
	if err != nil {
		return err
	}

	if _, found := node.findValue(key); !found {
		return ErrKeyNotFound
	}

	// Perform deletion (may cause merges)
	err = bt.deleteRecursive(bt.root, key, bt.height)
	if err != nil {
		return err
	}

	bt.numKeys--

	// Check if root became empty and tree height should decrease
	if bt.height > 0 {
		rootPage, err := bt.pageManager.GetPage(bt.root)
		if err != nil {
			return err
		}

		rootNode, err := bt.deserializeNode(rootPage)
		if err != nil {
			return err
		}

		// If root has only one child, make that child the new root
		if len(rootNode.keys) == 0 && len(rootNode.children) == 1 {
			if err := bt.pageManager.DeallocatePage(bt.root); err != nil {
				// Log the error but don't fail the delete operation
				// In a production system, this would be logged to a proper logger
				_ = err
			}
			bt.root = rootNode.children[0]
			bt.height--
		}
	}

	return nil
}

// Exists checks if a key exists in the B+ Tree.
func (bt *BPlusTree) Exists(key []byte) (bool, error) {
	_, err := bt.Get(key)
	if err == ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// existsUnlocked checks if a key exists in the B+ Tree without locking.
// This is used internally when the lock is already held.
func (bt *BPlusTree) existsUnlocked(key []byte) (bool, error) {
	// Find the leaf node containing the key
	leafPageID, err := bt.findLeafPage(key)
	if err != nil {
		return false, err
	}

	// Search within the leaf node
	leafPage, err := bt.pageManager.GetPage(leafPageID)
	if err != nil {
		return false, err
	}

	node, err := bt.deserializeNode(leafPage)
	if err != nil {
		return false, err
	}

	// Binary search for the key in the leaf node
	_, found := node.findValue(key)
	return found, nil
}

// findLeafPage traverses the tree to find the leaf page that should contain the given key.
func (bt *BPlusTree) findLeafPage(key []byte) (page.PageID, error) {
	currentPageID := bt.root
	currentHeight := bt.height

	// Traverse down to leaf level
	for currentHeight > 0 {
		currentPage, err := bt.pageManager.GetPage(currentPageID)
		if err != nil {
			return 0, err
		}

		node, err := bt.deserializeNode(currentPage)
		if err != nil {
			return 0, err
		}

		// Find the appropriate child
		childIndex := node.findChildIndex(key)
		if childIndex >= len(node.children) {
			return 0, errors.New("invalid child index in internal node")
		}

		currentPageID = node.children[childIndex]
		currentHeight--
	}

	return currentPageID, nil
}

// Stats returns statistics about the B+ Tree.
func (bt *BPlusTree) Stats() TreeStats {
	bt.treeLatch.RLock()
	defer bt.treeLatch.RUnlock()

	return TreeStats{
		Height:          bt.height,
		NumKeys:         bt.numKeys,
		BranchingFactor: bt.branchingFactor,
		LeafCapacity:    bt.leafCapacity,
		RootPageID:      bt.root,
	}
}

// TreeStats contains statistics about the B+ Tree.
type TreeStats struct {
	Height          int         // Height of the tree
	NumKeys         int64       // Total number of keys
	BranchingFactor int         // Branching factor
	LeafCapacity    int         // Leaf node capacity
	RootPageID      page.PageID // Root page ID
}

// Common errors for B+ Tree operations.
var (
	ErrKeyNotFound   = errors.New("key not found")
	ErrInvalidKey    = errors.New("invalid key")
	ErrKeyTooLarge   = errors.New("key too large")
	ErrValueTooLarge = errors.New("value too large")
	ErrTreeCorrupted = errors.New("tree structure corrupted")
)
