package btree

import (
	"errors"

	"github.com/thromel/go-database/pkg/storage/page"
)

// insertRecursive recursively inserts a key-value pair into the tree.
// Returns the new root page ID if the tree height increased.
func (bt *BPlusTree) insertRecursive(pageID page.PageID, key []byte, value []byte, height int) (page.PageID, error) {
	currentPage, err := bt.pageManager.GetPage(pageID)
	if err != nil {
		return 0, err
	}

	node, err := bt.deserializeNode(currentPage)
	if err != nil {
		return 0, err
	}

	if height == 0 {
		// Leaf level - insert key-value pair
		needsSplit := node.insertInLeaf(key, value, bt.leafCapacity)

		// Serialize the updated node back to the page
		if err := bt.writeNodeToPage(node, currentPage); err != nil {
			return 0, err
		}

		if needsSplit {
			return bt.splitLeafPage(pageID, node)
		}
		return pageID, nil
	} else {
		// Internal level - find child and recurse
		childIndex := node.findChildIndex(key)
		if childIndex >= len(node.children) {
			return 0, ErrTreeCorrupted
		}

		newChildRoot, err := bt.insertRecursive(node.children[childIndex], key, value, height-1)
		if err != nil {
			return 0, err
		}

		// If child split, we need to insert the new separator key
		if newChildRoot != node.children[childIndex] {
			// Get the separator key from the split
			newChildPage, err := bt.pageManager.GetPage(newChildRoot)
			if err != nil {
				return 0, err
			}

			newChildNode, err := bt.deserializeNode(newChildPage)
			if err != nil {
				return 0, err
			}

			// The separator key is the first key of the new node
			separatorKey := newChildNode.keys[0]

			// Insert separator into this internal node
			needsSplit := node.insertInInternal(separatorKey, newChildRoot, bt.branchingFactor)

			// Update the child pointer
			node.children[childIndex] = newChildRoot

			// Serialize the updated node back to the page
			if err := bt.writeNodeToPage(node, currentPage); err != nil {
				return 0, err
			}

			if needsSplit {
				return bt.splitInternalPage(pageID, node)
			}
		}
		return pageID, nil
	}
}

// deleteRecursive recursively deletes a key from the tree.
func (bt *BPlusTree) deleteRecursive(pageID page.PageID, key []byte, height int) error {
	currentPage, err := bt.pageManager.GetPage(pageID)
	if err != nil {
		return err
	}

	node, err := bt.deserializeNode(currentPage)
	if err != nil {
		return err
	}

	if height == 0 {
		// Leaf level - delete key-value pair
		minCapacity := bt.leafCapacity / 2
		isUnderfull := node.deleteFromLeaf(key, minCapacity)

		// Serialize the updated node back to the page
		if err := bt.writeNodeToPage(node, currentPage); err != nil {
			return err
		}

		if isUnderfull {
			return bt.handleLeafUnderflow(pageID, node)
		}
		return nil
	} else {
		// Internal level - find child and recurse
		childIndex := node.findChildIndex(key)
		if childIndex >= len(node.children) {
			return ErrTreeCorrupted
		}

		err := bt.deleteRecursive(node.children[childIndex], key, height-1)
		if err != nil {
			return err
		}

		// Check if child became underfull and needs rebalancing
		return bt.handleInternalChildUnderflow(pageID, node, childIndex)
	}
}

// splitLeafPage splits a leaf page and returns the new root.
func (bt *BPlusTree) splitLeafPage(pageID page.PageID, node *BPlusTreeNode) (page.PageID, error) {
	// Split the node
	newNode, promoteKey := node.splitLeaf(bt.leafCapacity)

	// Allocate a new page for the split node
	newPage, err := bt.pageManager.AllocatePage(page.PageTypeLeaf)
	if err != nil {
		return 0, err
	}

	// Update next pointers for leaf linking
	node.next = newPage.ID()

	// Write both nodes to their pages
	originalPage, err := bt.pageManager.GetPage(pageID)
	if err != nil {
		return 0, err
	}

	if err := bt.writeNodeToPage(node, originalPage); err != nil {
		return 0, err
	}

	if err := bt.writeNodeToPage(newNode, newPage); err != nil {
		return 0, err
	}

	// Create new root if this was the root page
	if pageID == bt.root {
		return bt.createNewRoot(pageID, newPage.ID(), promoteKey)
	}

	return newPage.ID(), nil
}

// splitInternalPage splits an internal page and returns the new root.
func (bt *BPlusTree) splitInternalPage(pageID page.PageID, node *BPlusTreeNode) (page.PageID, error) {
	// Split the node
	newNode, promoteKey := node.splitInternal(bt.branchingFactor)

	// Allocate a new page for the split node
	newPage, err := bt.pageManager.AllocatePage(page.PageTypeInternal)
	if err != nil {
		return 0, err
	}

	// Write both nodes to their pages
	originalPage, err := bt.pageManager.GetPage(pageID)
	if err != nil {
		return 0, err
	}

	if err := bt.writeNodeToPage(node, originalPage); err != nil {
		return 0, err
	}

	if err := bt.writeNodeToPage(newNode, newPage); err != nil {
		return 0, err
	}

	// Create new root if this was the root page
	if pageID == bt.root {
		return bt.createNewRoot(pageID, newPage.ID(), promoteKey)
	}

	return newPage.ID(), nil
}

// createNewRoot creates a new root node with two children.
func (bt *BPlusTree) createNewRoot(leftChildID, rightChildID page.PageID, separatorKey []byte) (page.PageID, error) {
	// Allocate a new page for the root
	newRootPage, err := bt.pageManager.AllocatePage(page.PageTypeInternal)
	if err != nil {
		return 0, err
	}

	// Create new root node
	newRoot := newInternalNode()
	newRoot.keys = append(newRoot.keys, separatorKey)
	newRoot.children = append(newRoot.children, leftChildID, rightChildID)

	// Write the new root to its page
	if err := bt.writeNodeToPage(newRoot, newRootPage); err != nil {
		return 0, err
	}

	return newRootPage.ID(), nil
}

// handleLeafUnderflow handles underflow in a leaf node.
func (bt *BPlusTree) handleLeafUnderflow(pageID page.PageID, node *BPlusTreeNode) error {
	_ = pageID // Suppress unused parameter warning
	minCapacity := bt.leafCapacity / 2
	if len(node.keys) >= minCapacity {
		return nil // No underflow
	}

	// If this is the root and it's empty, tree becomes empty
	if pageID == bt.root && len(node.keys) == 0 {
		// Tree becomes empty - this is acceptable for a leaf root
		return nil
	}

	// For non-root leaves, we need to find the parent to handle redistribution/merging
	// Since we don't maintain parent pointers in this implementation,
	// we'll implement a simple approach: allow underflow temporarily
	// In a production implementation, we would:
	// 1. Find parent node and locate this leaf's position
	// 2. Try to borrow from left or right sibling
	// 3. If borrowing fails, merge with a sibling
	// 4. Recursively handle any resulting underflow in parent

	// For now, just ensure the node is still valid
	// Note: We don't deallocate leaf pages here as they might still be referenced
	// In a complete implementation, we would handle this through parent node updates

	return nil
}

// handleInternalChildUnderflow handles underflow in a child of an internal node.
func (bt *BPlusTree) handleInternalChildUnderflow(pageID page.PageID, node *BPlusTreeNode, childIndex int) error {
	// Check if the child actually has underflow
	if childIndex >= len(node.children) {
		return nil // Invalid child index
	}

	childPageID := node.children[childIndex]
	childPage, err := bt.pageManager.GetPage(childPageID)
	if err != nil {
		return err
	}

	childNode, err := bt.deserializeNode(childPage)
	if err != nil {
		return err
	}

	var minCapacity int
	if childNode.isLeaf {
		minCapacity = bt.leafCapacity / 2
	} else {
		minCapacity = bt.branchingFactor / 2
	}

	// Check if child actually has underflow
	if len(childNode.keys) >= minCapacity {
		return nil // No underflow
	}

	// For a more complete implementation, we would:
	// 1. Try to borrow from left sibling (if exists and has enough keys)
	// 2. Try to borrow from right sibling (if exists and has enough keys)
	// 3. Merge with left or right sibling if borrowing is not possible
	// 4. Remove the merged child from this internal node
	// 5. Recursively handle any resulting underflow in this node

	// For now, implement basic case: allow underflow temporarily
	// Complete implementation would handle:
	// - Borrowing from siblings when they have extra keys
	// - Merging with siblings when borrowing isn't possible
	// - Updating parent keys after merging
	// - Recursive underflow handling up the tree

	// For this basic implementation, we just ensure the tree structure remains valid
	// without deallocating pages that might still be referenced

	return nil
}

// writeNodeToPage serializes a node and writes it to a page.
func (bt *BPlusTree) writeNodeToPage(node *BPlusTreeNode, pg *page.Page) error {
	data, err := bt.serializeNode(node)
	if err != nil {
		return err
	}

	// Copy the serialized data to the page's data section
	pageData := pg.Data()
	if len(data) > len(pageData) {
		return errors.New("serialized node too large for page")
	}

	copy(pageData, data)

	// Clear any remaining data
	for i := len(data); i < len(pageData); i++ {
		pageData[i] = 0
	}

	return nil
}
