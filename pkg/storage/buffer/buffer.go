// Package buffer provides buffer pool management for efficient page caching.
// The buffer pool sits between the storage engine and the page manager,
// providing intelligent caching with LRU eviction policy.
package buffer

import (
	"container/list"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/thromel/go-database/pkg/storage/page"
)

// Frame represents a buffer frame containing a page and its metadata.
type Frame struct {
	// PageID is the ID of the page stored in this frame.
	PageID page.PageID

	// Page is the actual page data.
	Page *page.Page

	// PinCount tracks how many operations are using this frame.
	PinCount int32

	// IsDirty indicates whether the page has been modified.
	IsDirty bool

	// LastAccess tracks when this frame was last accessed (for LRU).
	LastAccess time.Time

	// LRUElement points to the element in the LRU list.
	LRUElement *list.Element
}

// Pin increments the pin count for this frame.
func (f *Frame) Pin() {
	atomic.AddInt32(&f.PinCount, 1)
	f.LastAccess = time.Now()
}

// Unpin decrements the pin count for this frame.
func (f *Frame) Unpin() {
	atomic.AddInt32(&f.PinCount, -1)
}

// IsPinned returns true if the frame is currently pinned.
func (f *Frame) IsPinned() bool {
	return atomic.LoadInt32(&f.PinCount) > 0
}

// GetPinCount returns the current pin count.
func (f *Frame) GetPinCount() int32 {
	return atomic.LoadInt32(&f.PinCount)
}

// SetDirty marks the frame as dirty.
func (f *Frame) SetDirty() {
	f.IsDirty = true
}

// BufferPool manages a fixed-size buffer of page frames with LRU eviction.
type BufferPool struct {
	// poolSize is the maximum number of frames in the buffer pool.
	poolSize int

	// frames is the array of buffer frames.
	frames []*Frame

	// pageTable maps page IDs to frame indices.
	pageTable map[page.PageID]int

	// freeList tracks available frame indices.
	freeList []int

	// lruList maintains LRU ordering of frames.
	lruList *list.List

	// mu protects all buffer pool state.
	mu sync.RWMutex

	// pageManager is used for page allocation and I/O.
	pageManager *page.Manager

	// Statistics
	stats Statistics
}

// Statistics tracks buffer pool performance metrics.
type Statistics struct {
	// TotalRequests is the total number of page requests.
	TotalRequests int64

	// CacheHits is the number of requests served from the buffer pool.
	CacheHits int64

	// CacheMisses is the number of requests requiring disk I/O.
	CacheMisses int64

	// Evictions is the number of pages evicted from the buffer pool.
	Evictions int64

	// DirtyEvictions is the number of dirty pages evicted.
	DirtyEvictions int64

	// PinnedPages is the current number of pinned pages.
	PinnedPages int64
}

// NewBufferPool creates a new buffer pool with the specified size.
func NewBufferPool(poolSize int, pageManager *page.Manager) *BufferPool {
	if poolSize <= 0 {
		poolSize = 1024 // Default size
	}

	bp := &BufferPool{
		poolSize:    poolSize,
		frames:      make([]*Frame, poolSize),
		pageTable:   make(map[page.PageID]int),
		freeList:    make([]int, poolSize),
		lruList:     list.New(),
		pageManager: pageManager,
	}

	// Initialize frames and free list
	for i := 0; i < poolSize; i++ {
		bp.frames[i] = &Frame{
			PageID:     page.InvalidPageID,
			Page:       nil,
			PinCount:   0,
			IsDirty:    false,
			LastAccess: time.Now(),
		}
		bp.freeList[i] = i
	}

	return bp
}

// GetPage retrieves a page from the buffer pool or loads it from storage.
func (bp *BufferPool) GetPage(pageID page.PageID) (*page.Page, error) {
	if pageID == page.InvalidPageID {
		return nil, fmt.Errorf("invalid page ID")
	}

	bp.mu.Lock()
	defer bp.mu.Unlock()

	atomic.AddInt64(&bp.stats.TotalRequests, 1)

	// Check if page is already in buffer pool
	if frameIndex, exists := bp.pageTable[pageID]; exists {
		atomic.AddInt64(&bp.stats.CacheHits, 1)
		frame := bp.frames[frameIndex]
		frame.Pin()
		bp.moveToFront(frame)
		return frame.Page, nil
	}

	atomic.AddInt64(&bp.stats.CacheMisses, 1)

	// Page not in buffer, need to load it
	frame, err := bp.allocateFrame()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate frame: %w", err)
	}

	// Load page from storage
	pg, err := bp.pageManager.GetPage(pageID)
	if err != nil {
		// Return frame to free list
		bp.returnFrame(frame)
		return nil, fmt.Errorf("failed to load page %d: %w", pageID, err)
	}

	// Initialize frame
	frame.PageID = pageID
	frame.Page = pg
	frame.IsDirty = false
	frame.Pin()
	frame.LastAccess = time.Now()

	// Add to page table and LRU list
	frameIndex := bp.getFrameIndex(frame)
	bp.pageTable[pageID] = frameIndex
	frame.LRUElement = bp.lruList.PushFront(frameIndex)

	return pg, nil
}

// UnpinPage unpins a page in the buffer pool.
func (bp *BufferPool) UnpinPage(pageID page.PageID, isDirty bool) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	frameIndex, exists := bp.pageTable[pageID]
	if !exists {
		return fmt.Errorf("page %d not found in buffer pool", pageID)
	}

	frame := bp.frames[frameIndex]
	frame.Unpin()

	if isDirty {
		frame.SetDirty()
	}

	// Update pinned pages count
	if !frame.IsPinned() {
		atomic.AddInt64(&bp.stats.PinnedPages, -1)
	}

	return nil
}

// FlushPage writes a specific page to storage if it's dirty.
func (bp *BufferPool) FlushPage(pageID page.PageID) error {
	bp.mu.RLock()
	frameIndex, exists := bp.pageTable[pageID]
	if !exists {
		bp.mu.RUnlock()
		return fmt.Errorf("page %d not found in buffer pool", pageID)
	}

	frame := bp.frames[frameIndex]
	bp.mu.RUnlock()

	if !frame.IsDirty {
		return nil // Nothing to flush
	}

	// TODO: Implement actual page writing to storage
	// For now, just mark as clean
	bp.mu.Lock()
	frame.IsDirty = false
	bp.mu.Unlock()

	return nil
}

// FlushAllPages writes all dirty pages to storage.
func (bp *BufferPool) FlushAllPages() error {
	bp.mu.RLock()
	var dirtyPages []page.PageID
	for pageID, frameIndex := range bp.pageTable {
		if bp.frames[frameIndex].IsDirty {
			dirtyPages = append(dirtyPages, pageID)
		}
	}
	bp.mu.RUnlock()

	for _, pageID := range dirtyPages {
		if err := bp.FlushPage(pageID); err != nil {
			return fmt.Errorf("failed to flush page %d: %w", pageID, err)
		}
	}

	return nil
}

// allocateFrame finds or creates an available frame.
func (bp *BufferPool) allocateFrame() (*Frame, error) {
	// Try to get a free frame first
	if len(bp.freeList) > 0 {
		frameIndex := bp.freeList[len(bp.freeList)-1]
		bp.freeList = bp.freeList[:len(bp.freeList)-1]
		return bp.frames[frameIndex], nil
	}

	// No free frames, need to evict using LRU
	return bp.evictFrame()
}

// evictFrame evicts the least recently used unpinned frame.
func (bp *BufferPool) evictFrame() (*Frame, error) {
	// Find LRU unpinned frame
	for element := bp.lruList.Back(); element != nil; element = element.Prev() {
		frameIndex := element.Value.(int)
		frame := bp.frames[frameIndex]

		if !frame.IsPinned() {
			// Found victim frame
			if frame.IsDirty {
				atomic.AddInt64(&bp.stats.DirtyEvictions, 1)
				// TODO: Flush page to storage before evicting
			}

			atomic.AddInt64(&bp.stats.Evictions, 1)

			// Remove from page table and LRU list
			delete(bp.pageTable, frame.PageID)
			bp.lruList.Remove(element)

			// Reset frame
			frame.PageID = page.InvalidPageID
			frame.Page = nil
			frame.IsDirty = false
			frame.LRUElement = nil

			return frame, nil
		}
	}

	return nil, fmt.Errorf("no unpinned frames available for eviction")
}

// returnFrame returns a frame to the free list.
func (bp *BufferPool) returnFrame(frame *Frame) {
	frameIndex := bp.getFrameIndex(frame)
	bp.freeList = append(bp.freeList, frameIndex)
}

// getFrameIndex returns the index of a frame in the frames array.
func (bp *BufferPool) getFrameIndex(frame *Frame) int {
	for i, f := range bp.frames {
		if f == frame {
			return i
		}
	}
	return -1 // Should never happen
}

// moveToFront moves a frame to the front of the LRU list.
func (bp *BufferPool) moveToFront(frame *Frame) {
	if frame.LRUElement != nil {
		bp.lruList.MoveToFront(frame.LRUElement)
	}
}

// GetStatistics returns a copy of the current buffer pool statistics.
func (bp *BufferPool) GetStatistics() Statistics {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	
	stats := bp.stats
	stats.PinnedPages = 0
	
	// Count pinned pages
	for _, frame := range bp.frames {
		if frame.IsPinned() {
			stats.PinnedPages++
		}
	}
	
	return stats
}

// GetHitRate returns the cache hit rate as a percentage.
func (bp *BufferPool) GetHitRate() float64 {
	stats := bp.GetStatistics()
	if stats.TotalRequests == 0 {
		return 0.0
	}
	return float64(stats.CacheHits) / float64(stats.TotalRequests) * 100.0
}

// Size returns the buffer pool size.
func (bp *BufferPool) Size() int {
	return bp.poolSize
}

// Close flushes all dirty pages and releases resources.
func (bp *BufferPool) Close() error {
	return bp.FlushAllPages()
}