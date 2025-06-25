// Package file provides file-based persistent storage for the database engine.
// It implements atomic page writes, file locking, and corruption recovery.
package file

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/thromel/go-database/pkg/storage/page"
)

const (
	// DatabaseFileExtension is the default extension for database files
	DatabaseFileExtension = ".godb"

	// LockFileExtension is the extension for lock files
	LockFileExtension = ".lock"

	// DefaultFileMode is the default file permissions for database files
	DefaultFileMode = 0644

	// DefaultDirMode is the default directory permissions
	DefaultDirMode = 0755
)

// FileManager handles all file I/O operations for the database.
type FileManager struct {
	// filePath is the path to the database file
	filePath string

	// lockPath is the path to the lock file
	lockPath string

	// file is the database file handle
	file *os.File

	// lockFile is the lock file handle
	lockFile *os.File

	// mu protects file operations
	mu sync.RWMutex

	// fileSize tracks the current file size in bytes
	fileSize atomic.Int64

	// pageCount tracks the number of pages in the file
	pageCount atomic.Int64

	// Statistics
	stats   FileStatistics
	statsMu sync.RWMutex
}

// FileStatistics tracks file I/O performance metrics.
type FileStatistics struct {
	// TotalReads is the total number of page reads
	TotalReads int64

	// TotalWrites is the total number of page writes
	TotalWrites int64

	// TotalSyncs is the total number of sync operations
	TotalSyncs int64

	// BytesRead is the total bytes read from disk
	BytesRead int64

	// BytesWritten is the total bytes written to disk
	BytesWritten int64

	// CorruptionDetected is the number of corruption incidents
	CorruptionDetected int64
}

// Config holds file manager configuration.
type Config struct {
	// SyncWrites enables sync after each write operation
	SyncWrites bool

	// UseDirectIO enables direct I/O (bypassing OS cache)
	UseDirectIO bool

	// PreallocateSize is the initial file size to preallocate
	PreallocateSize int64
}

// DefaultConfig returns a default file manager configuration.
func DefaultConfig() *Config {
	return &Config{
		SyncWrites:      true,
		UseDirectIO:     false,
		PreallocateSize: 1024 * page.PageSize, // 1024 pages = 8MB
	}
}

// NewFileManager creates a new file manager for the given database path.
func NewFileManager(dbPath string, config *Config) (*FileManager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if dbPath == "" {
		return nil, errors.New("database path cannot be empty")
	}

	// Ensure path has correct extension
	if filepath.Ext(dbPath) == "" {
		dbPath += DatabaseFileExtension
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, DefaultDirMode); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	fm := &FileManager{
		filePath: dbPath,
		lockPath: dbPath + LockFileExtension,
	}

	// Acquire file lock
	if err := fm.acquireLock(); err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Open or create database file
	if err := fm.openFile(config); err != nil {
		_ = fm.releaseLock() // Clean up lock on failure, ignore error
		return nil, fmt.Errorf("failed to open database file: %w", err)
	}

	// Initialize file size and page count
	if err := fm.updateFileSizeInfo(); err != nil {
		fm.Close() // Clean up on failure
		return nil, fmt.Errorf("failed to update file size info: %w", err)
	}

	return fm, nil
}

// acquireLock creates and locks the lock file.
func (fm *FileManager) acquireLock() error {
	lockFile, err := os.OpenFile(fm.lockPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, DefaultFileMode)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("database is already in use by another process")
		}
		return fmt.Errorf("failed to create lock file: %w", err)
	}

	// Try to acquire an exclusive lock
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lockFile.Close()
		os.Remove(fm.lockPath)
		return fmt.Errorf("failed to acquire file lock: %w", err)
	}

	fm.lockFile = lockFile
	return nil
}

// releaseLock releases the file lock.
func (fm *FileManager) releaseLock() error {
	if fm.lockFile == nil {
		return nil
	}

	// Release the lock
	_ = syscall.Flock(int(fm.lockFile.Fd()), syscall.LOCK_UN)

	// Close and remove lock file
	fm.lockFile.Close()
	fm.lockFile = nil

	return os.Remove(fm.lockPath)
}

// openFile opens or creates the database file.
func (fm *FileManager) openFile(config *Config) error {
	flags := os.O_RDWR | os.O_CREATE

	// Add direct I/O flag if enabled (platform-specific)
	if config.UseDirectIO {
		flags |= getDirectIOFlag()
	}

	file, err := os.OpenFile(fm.filePath, flags, DefaultFileMode)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", fm.filePath, err)
	}

	fm.file = file

	// Preallocate file space if needed
	if config.PreallocateSize > 0 {
		if err := fm.preallocate(config.PreallocateSize); err != nil {
			return fmt.Errorf("failed to preallocate file space: %w", err)
		}
	}

	return nil
}

// preallocate extends the file to the specified size.
func (fm *FileManager) preallocate(size int64) error {
	stat, err := fm.file.Stat()
	if err != nil {
		return err
	}

	if stat.Size() < size {
		return fm.file.Truncate(size)
	}

	return nil
}

// updateFileSizeInfo updates the cached file size and page count.
func (fm *FileManager) updateFileSizeInfo() error {
	stat, err := fm.file.Stat()
	if err != nil {
		return err
	}

	size := stat.Size()
	fm.fileSize.Store(size)
	fm.pageCount.Store(size / page.PageSize)

	return nil
}

// ReadPage reads a page from the file at the specified page ID.
func (fm *FileManager) ReadPage(pageID page.PageID) (*page.Page, error) {
	if pageID == page.InvalidPageID {
		return nil, errors.New("invalid page ID")
	}

	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if fm.file == nil {
		return nil, errors.New("file manager is closed")
	}

	// Calculate file offset
	offset := int64(pageID) * page.PageSize

	// Check if page exists in file
	if offset >= fm.fileSize.Load() {
		return nil, fmt.Errorf("page %d does not exist in file", pageID)
	}

	// Read page data
	buffer := make([]byte, page.PageSize)
	n, err := fm.file.ReadAt(buffer, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to read page %d: %w", pageID, err)
	}

	if n != page.PageSize {
		return nil, fmt.Errorf("incomplete page read: expected %d bytes, got %d", page.PageSize, n)
	}

	// Update statistics
	fm.statsMu.Lock()
	fm.stats.TotalReads++
	fm.stats.BytesRead += int64(n)
	fm.statsMu.Unlock()

	// Deserialize page
	pg := &page.Page{}
	if err := pg.Deserialize(buffer); err != nil {
		fm.statsMu.Lock()
		fm.stats.CorruptionDetected++
		fm.statsMu.Unlock()
		return nil, fmt.Errorf("failed to deserialize page %d: %w", pageID, err)
	}

	return pg, nil
}

// WritePage writes a page to the file at the specified page ID.
func (fm *FileManager) WritePage(pg *page.Page) error {
	if pg == nil {
		return errors.New("page cannot be nil")
	}

	pageID := pg.ID()
	if pageID == page.InvalidPageID {
		return errors.New("invalid page ID")
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	if fm.file == nil {
		return errors.New("file manager is closed")
	}

	// Serialize page
	buffer, err := pg.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize page %d: %w", pageID, err)
	}

	// Calculate file offset
	offset := int64(pageID) * page.PageSize

	// Extend file if necessary
	if offset >= fm.fileSize.Load() {
		if err := fm.extendFile(offset + page.PageSize); err != nil {
			return fmt.Errorf("failed to extend file: %w", err)
		}
	}

	// Write page data atomically
	if err := fm.writePageAtomic(buffer, offset); err != nil {
		return fmt.Errorf("failed to write page %d: %w", pageID, err)
	}

	// Update statistics
	fm.statsMu.Lock()
	fm.stats.TotalWrites++
	fm.stats.BytesWritten += int64(len(buffer))
	fm.statsMu.Unlock()

	return nil
}

// writePageAtomic performs an atomic page write operation.
func (fm *FileManager) writePageAtomic(buffer []byte, offset int64) error {
	// Write data
	n, err := fm.file.WriteAt(buffer, offset)
	if err != nil {
		return err
	}

	if n != len(buffer) {
		return fmt.Errorf("incomplete write: expected %d bytes, wrote %d", len(buffer), n)
	}

	// Force sync to ensure data is written to disk
	if err := fm.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	fm.statsMu.Lock()
	fm.stats.TotalSyncs++
	fm.statsMu.Unlock()

	return nil
}

// extendFile extends the file to the specified size.
func (fm *FileManager) extendFile(newSize int64) error {
	if err := fm.file.Truncate(newSize); err != nil {
		return err
	}

	fm.fileSize.Store(newSize)
	fm.pageCount.Store(newSize / page.PageSize)

	return nil
}

// Sync forces all pending writes to disk.
func (fm *FileManager) Sync() error {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if fm.file == nil {
		return errors.New("file manager is closed")
	}

	if err := fm.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	fm.statsMu.Lock()
	fm.stats.TotalSyncs++
	fm.statsMu.Unlock()

	return nil
}

// GetPageCount returns the number of pages in the file.
func (fm *FileManager) GetPageCount() int64 {
	return fm.pageCount.Load()
}

// GetFileSize returns the current file size in bytes.
func (fm *FileManager) GetFileSize() int64 {
	return fm.fileSize.Load()
}

// GetPath returns the database file path.
func (fm *FileManager) GetPath() string {
	return fm.filePath
}

// GetStatistics returns a copy of the current file statistics.
func (fm *FileManager) GetStatistics() FileStatistics {
	fm.statsMu.RLock()
	defer fm.statsMu.RUnlock()
	return fm.stats
}

// CheckIntegrity performs basic file integrity checks.
func (fm *FileManager) CheckIntegrity() error {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if fm.file == nil {
		return errors.New("file manager is closed")
	}

	// Update file size info
	if err := fm.updateFileSizeInfo(); err != nil {
		return fmt.Errorf("failed to update file size: %w", err)
	}

	fileSize := fm.fileSize.Load()

	// Check if file size is multiple of page size
	if fileSize%page.PageSize != 0 {
		return fmt.Errorf("file size %d is not a multiple of page size %d", fileSize, page.PageSize)
	}

	// Note: We skip checking individual pages for corruption here since
	// the file might contain preallocated space with zero data.
	// In a production implementation, we would maintain a bitmap or metadata
	// to track which pages have been written and only validate those.

	return nil
}

// Close closes the file manager and releases all resources.
func (fm *FileManager) Close() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	var errs []error

	// Close database file
	if fm.file != nil {
		if err := fm.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close database file: %w", err))
		}
		fm.file = nil
	}

	// Release file lock
	if err := fm.releaseLock(); err != nil {
		errs = append(errs, fmt.Errorf("failed to release lock: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	return nil
}
