package page

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Manager handles page allocation, deallocation, and management.
type Manager struct {
	// nextPageID is the next available page ID.
	nextPageID atomic.Uint32

	// freeList contains IDs of free pages available for reuse.
	freeList []PageID

	// freeListMu protects access to the free list.
	freeListMu sync.Mutex

	// pageMap tracks allocated pages (for testing/debugging).
	pageMap map[PageID]*Page

	// pageMapMu protects access to the page map.
	pageMapMu sync.RWMutex

	// metaPage holds database metadata.
	metaPage *Page
}

// NewManager creates a new page manager.
func NewManager() *Manager {
	m := &Manager{
		freeList: make([]PageID, 0),
		pageMap:  make(map[PageID]*Page),
	}

	// Initialize with meta page at ID 0
	m.nextPageID.Store(1)
	m.metaPage = NewPage(0, PageTypeMeta)
	m.pageMap[0] = m.metaPage

	return m
}

// AllocatePage allocates a new page with the given type.
func (m *Manager) AllocatePage(pageType PageType) (*Page, error) {
	// First try to reuse a free page
	m.freeListMu.Lock()
	if len(m.freeList) > 0 {
		// Pop from free list
		pageID := m.freeList[len(m.freeList)-1]
		m.freeList = m.freeList[:len(m.freeList)-1]
		m.freeListMu.Unlock()

		// Create page with reused ID
		page := NewPage(pageID, pageType)

		// Track the page
		m.pageMapMu.Lock()
		m.pageMap[pageID] = page
		m.pageMapMu.Unlock()

		return page, nil
	}
	m.freeListMu.Unlock()

	// Allocate new page ID
	pageID := PageID(m.nextPageID.Add(1) - 1)
	if pageID == InvalidPageID {
		// Skip invalid page ID (0)
		pageID = PageID(m.nextPageID.Add(1) - 1)
	}

	// Create new page
	page := NewPage(pageID, pageType)

	// Track the page
	m.pageMapMu.Lock()
	m.pageMap[pageID] = page
	m.pageMapMu.Unlock()

	return page, nil
}

// DeallocatePage marks a page as free for reuse.
func (m *Manager) DeallocatePage(pageID PageID) error {
	if pageID == InvalidPageID {
		return ErrInvalidPageID
	}

	if pageID == 0 {
		return fmt.Errorf("cannot deallocate meta page")
	}

	// Remove from page map
	m.pageMapMu.Lock()
	page, exists := m.pageMap[pageID]
	if !exists {
		m.pageMapMu.Unlock()
		return fmt.Errorf("page %d not found", pageID)
	}
	delete(m.pageMap, pageID)
	m.pageMapMu.Unlock()

	// Mark as free
	page.header.PageType = PageTypeFree

	// Add to free list
	m.freeListMu.Lock()
	m.freeList = append(m.freeList, pageID)
	m.freeListMu.Unlock()

	return nil
}

// GetPage retrieves a page by ID.
func (m *Manager) GetPage(pageID PageID) (*Page, error) {
	m.pageMapMu.RLock()
	page, exists := m.pageMap[pageID]
	m.pageMapMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("page %d not found", pageID)
	}

	return page, nil
}

// GetMetaPage returns the database metadata page.
func (m *Manager) GetMetaPage() *Page {
	return m.metaPage
}

// GetFreePageCount returns the number of free pages.
func (m *Manager) GetFreePageCount() int {
	m.freeListMu.Lock()
	count := len(m.freeList)
	m.freeListMu.Unlock()
	return count
}

// GetAllocatedPageCount returns the number of allocated pages.
func (m *Manager) GetAllocatedPageCount() int {
	m.pageMapMu.RLock()
	count := len(m.pageMap)
	m.pageMapMu.RUnlock()
	return count
}

// GetNextPageID returns the next page ID that would be allocated.
func (m *Manager) GetNextPageID() PageID {
	return PageID(m.nextPageID.Load())
}

// Statistics holds page manager statistics.
type Statistics struct {
	AllocatedPages int
	FreePages      int
	NextPageID     PageID
	PageTypeCounts map[PageType]int
}

// GetStatistics returns current page manager statistics.
func (m *Manager) GetStatistics() Statistics {
	stats := Statistics{
		PageTypeCounts: make(map[PageType]int),
	}

	// Get free page count
	m.freeListMu.Lock()
	stats.FreePages = len(m.freeList)
	m.freeListMu.Unlock()

	// Get allocated pages and type counts
	m.pageMapMu.RLock()
	stats.AllocatedPages = len(m.pageMap)
	for _, page := range m.pageMap {
		stats.PageTypeCounts[page.Type()]++
	}
	m.pageMapMu.RUnlock()

	stats.NextPageID = PageID(m.nextPageID.Load())

	return stats
}
