package page

import (
	"bytes"
	"testing"
)

func TestPageCreation(t *testing.T) {
	tests := []struct {
		name     string
		pageID   PageID
		pageType PageType
	}{
		{"Leaf Page", 1, PageTypeLeaf},
		{"Internal Page", 2, PageTypeInternal},
		{"Meta Page", 0, PageTypeMeta},
		{"Free Page", 3, PageTypeFree},
		{"Overflow Page", 4, PageTypeOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := NewPage(tt.pageID, tt.pageType)

			if page.ID() != tt.pageID {
				t.Errorf("expected page ID %d, got %d", tt.pageID, page.ID())
			}

			if page.Type() != tt.pageType {
				t.Errorf("expected page type %v, got %v", tt.pageType, page.Type())
			}

			if page.FreeSpace() != PageSize-PageHeaderSize {
				t.Errorf("expected free space %d, got %d", PageSize-PageHeaderSize, page.FreeSpace())
			}

			if page.NumSlots() != 0 {
				t.Errorf("expected 0 slots, got %d", page.NumSlots())
			}

			if page.NextPage() != InvalidPageID {
				t.Errorf("expected next page %d, got %d", InvalidPageID, page.NextPage())
			}
		})
	}
}

func TestPageSerialization(t *testing.T) {
	// Create a test page
	page := NewPage(42, PageTypeLeaf)
	page.SetLSN(12345)
	page.SetNextPage(43)

	// Write some test data
	testData := []byte("Hello, Database!")
	copy(page.Data(), testData)

	// Serialize the page
	serialized, err := page.Serialize()
	if err != nil {
		t.Fatalf("failed to serialize page: %v", err)
	}

	if len(serialized) != PageSize {
		t.Errorf("expected serialized size %d, got %d", PageSize, len(serialized))
	}

	// Deserialize into a new page
	newPage := &Page{}
	err = newPage.Deserialize(serialized)
	if err != nil {
		t.Fatalf("failed to deserialize page: %v", err)
	}

	// Verify the deserialized page matches
	if newPage.ID() != page.ID() {
		t.Errorf("page ID mismatch: expected %d, got %d", page.ID(), newPage.ID())
	}

	if newPage.Type() != page.Type() {
		t.Errorf("page type mismatch: expected %v, got %v", page.Type(), newPage.Type())
	}

	if newPage.LSN() != page.LSN() {
		t.Errorf("LSN mismatch: expected %d, got %d", page.LSN(), newPage.LSN())
	}

	if newPage.NextPage() != page.NextPage() {
		t.Errorf("next page mismatch: expected %d, got %d", page.NextPage(), newPage.NextPage())
	}

	// Verify data matches
	if !bytes.Equal(newPage.Data()[:len(testData)], testData) {
		t.Errorf("data mismatch: expected %v, got %v", testData, newPage.Data()[:len(testData)])
	}
}

func TestPageChecksumValidation(t *testing.T) {
	// Create and serialize a page
	page := NewPage(1, PageTypeLeaf)
	serialized, err := page.Serialize()
	if err != nil {
		t.Fatalf("failed to serialize page: %v", err)
	}

	// Verify checksum is valid
	err = VerifyChecksum(serialized)
	if err != nil {
		t.Errorf("checksum validation failed for valid page: %v", err)
	}

	// Corrupt the data
	serialized[100] ^= 0xFF

	// Verify checksum detects corruption
	err = VerifyChecksum(serialized)
	if err == nil {
		t.Error("checksum validation should have failed for corrupted page")
	}
}

func TestPageValidation(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Page)
		wantError bool
	}{
		{
			name:      "Valid Page",
			setup:     func(p *Page) {},
			wantError: false,
		},
		{
			name: "Invalid Page Type",
			setup: func(p *Page) {
				p.header.PageType = 99
			},
			wantError: true,
		},
		{
			name: "Invalid Free Space",
			setup: func(p *Page) {
				p.header.FreeSpace = PageSize
			},
			wantError: true,
		},
		{
			name: "Invalid Free Space Pointer - Too Low",
			setup: func(p *Page) {
				p.header.FreeSpacePtr = 0
			},
			wantError: true,
		},
		{
			name: "Invalid Free Space Pointer - Too High",
			setup: func(p *Page) {
				p.header.FreeSpacePtr = PageSize + 1
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := NewPage(1, PageTypeLeaf)
			tt.setup(page)

			err := page.IsValid()
			if (err != nil) != tt.wantError {
				t.Errorf("IsValid() error = %v, wantError = %v", err, tt.wantError)
			}
		})
	}
}

func TestPageTypeString(t *testing.T) {
	tests := []struct {
		pageType PageType
		expected string
	}{
		{PageTypeLeaf, "LEAF"},
		{PageTypeInternal, "INTERNAL"},
		{PageTypeMeta, "META"},
		{PageTypeFree, "FREE"},
		{PageTypeOverflow, "OVERFLOW"},
		{PageType(99), "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.pageType.String()
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func BenchmarkPageSerialization(b *testing.B) {
	page := NewPage(1, PageTypeLeaf)
	testData := bytes.Repeat([]byte("data"), 1000)
	copy(page.Data(), testData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := page.Serialize()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPageDeserialization(b *testing.B) {
	page := NewPage(1, PageTypeLeaf)
	serialized, _ := page.Serialize()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newPage := &Page{}
		err := newPage.Deserialize(serialized)
		if err != nil {
			b.Fatal(err)
		}
	}
}
