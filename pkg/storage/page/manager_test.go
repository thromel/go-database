package page

import (
	"sync"
	"testing"
)

func TestManagerCreation(t *testing.T) {
	mgr := NewManager()
	
	// Check initial state
	if mgr.GetAllocatedPageCount() != 1 {
		t.Errorf("expected 1 allocated page (meta), got %d", mgr.GetAllocatedPageCount())
	}
	
	if mgr.GetFreePageCount() != 0 {
		t.Errorf("expected 0 free pages, got %d", mgr.GetFreePageCount())
	}
	
	if mgr.GetNextPageID() != 1 {
		t.Errorf("expected next page ID 1, got %d", mgr.GetNextPageID())
	}
	
	// Check meta page
	metaPage := mgr.GetMetaPage()
	if metaPage == nil {
		t.Fatal("meta page should not be nil")
	}
	
	if metaPage.ID() != 0 {
		t.Errorf("meta page should have ID 0, got %d", metaPage.ID())
	}
	
	if metaPage.Type() != PageTypeMeta {
		t.Errorf("meta page should have type META, got %v", metaPage.Type())
	}
}

func TestPageAllocation(t *testing.T) {
	mgr := NewManager()
	
	// Allocate multiple pages
	pageTypes := []PageType{PageTypeLeaf, PageTypeInternal, PageTypeOverflow}
	allocatedPages := make([]*Page, 0, len(pageTypes))
	
	for i, pt := range pageTypes {
		page, err := mgr.AllocatePage(pt)
		if err != nil {
			t.Fatalf("failed to allocate page %d: %v", i, err)
		}
		
		if page.Type() != pt {
			t.Errorf("expected page type %v, got %v", pt, page.Type())
		}
		
		allocatedPages = append(allocatedPages, page)
	}
	
	// Verify allocation count
	if mgr.GetAllocatedPageCount() != len(pageTypes)+1 { // +1 for meta page
		t.Errorf("expected %d allocated pages, got %d", len(pageTypes)+1, mgr.GetAllocatedPageCount())
	}
	
	// Verify page IDs are unique
	seenIDs := make(map[PageID]bool)
	for _, page := range allocatedPages {
		if seenIDs[page.ID()] {
			t.Errorf("duplicate page ID: %d", page.ID())
		}
		seenIDs[page.ID()] = true
	}
}

func TestPageDeallocation(t *testing.T) {
	mgr := NewManager()
	
	// Allocate some pages
	pages := make([]*Page, 5)
	for i := range pages {
		page, err := mgr.AllocatePage(PageTypeLeaf)
		if err != nil {
			t.Fatalf("failed to allocate page %d: %v", i, err)
		}
		pages[i] = page
	}
	
	initialCount := mgr.GetAllocatedPageCount()
	
	// Deallocate some pages
	for i := 0; i < 3; i++ {
		err := mgr.DeallocatePage(pages[i].ID())
		if err != nil {
			t.Fatalf("failed to deallocate page %d: %v", pages[i].ID(), err)
		}
	}
	
	// Check counts
	if mgr.GetFreePageCount() != 3 {
		t.Errorf("expected 3 free pages, got %d", mgr.GetFreePageCount())
	}
	
	if mgr.GetAllocatedPageCount() != initialCount-3 {
		t.Errorf("expected %d allocated pages, got %d", initialCount-3, mgr.GetAllocatedPageCount())
	}
}

func TestPageReuse(t *testing.T) {
	mgr := NewManager()
	
	// Allocate and deallocate a page
	page1, err := mgr.AllocatePage(PageTypeLeaf)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}
	
	page1ID := page1.ID()
	
	err = mgr.DeallocatePage(page1ID)
	if err != nil {
		t.Fatalf("failed to deallocate page: %v", err)
	}
	
	// Allocate a new page - should reuse the deallocated one
	page2, err := mgr.AllocatePage(PageTypeInternal)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}
	
	if page2.ID() != page1ID {
		t.Errorf("expected reused page ID %d, got %d", page1ID, page2.ID())
	}
	
	if mgr.GetFreePageCount() != 0 {
		t.Errorf("expected 0 free pages after reuse, got %d", mgr.GetFreePageCount())
	}
}

func TestInvalidOperations(t *testing.T) {
	mgr := NewManager()
	
	// Try to deallocate invalid page ID
	err := mgr.DeallocatePage(InvalidPageID)
	if err == nil {
		t.Error("expected error when deallocating invalid page ID")
	}
	
	// Try to deallocate meta page
	err = mgr.DeallocatePage(0)
	if err == nil {
		t.Error("expected error when deallocating meta page")
	}
	
	// Try to deallocate non-existent page
	err = mgr.DeallocatePage(999)
	if err == nil {
		t.Error("expected error when deallocating non-existent page")
	}
	
	// Try to get non-existent page
	_, err = mgr.GetPage(999)
	if err == nil {
		t.Error("expected error when getting non-existent page")
	}
}

func TestConcurrentAllocation(t *testing.T) {
	mgr := NewManager()
	numGoroutines := 10
	pagesPerGoroutine := 100
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)
	pageIDs := make(chan PageID, numGoroutines*pagesPerGoroutine)
	
	// Allocate pages concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for j := 0; j < pagesPerGoroutine; j++ {
				page, err := mgr.AllocatePage(PageTypeLeaf)
				if err != nil {
					errors <- err
					return
				}
				pageIDs <- page.ID()
			}
		}()
	}
	
	wg.Wait()
	close(errors)
	close(pageIDs)
	
	// Check for errors
	for err := range errors {
		t.Errorf("concurrent allocation error: %v", err)
	}
	
	// Verify all page IDs are unique
	seenIDs := make(map[PageID]bool)
	for id := range pageIDs {
		if seenIDs[id] {
			t.Errorf("duplicate page ID in concurrent allocation: %d", id)
		}
		seenIDs[id] = true
	}
	
	// Verify total count
	expectedCount := numGoroutines*pagesPerGoroutine + 1 // +1 for meta page
	if mgr.GetAllocatedPageCount() != expectedCount {
		t.Errorf("expected %d allocated pages, got %d", expectedCount, mgr.GetAllocatedPageCount())
	}
}

func TestStatistics(t *testing.T) {
	mgr := NewManager()
	
	// Allocate various page types
	pageTypes := []PageType{
		PageTypeLeaf, PageTypeLeaf, PageTypeLeaf,
		PageTypeInternal, PageTypeInternal,
		PageTypeOverflow,
	}
	
	for _, pt := range pageTypes {
		_, err := mgr.AllocatePage(pt)
		if err != nil {
			t.Fatalf("failed to allocate page: %v", err)
		}
	}
	
	stats := mgr.GetStatistics()
	
	// Check counts
	if stats.AllocatedPages != len(pageTypes)+1 { // +1 for meta
		t.Errorf("expected %d allocated pages, got %d", len(pageTypes)+1, stats.AllocatedPages)
	}
	
	// Check type counts
	expectedTypeCounts := map[PageType]int{
		PageTypeMeta:     1,
		PageTypeLeaf:     3,
		PageTypeInternal: 2,
		PageTypeOverflow: 1,
	}
	
	for pt, expected := range expectedTypeCounts {
		if stats.PageTypeCounts[pt] != expected {
			t.Errorf("expected %d pages of type %v, got %d", expected, pt, stats.PageTypeCounts[pt])
		}
	}
}

func BenchmarkPageAllocation(b *testing.B) {
	mgr := NewManager()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mgr.AllocatePage(PageTypeLeaf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConcurrentPageAllocation(b *testing.B) {
	mgr := NewManager()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := mgr.AllocatePage(PageTypeLeaf)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}