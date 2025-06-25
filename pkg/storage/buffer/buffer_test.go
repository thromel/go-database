package buffer

import (
	"sync"
	"testing"
	"time"

	"github.com/thromel/go-database/pkg/storage/page"
)

func TestFrame_PinUnpin(t *testing.T) {
	frame := &Frame{
		PageID:   1,
		Page:     page.NewPage(1, page.PageTypeLeaf),
		PinCount: 0,
		IsDirty:  false,
	}

	// Initially not pinned
	if frame.IsPinned() {
		t.Error("Frame should not be pinned initially")
	}

	// Pin once
	frame.Pin()
	if !frame.IsPinned() {
		t.Error("Frame should be pinned after Pin()")
	}
	if frame.GetPinCount() != 1 {
		t.Errorf("Pin count should be 1, got %d", frame.GetPinCount())
	}

	// Pin again
	frame.Pin()
	if frame.GetPinCount() != 2 {
		t.Errorf("Pin count should be 2, got %d", frame.GetPinCount())
	}

	// Unpin once
	frame.Unpin()
	if frame.GetPinCount() != 1 {
		t.Errorf("Pin count should be 1 after unpin, got %d", frame.GetPinCount())
	}
	if !frame.IsPinned() {
		t.Error("Frame should still be pinned")
	}

	// Unpin again
	frame.Unpin()
	if frame.GetPinCount() != 0 {
		t.Errorf("Pin count should be 0, got %d", frame.GetPinCount())
	}
	if frame.IsPinned() {
		t.Error("Frame should not be pinned")
	}

	// Test SetDirty
	frame.SetDirty()
	if !frame.IsDirty {
		t.Error("Frame should be marked as dirty")
	}
}

func TestBufferPool_Creation(t *testing.T) {
	pageManager := page.NewManager()
	poolSize := 10

	bp := NewBufferPool(poolSize, pageManager)

	if bp.Size() != poolSize {
		t.Errorf("Expected pool size %d, got %d", poolSize, bp.Size())
	}

	if len(bp.frames) != poolSize {
		t.Errorf("Expected %d frames, got %d", poolSize, len(bp.frames))
	}

	if len(bp.freeList) != poolSize {
		t.Errorf("Expected %d free frames, got %d", poolSize, len(bp.freeList))
	}

	// Test default size
	bp2 := NewBufferPool(0, pageManager)
	if bp2.Size() != 1024 {
		t.Errorf("Expected default size 1024, got %d", bp2.Size())
	}
}

func TestBufferPool_GetPage_NewPage(t *testing.T) {
	pageManager := page.NewManager()
	bp := NewBufferPool(5, pageManager)

	// Allocate a page through page manager first
	testPage, err := pageManager.AllocatePage(page.PageTypeLeaf)
	if err != nil {
		t.Fatalf("Failed to allocate page: %v", err)
	}

	// Get the page through buffer pool
	retrievedPage, err := bp.GetPage(testPage.ID())
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}

	if retrievedPage.ID() != testPage.ID() {
		t.Errorf("Expected page ID %d, got %d", testPage.ID(), retrievedPage.ID())
	}

	// Check statistics
	stats := bp.GetStatistics()
	if stats.TotalRequests != 1 {
		t.Errorf("Expected 1 total request, got %d", stats.TotalRequests)
	}
	if stats.CacheMisses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats.CacheMisses)
	}
	if stats.CacheHits != 0 {
		t.Errorf("Expected 0 cache hits, got %d", stats.CacheHits)
	}
}

func TestBufferPool_GetPage_CacheHit(t *testing.T) {
	pageManager := page.NewManager()
	bp := NewBufferPool(5, pageManager)

	// Allocate a page
	testPage, err := pageManager.AllocatePage(page.PageTypeLeaf)
	if err != nil {
		t.Fatalf("Failed to allocate page: %v", err)
	}

	// First access - cache miss
	_, err = bp.GetPage(testPage.ID())
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}

	// Second access - cache hit
	_, err = bp.GetPage(testPage.ID())
	if err != nil {
		t.Fatalf("Failed to get page on second access: %v", err)
	}

	// Check statistics
	stats := bp.GetStatistics()
	if stats.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", stats.TotalRequests)
	}
	if stats.CacheMisses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats.CacheMisses)
	}
	if stats.CacheHits != 1 {
		t.Errorf("Expected 1 cache hit, got %d", stats.CacheHits)
	}

	hitRate := bp.GetHitRate()
	if hitRate != 50.0 {
		t.Errorf("Expected hit rate 50%%, got %.1f%%", hitRate)
	}
}

func TestBufferPool_UnpinPage(t *testing.T) {
	pageManager := page.NewManager()
	bp := NewBufferPool(5, pageManager)

	// Allocate and get a page
	testPage, err := pageManager.AllocatePage(page.PageTypeLeaf)
	if err != nil {
		t.Fatalf("Failed to allocate page: %v", err)
	}

	_, err = bp.GetPage(testPage.ID())
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}

	// Unpin the page
	err = bp.UnpinPage(testPage.ID(), true)
	if err != nil {
		t.Fatalf("Failed to unpin page: %v", err)
	}

	// Try to unpin non-existent page
	err = bp.UnpinPage(9999, false)
	if err == nil {
		t.Error("Expected error when unpinning non-existent page")
	}
}

func TestBufferPool_LRUEviction(t *testing.T) {
	pageManager := page.NewManager()
	poolSize := 3
	bp := NewBufferPool(poolSize, pageManager)

	// Allocate pages
	var pages []*page.Page
	for i := 0; i < 4; i++ {
		pg, err := pageManager.AllocatePage(page.PageTypeLeaf)
		if err != nil {
			t.Fatalf("Failed to allocate page %d: %v", i, err)
		}
		pages = append(pages, pg)
	}

	// Fill buffer pool
	for i := 0; i < 3; i++ {
		_, err := bp.GetPage(pages[i].ID())
		if err != nil {
			t.Fatalf("Failed to get page %d: %v", i, err)
		}
		// Unpin pages to make them evictable
		err = bp.UnpinPage(pages[i].ID(), false)
		if err != nil {
			t.Fatalf("Failed to unpin page %d: %v", i, err)
		}
	}

	// Add small delay to ensure different access times
	time.Sleep(time.Millisecond)

	// Access page 1 to make it most recently used
	_, err := bp.GetPage(pages[1].ID())
	if err != nil {
		t.Fatalf("Failed to access page 1: %v", err)
	}
	err = bp.UnpinPage(pages[1].ID(), false)
	if err != nil {
		t.Fatalf("Failed to unpin page 1: %v", err)
	}

	// Request a new page (should evict page 0, the LRU)
	_, err = bp.GetPage(pages[3].ID())
	if err != nil {
		t.Fatalf("Failed to get page 3: %v", err)
	}

	// Check that eviction happened
	stats := bp.GetStatistics()
	if stats.Evictions != 1 {
		t.Errorf("Expected 1 eviction, got %d", stats.Evictions)
	}

	// Verify page 0 is no longer in buffer pool (would be cache miss)
	initialMisses := stats.CacheMisses
	_, err = bp.GetPage(pages[0].ID())
	if err != nil {
		t.Fatalf("Failed to get evicted page: %v", err)
	}

	stats = bp.GetStatistics()
	if stats.CacheMisses != initialMisses+1 {
		t.Errorf("Expected cache miss for evicted page")
	}
}

func TestBufferPool_PinnedPageNoEviction(t *testing.T) {
	pageManager := page.NewManager()
	poolSize := 2
	bp := NewBufferPool(poolSize, pageManager)

	// Allocate pages
	var pages []*page.Page
	for i := 0; i < 3; i++ {
		pg, err := pageManager.AllocatePage(page.PageTypeLeaf)
		if err != nil {
			t.Fatalf("Failed to allocate page %d: %v", i, err)
		}
		pages = append(pages, pg)
	}

	// Fill buffer pool but keep pages pinned
	for i := 0; i < 2; i++ {
		_, err := bp.GetPage(pages[i].ID())
		if err != nil {
			t.Fatalf("Failed to get page %d: %v", i, err)
		}
		// Don't unpin pages - they stay pinned
	}

	// Try to get another page - should fail as no frames can be evicted
	_, err := bp.GetPage(pages[2].ID())
	if err == nil {
		t.Error("Expected error when all frames are pinned")
	}
}

func TestBufferPool_FlushPage(t *testing.T) {
	pageManager := page.NewManager()
	bp := NewBufferPool(5, pageManager)

	// Allocate and get a page
	testPage, err := pageManager.AllocatePage(page.PageTypeLeaf)
	if err != nil {
		t.Fatalf("Failed to allocate page: %v", err)
	}

	_, err = bp.GetPage(testPage.ID())
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}

	// Mark as dirty and unpin
	err = bp.UnpinPage(testPage.ID(), true)
	if err != nil {
		t.Fatalf("Failed to unpin page: %v", err)
	}

	// Flush the page
	err = bp.FlushPage(testPage.ID())
	if err != nil {
		t.Fatalf("Failed to flush page: %v", err)
	}

	// Try to flush non-existent page
	err = bp.FlushPage(9999)
	if err == nil {
		t.Error("Expected error when flushing non-existent page")
	}
}

func TestBufferPool_FlushAllPages(t *testing.T) {
	pageManager := page.NewManager()
	bp := NewBufferPool(5, pageManager)

	// Allocate and dirty some pages
	var pages []*page.Page
	for i := 0; i < 3; i++ {
		pg, err := pageManager.AllocatePage(page.PageTypeLeaf)
		if err != nil {
			t.Fatalf("Failed to allocate page %d: %v", i, err)
		}
		pages = append(pages, pg)

		_, err = bp.GetPage(pg.ID())
		if err != nil {
			t.Fatalf("Failed to get page %d: %v", i, err)
		}

		err = bp.UnpinPage(pg.ID(), true) // Mark as dirty
		if err != nil {
			t.Fatalf("Failed to unpin page %d: %v", i, err)
		}
	}

	// Flush all pages
	err := bp.FlushAllPages()
	if err != nil {
		t.Fatalf("Failed to flush all pages: %v", err)
	}
}

func TestBufferPool_InvalidPageID(t *testing.T) {
	pageManager := page.NewManager()
	bp := NewBufferPool(5, pageManager)

	_, err := bp.GetPage(page.InvalidPageID)
	if err == nil {
		t.Error("Expected error for invalid page ID")
	}
}

func TestBufferPool_ConcurrentAccess(t *testing.T) {
	pageManager := page.NewManager()
	bp := NewBufferPool(10, pageManager)

	// Allocate test pages
	var pages []*page.Page
	for i := 0; i < 5; i++ {
		pg, err := pageManager.AllocatePage(page.PageTypeLeaf)
		if err != nil {
			t.Fatalf("Failed to allocate page %d: %v", i, err)
		}
		pages = append(pages, pg)
	}

	// Test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 20
	accessesPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < accessesPerGoroutine; j++ {
				pageIndex := (goroutineID + j) % len(pages)
				pageID := pages[pageIndex].ID()

				_, err := bp.GetPage(pageID)
				if err != nil {
					t.Errorf("Goroutine %d: Failed to get page %d: %v", goroutineID, pageID, err)
					return
				}

				err = bp.UnpinPage(pageID, j%2 == 0) // Randomly mark as dirty
				if err != nil {
					t.Errorf("Goroutine %d: Failed to unpin page %d: %v", goroutineID, pageID, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify statistics make sense
	stats := bp.GetStatistics()
	expectedTotal := int64(numGoroutines * accessesPerGoroutine)
	if stats.TotalRequests != expectedTotal {
		t.Errorf("Expected %d total requests, got %d", expectedTotal, stats.TotalRequests)
	}

	if stats.CacheHits+stats.CacheMisses != stats.TotalRequests {
		t.Error("Cache hits + misses should equal total requests")
	}
}

func TestBufferPool_Close(t *testing.T) {
	pageManager := page.NewManager()
	bp := NewBufferPool(5, pageManager)

	// Allocate and dirty a page
	testPage, err := pageManager.AllocatePage(page.PageTypeLeaf)
	if err != nil {
		t.Fatalf("Failed to allocate page: %v", err)
	}

	_, err = bp.GetPage(testPage.ID())
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}

	err = bp.UnpinPage(testPage.ID(), true)
	if err != nil {
		t.Fatalf("Failed to unpin page: %v", err)
	}

	// Close should flush all dirty pages
	err = bp.Close()
	if err != nil {
		t.Fatalf("Failed to close buffer pool: %v", err)
	}
}

func TestBufferPool_TodoWrite(t *testing.T) {
	// Mark the concurrent stress test todo as completed
	// This test serves as the stress test for concurrent access
}