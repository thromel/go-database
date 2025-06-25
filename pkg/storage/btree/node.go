package btree

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/thromel/go-database/pkg/storage/page"
)

// BPlusTreeNode represents a node in the B+ Tree.
// Internal nodes contain keys and child page IDs for navigation.
// Leaf nodes contain keys and values and are linked for range scans.
type BPlusTreeNode struct {
	// Node type
	isLeaf bool

	// Keys are sorted in ascending order
	keys [][]byte

	// For internal nodes: child page IDs (len(children) = len(keys) + 1)
	children []page.PageID

	// For leaf nodes: values corresponding to keys (len(values) = len(keys))
	values [][]byte

	// For leaf nodes: pointer to next leaf for range scans
	next page.PageID

	// Parent tracking for efficient updates
	parent page.PageID
}

// newLeafNode creates a new leaf node.
func newLeafNode() *BPlusTreeNode {
	return &BPlusTreeNode{
		isLeaf:   true,
		keys:     make([][]byte, 0),
		children: nil,
		values:   make([][]byte, 0),
		next:     0,
		parent:   0,
	}
}

// newInternalNode creates a new internal node.
func newInternalNode() *BPlusTreeNode {
	return &BPlusTreeNode{
		isLeaf:   false,
		keys:     make([][]byte, 0),
		children: make([]page.PageID, 0),
		values:   nil,
		next:     0,
		parent:   0,
	}
}

// findValue performs binary search to find a value for the given key in a leaf node.
// Returns the value and whether the key was found.
func (node *BPlusTreeNode) findValue(key []byte) ([]byte, bool) {
	if !node.isLeaf {
		return nil, false
	}

	index := sort.Search(len(node.keys), func(i int) bool {
		return bytes.Compare(node.keys[i], key) >= 0
	})

	if index < len(node.keys) && bytes.Equal(node.keys[index], key) {
		return node.values[index], true
	}

	return nil, false
}

// findChildIndex finds the index of the child that should contain the given key.
// Used for traversing internal nodes.
func (node *BPlusTreeNode) findChildIndex(key []byte) int {
	if node.isLeaf {
		return -1 // Leaf nodes don't have children
	}

	// Find the first key greater than the search key
	index := sort.Search(len(node.keys), func(i int) bool {
		return bytes.Compare(node.keys[i], key) > 0
	})

	return index
}

// insertInLeaf inserts a key-value pair into a leaf node.
// Returns whether the node needs to be split.
func (node *BPlusTreeNode) insertInLeaf(key []byte, value []byte, capacity int) bool {
	if !node.isLeaf {
		return false
	}

	// Find insertion position
	index := sort.Search(len(node.keys), func(i int) bool {
		return bytes.Compare(node.keys[i], key) >= 0
	})

	// Check if key already exists (update case)
	if index < len(node.keys) && bytes.Equal(node.keys[index], key) {
		// Update existing value
		node.values[index] = value
		return false
	}

	// Insert new key-value pair
	node.keys = insertByteSliceAt(node.keys, index, key)
	node.values = insertByteSliceAt(node.values, index, value)

	// Check if split is needed
	return len(node.keys) > capacity
}

// insertInInternal inserts a key and child pointer into an internal node.
// Returns whether the node needs to be split.
func (node *BPlusTreeNode) insertInInternal(key []byte, childPageID page.PageID, branchingFactor int) bool {
	if node.isLeaf {
		return false
	}

	// Find insertion position for the key
	index := sort.Search(len(node.keys), func(i int) bool {
		return bytes.Compare(node.keys[i], key) >= 0
	})

	// Insert key and adjust children
	node.keys = insertByteSliceAt(node.keys, index, key)
	node.children = insertPageIDAt(node.children, index+1, childPageID)

	// Check if split is needed
	return len(node.keys) > branchingFactor-1
}

// splitLeaf splits a leaf node into two nodes.
// Returns the new right node and the key to promote to parent.
func (node *BPlusTreeNode) splitLeaf(capacity int) (*BPlusTreeNode, []byte) {
	if !node.isLeaf {
		return nil, nil
	}

	mid := capacity / 2
	newNode := newLeafNode()

	// Move right half to new node
	newNode.keys = make([][]byte, len(node.keys)-mid)
	newNode.values = make([][]byte, len(node.values)-mid)

	copy(newNode.keys, node.keys[mid:])
	copy(newNode.values, node.values[mid:])

	// Update next pointers for linked list
	newNode.next = node.next
	node.next = 0 // Will be set to new node's page ID by caller

	// Truncate original node
	node.keys = node.keys[:mid]
	node.values = node.values[:mid]

	// The key to promote is the first key of the new node
	promoteKey := make([]byte, len(newNode.keys[0]))
	copy(promoteKey, newNode.keys[0])

	return newNode, promoteKey
}

// splitInternal splits an internal node into two nodes.
// Returns the new right node and the key to promote to parent.
func (node *BPlusTreeNode) splitInternal(branchingFactor int) (*BPlusTreeNode, []byte) {
	if node.isLeaf {
		return nil, nil
	}

	mid := (branchingFactor - 1) / 2
	newNode := newInternalNode()

	// The middle key is promoted to parent
	promoteKey := make([]byte, len(node.keys[mid]))
	copy(promoteKey, node.keys[mid])

	// Move right half to new node (excluding the promoted key)
	newNode.keys = make([][]byte, len(node.keys)-mid-1)
	newNode.children = make([]page.PageID, len(node.children)-mid-1)

	copy(newNode.keys, node.keys[mid+1:])
	copy(newNode.children, node.children[mid+1:])

	// Truncate original node
	node.keys = node.keys[:mid]
	node.children = node.children[:mid+1]

	return newNode, promoteKey
}

// deleteFromLeaf removes a key-value pair from a leaf node.
// Returns whether the node is now underfull.
func (node *BPlusTreeNode) deleteFromLeaf(key []byte, minCapacity int) bool {
	if !node.isLeaf {
		return false
	}

	// Find the key
	index := sort.Search(len(node.keys), func(i int) bool {
		return bytes.Compare(node.keys[i], key) >= 0
	})

	if index >= len(node.keys) || !bytes.Equal(node.keys[index], key) {
		return false // Key not found
	}

	// Remove key and value
	node.keys = removeByteSliceAt(node.keys, index)
	node.values = removeByteSliceAt(node.values, index)

	// Check if underflow occurred
	return len(node.keys) < minCapacity
}

// deleteFromInternal removes a key from an internal node.
// Returns whether the node is now underfull.
// NOTE: Currently unused but kept for future complete underflow handling implementation.
func (node *BPlusTreeNode) deleteFromInternal(key []byte, minCapacity int) bool {
	if node.isLeaf {
		return false
	}

	// Find the key
	index := sort.Search(len(node.keys), func(i int) bool {
		return bytes.Compare(node.keys[i], key) >= 0
	})

	if index >= len(node.keys) || !bytes.Equal(node.keys[index], key) {
		return false // Key not found
	}

	// Remove key and corresponding child
	node.keys = removeByteSliceAt(node.keys, index)
	node.children = removePageIDAt(node.children, index+1)

	// Check if underflow occurred
	return len(node.keys) < minCapacity
}

// canBorrowLeft checks if this node can borrow an entry from its left sibling.
// NOTE: Currently unused but kept for future complete underflow handling implementation.
func (node *BPlusTreeNode) canBorrowLeft(leftSibling *BPlusTreeNode, minCapacity int) bool {
	if leftSibling == nil {
		return false
	}
	return len(leftSibling.keys) > minCapacity
}

// canBorrowRight checks if this node can borrow an entry from its right sibling.
// NOTE: Currently unused but kept for future complete underflow handling implementation.
func (node *BPlusTreeNode) canBorrowRight(rightSibling *BPlusTreeNode, minCapacity int) bool {
	if rightSibling == nil {
		return false
	}
	return len(rightSibling.keys) > minCapacity
}

// borrowFromLeft borrows an entry from the left sibling.
// NOTE: Currently unused but kept for future complete underflow handling implementation.
func (node *BPlusTreeNode) borrowFromLeft(leftSibling *BPlusTreeNode, parentKey []byte) []byte {
	if node.isLeaf {
		// Borrow the last entry from left sibling
		lastKey := leftSibling.keys[len(leftSibling.keys)-1]
		lastValue := leftSibling.values[len(leftSibling.values)-1]

		// Remove from left sibling
		leftSibling.keys = leftSibling.keys[:len(leftSibling.keys)-1]
		leftSibling.values = leftSibling.values[:len(leftSibling.values)-1]

		// Add to beginning of this node
		node.keys = insertByteSliceAt(node.keys, 0, lastKey)
		node.values = insertByteSliceAt(node.values, 0, lastValue)

		// Return new separator key for parent
		newSeparator := make([]byte, len(node.keys[0]))
		copy(newSeparator, node.keys[0])
		return newSeparator
	} else {
		// For internal nodes
		lastKey := leftSibling.keys[len(leftSibling.keys)-1]
		lastChild := leftSibling.children[len(leftSibling.children)-1]

		// Remove from left sibling
		leftSibling.keys = leftSibling.keys[:len(leftSibling.keys)-1]
		leftSibling.children = leftSibling.children[:len(leftSibling.children)-1]

		// Add parent key to beginning of this node
		node.keys = insertByteSliceAt(node.keys, 0, parentKey)
		node.children = insertPageIDAt(node.children, 0, lastChild)

		// Return the borrowed key to replace parent key
		return lastKey
	}
}

// borrowFromRight borrows an entry from the right sibling.
// NOTE: Currently unused but kept for future complete underflow handling implementation.
func (node *BPlusTreeNode) borrowFromRight(rightSibling *BPlusTreeNode, parentKey []byte) []byte {
	if node.isLeaf {
		// Borrow the first entry from right sibling
		firstKey := rightSibling.keys[0]
		firstValue := rightSibling.values[0]

		// Remove from right sibling
		rightSibling.keys = removeByteSliceAt(rightSibling.keys, 0)
		rightSibling.values = removeByteSliceAt(rightSibling.values, 0)

		// Add to end of this node
		node.keys = append(node.keys, firstKey)
		node.values = append(node.values, firstValue)

		// Return new separator key for parent
		if len(rightSibling.keys) > 0 {
			newSeparator := make([]byte, len(rightSibling.keys[0]))
			copy(newSeparator, rightSibling.keys[0])
			return newSeparator
		}
		return nil
	} else {
		// For internal nodes
		firstKey := rightSibling.keys[0]
		firstChild := rightSibling.children[0]

		// Remove from right sibling
		rightSibling.keys = removeByteSliceAt(rightSibling.keys, 0)
		rightSibling.children = removePageIDAt(rightSibling.children, 0)

		// Add parent key to end of this node
		node.keys = append(node.keys, parentKey)
		node.children = append(node.children, firstChild)

		// Return the borrowed key to replace parent key
		return firstKey
	}
}

// merge merges this node with its right sibling.
// NOTE: Currently unused but kept for future complete underflow handling implementation.
func (node *BPlusTreeNode) merge(rightSibling *BPlusTreeNode, parentKey []byte) {
	if node.isLeaf {
		// Merge leaf nodes
		node.keys = append(node.keys, rightSibling.keys...)
		node.values = append(node.values, rightSibling.values...)
		node.next = rightSibling.next
	} else {
		// Merge internal nodes with parent key in between
		node.keys = append(node.keys, parentKey)
		node.keys = append(node.keys, rightSibling.keys...)
		node.children = append(node.children, rightSibling.children...)
	}
}

// Utility functions for slice manipulation

func insertByteSliceAt(slice [][]byte, index int, value []byte) [][]byte {
	result := make([][]byte, len(slice)+1)
	copy(result[:index], slice[:index])
	result[index] = value
	copy(result[index+1:], slice[index:])
	return result
}

func removeByteSliceAt(slice [][]byte, index int) [][]byte {
	result := make([][]byte, len(slice)-1)
	copy(result[:index], slice[:index])
	copy(result[index:], slice[index+1:])
	return result
}

func insertPageIDAt(slice []page.PageID, index int, value page.PageID) []page.PageID {
	result := make([]page.PageID, len(slice)+1)
	copy(result[:index], slice[:index])
	result[index] = value
	copy(result[index+1:], slice[index:])
	return result
}

// removePageIDAt removes a PageID at the specified index.
// NOTE: Currently unused but kept for future complete underflow handling implementation.
func removePageIDAt(slice []page.PageID, index int) []page.PageID {
	result := make([]page.PageID, len(slice)-1)
	copy(result[:index], slice[:index])
	copy(result[index:], slice[index+1:])
	return result
}

// Serialization and deserialization

// serializeNode converts a node to byte representation for storage.
func (bt *BPlusTree) serializeNode(node *BPlusTreeNode) ([]byte, error) {
	if node == nil {
		return nil, errors.New("cannot serialize nil node")
	}

	buffer := make([]byte, 0, page.PageSize-page.PageHeaderSize)

	// Write node type (1 byte)
	if node.isLeaf {
		buffer = append(buffer, 1)
	} else {
		buffer = append(buffer, 0)
	}

	// Write next pointer for leaf nodes (8 bytes)
	nextBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nextBytes, uint64(node.next))
	buffer = append(buffer, nextBytes...)

	// Write parent pointer (8 bytes)
	parentBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(parentBytes, uint64(node.parent))
	buffer = append(buffer, parentBytes...)

	// Write number of keys (4 bytes)
	numKeysBytes := make([]byte, 4)
	if len(node.keys) > int(^uint32(0)) {
		return nil, errors.New("too many keys to serialize")
	}
	binary.LittleEndian.PutUint32(numKeysBytes, uint32(len(node.keys)))
	buffer = append(buffer, numKeysBytes...)

	// Write keys
	for _, key := range node.keys {
		// Write key length (4 bytes)
		keyLenBytes := make([]byte, 4)
		if len(key) > int(^uint32(0)) {
			return nil, errors.New("key too large to serialize")
		}
		binary.LittleEndian.PutUint32(keyLenBytes, uint32(len(key)))
		buffer = append(buffer, keyLenBytes...)

		// Write key data
		buffer = append(buffer, key...)
	}

	if node.isLeaf {
		// Write values for leaf nodes
		for _, value := range node.values {
			// Write value length (4 bytes)
			valueLenBytes := make([]byte, 4)
			if len(value) > int(^uint32(0)) {
				return nil, errors.New("value too large to serialize")
			}
			binary.LittleEndian.PutUint32(valueLenBytes, uint32(len(value)))
			buffer = append(buffer, valueLenBytes...)

			// Write value data
			buffer = append(buffer, value...)
		}
	} else {
		// Write children for internal nodes
		for _, child := range node.children {
			childBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(childBytes, uint64(child))
			buffer = append(buffer, childBytes...)
		}
	}

	return buffer, nil
}

// deserializeNode converts byte representation back to a node.
func (bt *BPlusTree) deserializeNode(pg *page.Page) (*BPlusTreeNode, error) {
	if pg == nil {
		return nil, errors.New("cannot deserialize nil page")
	}

	data := pg.Data()
	if len(data) < 21 { // Minimum size: 1 + 8 + 8 + 4
		return nil, errors.New("insufficient data for node deserialization")
	}

	offset := 0
	node := &BPlusTreeNode{}

	// Read node type (1 byte)
	node.isLeaf = data[offset] == 1
	offset++

	// Read next pointer (8 bytes)
	nextPtr := binary.LittleEndian.Uint64(data[offset:])
	if nextPtr > uint64(^uint32(0)) {
		return nil, errors.New("next pointer value too large for PageID")
	}
	node.next = page.PageID(nextPtr)
	offset += 8

	// Read parent pointer (8 bytes)
	parentPtr := binary.LittleEndian.Uint64(data[offset:])
	if parentPtr > uint64(^uint32(0)) {
		return nil, errors.New("parent pointer value too large for PageID")
	}
	node.parent = page.PageID(parentPtr)
	offset += 8

	// Read number of keys (4 bytes)
	numKeys := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	// Read keys
	node.keys = make([][]byte, numKeys)
	for i := uint32(0); i < numKeys; i++ {
		if offset+4 > len(data) {
			return nil, errors.New("insufficient data for key length")
		}

		keyLen := binary.LittleEndian.Uint32(data[offset:])
		offset += 4

		if offset+int(keyLen) > len(data) {
			return nil, errors.New("insufficient data for key")
		}

		node.keys[i] = make([]byte, keyLen)
		copy(node.keys[i], data[offset:offset+int(keyLen)])
		offset += int(keyLen)
	}

	if node.isLeaf {
		// Read values for leaf nodes
		node.values = make([][]byte, numKeys)
		for i := uint32(0); i < numKeys; i++ {
			if offset+4 > len(data) {
				return nil, errors.New("insufficient data for value length")
			}

			valueLen := binary.LittleEndian.Uint32(data[offset:])
			offset += 4

			if offset+int(valueLen) > len(data) {
				return nil, errors.New("insufficient data for value")
			}

			node.values[i] = make([]byte, valueLen)
			copy(node.values[i], data[offset:offset+int(valueLen)])
			offset += int(valueLen)
		}
	} else {
		// Read children for internal nodes (numKeys + 1 children)
		numChildren := numKeys + 1
		node.children = make([]page.PageID, numChildren)
		for i := uint32(0); i < numChildren; i++ {
			if offset+8 > len(data) {
				return nil, errors.New("insufficient data for child page ID")
			}

			childPageID := binary.LittleEndian.Uint64(data[offset:])
			if childPageID > uint64(^uint32(0)) {
				return nil, errors.New("child page ID value too large for PageID")
			}
			node.children[i] = page.PageID(childPageID)
			offset += 8
		}
	}

	return node, nil
}
