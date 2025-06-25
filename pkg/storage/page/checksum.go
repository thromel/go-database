package page

import (
	"encoding/binary"
	"hash/crc32"
)

// crc32Table is the CRC32 polynomial table for checksum calculation.
var crc32Table = crc32.MakeTable(crc32.IEEE)

// calculateChecksum computes the CRC32 checksum for page data.
// It takes two byte slices to handle the case where we need to skip
// the checksum field itself in the page header.
func calculateChecksum(data1, data2 []byte) uint32 {
	h := crc32.New(crc32Table)
	h.Write(data1)
	h.Write(data2)
	return h.Sum32()
}

// VerifyChecksum verifies the checksum of serialized page data.
func VerifyChecksum(pageData []byte) error {
	if len(pageData) != PageSize {
		return ErrPageCorrupted
	}
	
	// Extract stored checksum
	storedChecksum := binary.LittleEndian.Uint32(pageData[28:32])
	
	// Calculate expected checksum (excluding checksum field)
	expectedChecksum := calculateChecksum(pageData[:28], pageData[32:])
	
	if storedChecksum != expectedChecksum {
		return ErrPageCorrupted
	}
	
	return nil
}