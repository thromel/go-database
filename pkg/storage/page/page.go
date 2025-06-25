// Package page provides page-based storage for the database engine.
// Pages are fixed-size units of storage that form the foundation of the
// database's persistent storage layer.
package page

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// PageSize defines the size of a database page in bytes.
// We use 8KB pages as a balance between memory efficiency and I/O performance.
const PageSize = 8192

// PageHeaderSize is the size of the page header in bytes.
const PageHeaderSize = 32

// PageID uniquely identifies a page in the database.
type PageID uint32

// InvalidPageID represents an invalid or null page ID.
const InvalidPageID PageID = 0

// PageType represents the type of a database page.
type PageType uint8

const (
	// PageTypeLeaf represents a B+ tree leaf page containing key-value pairs.
	PageTypeLeaf PageType = iota
	// PageTypeInternal represents a B+ tree internal page containing keys and child pointers.
	PageTypeInternal
	// PageTypeMeta represents a metadata page containing database-level information.
	PageTypeMeta
	// PageTypeFree represents a free page available for allocation.
	PageTypeFree
	// PageTypeOverflow represents an overflow page for large values.
	PageTypeOverflow
)

// String returns the string representation of a PageType.
func (pt PageType) String() string {
	switch pt {
	case PageTypeLeaf:
		return "LEAF"
	case PageTypeInternal:
		return "INTERNAL"
	case PageTypeMeta:
		return "META"
	case PageTypeFree:
		return "FREE"
	case PageTypeOverflow:
		return "OVERFLOW"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", pt)
	}
}

// PageHeader contains metadata about a page.
type PageHeader struct {
	// PageID is the unique identifier for this page.
	PageID PageID
	// PageType indicates the type of this page.
	PageType PageType
	// LSN is the Log Sequence Number for recovery.
	LSN uint64
	// NumSlots is the number of data slots in this page.
	NumSlots uint16
	// FreeSpace is the amount of free space in bytes.
	FreeSpace uint16
	// FreeSpacePtr is the offset to the start of free space.
	FreeSpacePtr uint16
	// NextPage is the ID of the next page (for linked pages).
	NextPage PageID
	// Checksum is the CRC32 checksum of the page data.
	Checksum uint32
}

// Page represents a fixed-size unit of storage in the database.
type Page struct {
	header PageHeader
	data   [PageSize - PageHeaderSize]byte
}

// NewPage creates a new page with the given ID and type.
func NewPage(id PageID, pageType PageType) *Page {
	p := &Page{
		header: PageHeader{
			PageID:       id,
			PageType:     pageType,
			LSN:          0,
			NumSlots:     0,
			FreeSpace:    PageSize - PageHeaderSize,
			FreeSpacePtr: PageHeaderSize,
			NextPage:     InvalidPageID,
			Checksum:     0,
		},
	}
	return p
}

// ID returns the page ID.
func (p *Page) ID() PageID {
	return p.header.PageID
}

// Type returns the page type.
func (p *Page) Type() PageType {
	return p.header.PageType
}

// LSN returns the Log Sequence Number.
func (p *Page) LSN() uint64 {
	return p.header.LSN
}

// SetLSN sets the Log Sequence Number.
func (p *Page) SetLSN(lsn uint64) {
	p.header.LSN = lsn
}

// NumSlots returns the number of slots in the page.
func (p *Page) NumSlots() uint16 {
	return p.header.NumSlots
}

// FreeSpace returns the amount of free space in bytes.
func (p *Page) FreeSpace() uint16 {
	return p.header.FreeSpace
}

// NextPage returns the ID of the next page.
func (p *Page) NextPage() PageID {
	return p.header.NextPage
}

// SetNextPage sets the ID of the next page.
func (p *Page) SetNextPage(next PageID) {
	p.header.NextPage = next
}

// Data returns a reference to the page's data section.
func (p *Page) Data() []byte {
	return p.data[:]
}

// Serialize writes the page to a byte slice.
func (p *Page) Serialize() ([]byte, error) {
	buf := make([]byte, PageSize)

	// Write header
	binary.LittleEndian.PutUint32(buf[0:4], uint32(p.header.PageID))
	buf[4] = byte(p.header.PageType)
	binary.LittleEndian.PutUint64(buf[5:13], p.header.LSN)
	binary.LittleEndian.PutUint16(buf[13:15], p.header.NumSlots)
	binary.LittleEndian.PutUint16(buf[15:17], p.header.FreeSpace)
	binary.LittleEndian.PutUint16(buf[17:19], p.header.FreeSpacePtr)
	binary.LittleEndian.PutUint32(buf[19:23], uint32(p.header.NextPage))
	// Checksum is written at position 28-32 after calculation

	// Copy data
	copy(buf[PageHeaderSize:], p.data[:])

	// Calculate and write checksum (excluding checksum field itself)
	checksum := calculateChecksum(buf[:28], buf[32:])
	binary.LittleEndian.PutUint32(buf[28:32], checksum)

	return buf, nil
}

// Deserialize reads a page from a byte slice.
func (p *Page) Deserialize(buf []byte) error {
	if len(buf) != PageSize {
		return fmt.Errorf("invalid buffer size: expected %d, got %d", PageSize, len(buf))
	}

	// Read header
	p.header.PageID = PageID(binary.LittleEndian.Uint32(buf[0:4]))
	p.header.PageType = PageType(buf[4])
	p.header.LSN = binary.LittleEndian.Uint64(buf[5:13])
	p.header.NumSlots = binary.LittleEndian.Uint16(buf[13:15])
	p.header.FreeSpace = binary.LittleEndian.Uint16(buf[15:17])
	p.header.FreeSpacePtr = binary.LittleEndian.Uint16(buf[17:19])
	p.header.NextPage = PageID(binary.LittleEndian.Uint32(buf[19:23]))
	p.header.Checksum = binary.LittleEndian.Uint32(buf[28:32])

	// Verify checksum
	expectedChecksum := calculateChecksum(buf[:28], buf[32:])
	if p.header.Checksum != expectedChecksum {
		return fmt.Errorf("page checksum mismatch: expected %x, got %x", expectedChecksum, p.header.Checksum)
	}

	// Copy data
	copy(p.data[:], buf[PageHeaderSize:])

	return nil
}

// IsValid performs basic validation on the page.
func (p *Page) IsValid() error {
	// Validate page type
	if p.header.PageType > PageTypeOverflow {
		return fmt.Errorf("invalid page type: %d", p.header.PageType)
	}

	// Validate free space
	if p.header.FreeSpace > PageSize-PageHeaderSize {
		return fmt.Errorf("invalid free space: %d", p.header.FreeSpace)
	}

	// Validate free space pointer
	if p.header.FreeSpacePtr < PageHeaderSize || p.header.FreeSpacePtr > PageSize {
		return fmt.Errorf("invalid free space pointer: %d", p.header.FreeSpacePtr)
	}

	return nil
}

// Common errors
var (
	// ErrPageCorrupted indicates that a page is corrupted.
	ErrPageCorrupted = errors.New("page corrupted")
	// ErrInvalidPageID indicates an invalid page ID.
	ErrInvalidPageID = errors.New("invalid page ID")
	// ErrInvalidPageType indicates an invalid page type.
	ErrInvalidPageType = errors.New("invalid page type")
)
