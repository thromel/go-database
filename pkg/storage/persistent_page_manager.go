package storage

import (
	"fmt"
	"sync"

	"github.com/thromel/go-database/pkg/storage/buffer"
	"github.com/thromel/go-database/pkg/storage/file"
	"github.com/thromel/go-database/pkg/storage/page"
)

// PersistentPageManager integrates the in-memory page manager with file storage
// and buffer pool to provide persistent page management.
type PersistentPageManager struct {
	// Core components
	memoryPageManager *page.Manager
	fileManager       *file.FileManager
	bufferPool        *buffer.BufferPool

	// State
	mu sync.RWMutex

	// Page mapping: in-memory page ID -> file page ID
	pageMapping map[page.PageID]page.PageID
	nextFilePageID page.PageID
}

// NewPersistentPageManager creates a new persistent page manager.
func NewPersistentPageManager(fileManager *file.FileManager, bufferPool *buffer.BufferPool) *PersistentPageManager {
	return &PersistentPageManager{
		memoryPageManager: page.NewManager(),
		fileManager:       fileManager,
		bufferPool:        bufferPool,
		pageMapping:       make(map[page.PageID]page.PageID),
		nextFilePageID:    1, // Start from 1 (0 is invalid)
	}
}

// AllocatePage allocates a new page and ensures it's backed by persistent storage.
func (ppm *PersistentPageManager) AllocatePage(pageType page.PageType) (*page.Page, error) {
	ppm.mu.Lock()
	defer ppm.mu.Unlock()

	// Allocate page in memory
	memPage, err := ppm.memoryPageManager.AllocatePage(pageType)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate memory page: %w", err)
	}

	// Assign file page ID
	filePageID := ppm.nextFilePageID
	ppm.nextFilePageID++

	// Create persistent page with file page ID
	persistentPage := page.NewPage(filePageID, pageType)

	// Map memory page ID to file page ID
	ppm.pageMapping[memPage.ID()] = filePageID

	// Write initial page to file
	if err := ppm.fileManager.WritePage(persistentPage); err != nil {
		// Clean up on failure
		delete(ppm.pageMapping, memPage.ID())
		ppm.memoryPageManager.DeallocatePage(memPage.ID())
		return nil, fmt.Errorf("failed to write page to file: %w", err)
	}

	return persistentPage, nil
}

// DeallocatePage deallocates a page from both memory and persistent storage.
func (ppm *PersistentPageManager) DeallocatePage(pageID page.PageID) error {
	ppm.mu.Lock()
	defer ppm.mu.Unlock()

	// Find corresponding memory page ID
	var memoryPageID page.PageID
	for memID, fileID := range ppm.pageMapping {
		if fileID == pageID {
			memoryPageID = memID
			break
		}
	}

	if memoryPageID == page.InvalidPageID {
		return fmt.Errorf("page %d not found in mapping", pageID)
	}

	// Remove from mapping
	delete(ppm.pageMapping, memoryPageID)

	// Deallocate from memory page manager
	if err := ppm.memoryPageManager.DeallocatePage(memoryPageID); err != nil {
		return fmt.Errorf("failed to deallocate memory page: %w", err)
	}

	// Note: We don't actually delete the page from the file for simplicity.
	// In a production implementation, we would mark it as free in a free page list.

	return nil
}

// GetPage retrieves a page from the buffer pool or loads it from file storage.
func (ppm *PersistentPageManager) GetPage(pageID page.PageID) (*page.Page, error) {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()

	// Try to get from buffer pool first
	if pg, err := ppm.bufferPool.GetPage(pageID); err == nil {
		return pg, nil
	}

	// Load from file storage
	pg, err := ppm.fileManager.ReadPage(pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to read page from file: %w", err)
	}

	return pg, nil
}

// GetMetaPage returns the database metadata page.
func (ppm *PersistentPageManager) GetMetaPage() *page.Page {
	// For simplicity, delegate to memory page manager
	// In a full implementation, this would be stored in the file
	return ppm.memoryPageManager.GetMetaPage()
}

// GetFreePageCount returns the number of free pages.
func (ppm *PersistentPageManager) GetFreePageCount() int {
	return ppm.memoryPageManager.GetFreePageCount()
}

// GetAllocatedPageCount returns the number of allocated pages.
func (ppm *PersistentPageManager) GetAllocatedPageCount() int {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()
	return len(ppm.pageMapping)
}

// GetNextPageID returns the next page ID that would be allocated.
func (ppm *PersistentPageManager) GetNextPageID() page.PageID {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()
	return ppm.nextFilePageID
}

// GetStatistics returns current page manager statistics.
func (ppm *PersistentPageManager) GetStatistics() page.Statistics {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()

	stats := page.Statistics{
		AllocatedPages: len(ppm.pageMapping),
		FreePages:      ppm.memoryPageManager.GetFreePageCount(),
		NextPageID:     ppm.nextFilePageID,
		PageTypeCounts: make(map[page.PageType]int),
	}

	// Count page types by reading from file (simplified approach)
	// In a production implementation, this would be cached
	for _, filePageID := range ppm.pageMapping {
		if pg, err := ppm.fileManager.ReadPage(filePageID); err == nil {
			stats.PageTypeCounts[pg.Type()]++
		}
	}

	return stats
}

// Sync ensures all dirty pages are written to persistent storage.
func (ppm *PersistentPageManager) Sync() error {
	// Flush buffer pool dirty pages
	if err := ppm.bufferPool.FlushAllPages(); err != nil {
		return fmt.Errorf("failed to flush buffer pool: %w", err)
	}

	// Sync file manager
	if err := ppm.fileManager.Sync(); err != nil {
		return fmt.Errorf("failed to sync file manager: %w", err)
	}

	return nil
}