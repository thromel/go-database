package file

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thromel/go-database/pkg/storage/page"
)

func TestNewFileManager(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	fm, err := NewFileManager(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to create file manager: %v", err)
	}
	defer fm.Close()

	if fm.GetPath() != dbPath {
		t.Errorf("Expected path %s, got %s", dbPath, fm.GetPath())
	}

	// Check that file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Check that lock file exists
	lockPath := dbPath + LockFileExtension
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("Lock file was not created")
	}
}

func TestNewFileManager_WithConfig(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test")

	config := &Config{
		SyncWrites:      false,
		UseDirectIO:     false,
		PreallocateSize: 2 * page.PageSize,
	}

	fm, err := NewFileManager(dbPath, config)
	if err != nil {
		t.Fatalf("Failed to create file manager: %v", err)
	}
	defer fm.Close()

	// Check that extension was added
	expectedPath := dbPath + DatabaseFileExtension
	if fm.GetPath() != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, fm.GetPath())
	}

	// Check preallocated size
	if fm.GetFileSize() < config.PreallocateSize {
		t.Errorf("File was not preallocated to expected size")
	}
}

func TestFileManager_FileLocking(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	// Create first file manager
	fm1, err := NewFileManager(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to create first file manager: %v", err)
	}
	defer fm1.Close()

	// Try to create second file manager for same file - should fail
	_, err = NewFileManager(dbPath, nil)
	if err == nil {
		t.Error("Expected error when creating second file manager for same file")
	}
}

func TestFileManager_WriteAndReadPage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	fm, err := NewFileManager(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to create file manager: %v", err)
	}
	defer fm.Close()

	// Create a test page
	testPage := page.NewPage(1, page.PageTypeLeaf)
	testData := []byte("Hello, World!")
	copy(testPage.Data(), testData)

	// Write the page
	if err := fm.WritePage(testPage); err != nil {
		t.Fatalf("Failed to write page: %v", err)
	}

	// Read the page back
	readPage, err := fm.ReadPage(1)
	if err != nil {
		t.Fatalf("Failed to read page: %v", err)
	}

	// Verify the page
	if readPage.ID() != testPage.ID() {
		t.Errorf("Expected page ID %d, got %d", testPage.ID(), readPage.ID())
	}

	if readPage.Type() != testPage.Type() {
		t.Errorf("Expected page type %v, got %v", testPage.Type(), readPage.Type())
	}

	// Verify data
	readData := readPage.Data()[:len(testData)]
	for i, b := range testData {
		if readData[i] != b {
			t.Errorf("Data mismatch at position %d: expected %d, got %d", i, b, readData[i])
			break
		}
	}
}

func TestFileManager_MultiplePages(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	fm, err := NewFileManager(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to create file manager: %v", err)
	}
	defer fm.Close()

	// Write multiple pages
	numPages := 10
	pages := make([]*page.Page, numPages)

	for i := 0; i < numPages; i++ {
		pg := page.NewPage(page.PageID(i+1), page.PageTypeLeaf)
		testData := []byte{byte(i), byte(i + 1), byte(i + 2)}
		copy(pg.Data(), testData)
		pages[i] = pg

		if err := fm.WritePage(pg); err != nil {
			t.Fatalf("Failed to write page %d: %v", i+1, err)
		}
	}

	// Verify that we have at least the number of pages we wrote
	// Note: File might be preallocated, so it could have more pages
	minExpectedCount := int64(numPages)
	if fm.GetPageCount() < minExpectedCount {
		t.Errorf("Expected at least %d pages, got %d", minExpectedCount, fm.GetPageCount())
	}

	// Read and verify all pages
	for i, expectedPage := range pages {
		readPage, err := fm.ReadPage(page.PageID(i + 1))
		if err != nil {
			t.Fatalf("Failed to read page %d: %v", i+1, err)
		}

		if readPage.ID() != expectedPage.ID() {
			t.Errorf("Page %d: expected ID %d, got %d", i+1, expectedPage.ID(), readPage.ID())
		}

		// Verify first few bytes of data
		expectedData := expectedPage.Data()[:3]
		readData := readPage.Data()[:3]
		for j := 0; j < 3; j++ {
			if readData[j] != expectedData[j] {
				t.Errorf("Page %d data mismatch at position %d: expected %d, got %d",
					i+1, j, expectedData[j], readData[j])
			}
		}
	}
}

func TestFileManager_InvalidOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	fm, err := NewFileManager(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to create file manager: %v", err)
	}
	defer fm.Close()

	// Test reading non-existent page
	_, err = fm.ReadPage(999)
	if err == nil {
		t.Error("Expected error when reading non-existent page")
	}

	// Test writing nil page
	err = fm.WritePage(nil)
	if err == nil {
		t.Error("Expected error when writing nil page")
	}

	// Test invalid page ID
	_, err = fm.ReadPage(page.InvalidPageID)
	if err == nil {
		t.Error("Expected error when reading invalid page ID")
	}

	invalidPage := page.NewPage(page.InvalidPageID, page.PageTypeLeaf)
	err = fm.WritePage(invalidPage)
	if err == nil {
		t.Error("Expected error when writing page with invalid ID")
	}
}

func TestFileManager_Statistics(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	fm, err := NewFileManager(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to create file manager: %v", err)
	}
	defer fm.Close()

	// Initial statistics should be zero
	stats := fm.GetStatistics()
	if stats.TotalReads != 0 || stats.TotalWrites != 0 {
		t.Error("Initial statistics should be zero")
	}

	// Write a page
	testPage := page.NewPage(1, page.PageTypeLeaf)
	if err := fm.WritePage(testPage); err != nil {
		t.Fatalf("Failed to write page: %v", err)
	}

	// Check write statistics
	stats = fm.GetStatistics()
	if stats.TotalWrites != 1 {
		t.Errorf("Expected 1 write, got %d", stats.TotalWrites)
	}
	if stats.BytesWritten != page.PageSize {
		t.Errorf("Expected %d bytes written, got %d", page.PageSize, stats.BytesWritten)
	}

	// Read the page
	_, err = fm.ReadPage(1)
	if err != nil {
		t.Fatalf("Failed to read page: %v", err)
	}

	// Check read statistics
	stats = fm.GetStatistics()
	if stats.TotalReads != 1 {
		t.Errorf("Expected 1 read, got %d", stats.TotalReads)
	}
	if stats.BytesRead != page.PageSize {
		t.Errorf("Expected %d bytes read, got %d", page.PageSize, stats.BytesRead)
	}
}

func TestFileManager_Sync(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	fm, err := NewFileManager(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to create file manager: %v", err)
	}
	defer fm.Close()

	// Test sync
	if err := fm.Sync(); err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}

	// Check sync statistics
	stats := fm.GetStatistics()
	if stats.TotalSyncs == 0 {
		t.Error("Expected at least one sync operation")
	}
}

func TestFileManager_CheckIntegrity(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	fm, err := NewFileManager(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to create file manager: %v", err)
	}
	defer fm.Close()

	// Write some valid pages (using valid page IDs starting from 1)
	for i := 1; i <= 5; i++ {
		pg := page.NewPage(page.PageID(i), page.PageTypeLeaf)
		if err := fm.WritePage(pg); err != nil {
			t.Fatalf("Failed to write page %d: %v", i, err)
		}
	}

	// Check integrity - should pass
	if err := fm.CheckIntegrity(); err != nil {
		t.Fatalf("Integrity check failed: %v", err)
	}
}

func TestFileManager_Close(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	fm, err := NewFileManager(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to create file manager: %v", err)
	}

	// Close the file manager
	if err := fm.Close(); err != nil {
		t.Fatalf("Failed to close file manager: %v", err)
	}

	// Operations should fail after closing
	_, err = fm.ReadPage(1)
	if err == nil {
		t.Error("Expected error when reading from closed file manager")
	}

	err = fm.WritePage(page.NewPage(1, page.PageTypeLeaf))
	if err == nil {
		t.Error("Expected error when writing to closed file manager")
	}

	// Lock file should be removed
	lockPath := dbPath + LockFileExtension
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("Lock file should be removed after closing")
	}
}

func TestFileManager_EmptyPath(t *testing.T) {
	_, err := NewFileManager("", nil)
	if err == nil {
		t.Error("Expected error when creating file manager with empty path")
	}
}

func TestFileManager_FileGrowth(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	// Use config without preallocation
	config := &Config{
		SyncWrites:      true,
		UseDirectIO:     false,
		PreallocateSize: 0, // No preallocation
	}

	fm, err := NewFileManager(dbPath, config)
	if err != nil {
		t.Fatalf("Failed to create file manager: %v", err)
	}
	defer fm.Close()

	initialSize := fm.GetFileSize()

	// Write a page that should extend the file
	largePageID := page.PageID(100) // This should be beyond initial file size
	testPage := page.NewPage(largePageID, page.PageTypeLeaf)

	if err := fm.WritePage(testPage); err != nil {
		t.Fatalf("Failed to write page: %v", err)
	}

	// File should have grown
	newSize := fm.GetFileSize()
	expectedMinSize := int64(largePageID+1) * page.PageSize

	if newSize < expectedMinSize {
		t.Errorf("File did not grow as expected: got %d, expected at least %d", newSize, expectedMinSize)
	}

	if newSize <= initialSize {
		t.Error("File size should have increased")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if !config.SyncWrites {
		t.Error("Expected SyncWrites to be true by default")
	}

	if config.UseDirectIO {
		t.Error("Expected UseDirectIO to be false by default")
	}

	if config.PreallocateSize <= 0 {
		t.Error("Expected PreallocateSize to be positive")
	}
}

func TestFileManager_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.godb")

	fm, err := NewFileManager(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to create file manager: %v", err)
	}
	defer fm.Close()

	// Write initial pages
	numPages := 20
	for i := 0; i < numPages; i++ {
		pg := page.NewPage(page.PageID(i+1), page.PageTypeLeaf)
		if err := fm.WritePage(pg); err != nil {
			t.Fatalf("Failed to write initial page %d: %v", i+1, err)
		}
	}

	// Test concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				pageID := page.PageID((goroutineID*j)%numPages + 1)
				_, err := fm.ReadPage(pageID)
				if err != nil {
					t.Errorf("Goroutine %d: Failed to read page %d: %v", goroutineID, pageID, err)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent read test timed out")
		}
	}

	// Verify statistics
	stats := fm.GetStatistics()
	if stats.TotalReads < 1000 {
		t.Errorf("Expected at least 1000 reads, got %d", stats.TotalReads)
	}
}
