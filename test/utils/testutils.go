// Package testutils provides testing utilities and helper functions
// for the Go Database Engine test suite, including test database setup,
// data generation, and common test patterns.
package testutils

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thromel/go-database/pkg/api"
)

// TestDatabase provides utilities for testing database operations.
type TestDatabase struct {
	DB   api.Database
	Path string
	t    *testing.T
}

// NewTestDatabase creates a new test database instance.
func NewTestDatabase(t *testing.T) *TestDatabase {
	// Generate secure random suffix
	var suffix [8]byte
	_, err := rand.Read(suffix[:])
	if err != nil {
		t.Fatalf("Failed to generate random suffix: %v", err)
	}

	path := filepath.Join(os.TempDir(), fmt.Sprintf("test-db-%d-%x", time.Now().UnixNano(), suffix))

	config := api.DefaultConfig()
	config.Path = path

	db, err := api.Open(path, config)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	return &TestDatabase{
		DB:   db,
		Path: path,
		t:    t,
	}
}

// Close closes the test database and cleans up resources.
func (td *TestDatabase) Close() {
	if td.DB != nil {
		err := td.DB.Close()
		if err != nil && err.Error() != "database is closed" {
			td.t.Errorf("Failed to close test database: %v", err)
		}
	}

	// Clean up test file if it exists
	if td.Path != "" {
		_ = os.Remove(td.Path) // Ignore error on cleanup
	}
}

// AssertKeyExists verifies that a key exists in the database.
func (td *TestDatabase) AssertKeyExists(key []byte) {
	exists, err := td.DB.Exists(key)
	if err != nil {
		td.t.Fatalf("Failed to check key existence: %v", err)
	}
	if !exists {
		td.t.Errorf("Expected key %x to exist", key)
	}
}

// AssertKeyNotExists verifies that a key does not exist in the database.
func (td *TestDatabase) AssertKeyNotExists(key []byte) {
	exists, err := td.DB.Exists(key)
	if err != nil {
		td.t.Fatalf("Failed to check key existence: %v", err)
	}
	if exists {
		td.t.Errorf("Expected key %x to not exist", key)
	}
}

// AssertKeyValue verifies that a key has the expected value.
func (td *TestDatabase) AssertKeyValue(key, expectedValue []byte) {
	value, err := td.DB.Get(key)
	if err != nil {
		td.t.Fatalf("Failed to get key %x: %v", key, err)
	}
	if !bytes.Equal(value, expectedValue) {
		td.t.Errorf("Expected key %x to have value %x, got %x", key, expectedValue, value)
	}
}

// PutTestData adds test data to the database.
func (td *TestDatabase) PutTestData(data map[string]string) {
	for k, v := range data {
		err := td.DB.Put([]byte(k), []byte(v))
		if err != nil {
			td.t.Fatalf("Failed to put test data %s: %v", k, err)
		}
	}
}

// TestDataGenerator provides utilities for generating test data.
type TestDataGenerator struct {
	seed int64
}

// NewTestDataGenerator creates a new test data generator.
func NewTestDataGenerator(seed int64) *TestDataGenerator {
	return &TestDataGenerator{
		seed: seed,
	}
}

// GenerateKeyValuePairs generates random key-value pairs.
func (g *TestDataGenerator) GenerateKeyValuePairs(count int) map[string]string {
	data := make(map[string]string)

	for i := 0; i < count; i++ {
		key := g.GenerateKey(16)
		value := g.GenerateValue(64)
		data[key] = value
	}

	return data
}

// GenerateKey generates a deterministic key of the specified length using seed.
func (g *TestDataGenerator) GenerateKey(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)

	// Use crypto/rand for secure generation
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to deterministic generation based on seed for tests
		for i := range result {
			result[i] = charset[(g.seed+int64(i))%int64(len(charset))]
		}
	} else {
		for i := range result {
			result[i] = charset[randomBytes[i]%byte(len(charset))]
		}
	}

	return string(result)
}

// GenerateValue generates a deterministic value of the specified length using seed.
func (g *TestDataGenerator) GenerateValue(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 !@#$%^&*()_+-="
	result := make([]byte, length)

	// Use crypto/rand for secure generation
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to deterministic generation based on seed for tests
		for i := range result {
			result[i] = charset[(g.seed+int64(i)+100)%int64(len(charset))]
		}
	} else {
		for i := range result {
			result[i] = charset[randomBytes[i]%byte(len(charset))]
		}
	}

	return string(result)
}

// GenerateSequentialKeys generates sequential keys for testing ordering.
func (g *TestDataGenerator) GenerateSequentialKeys(count int, prefix string) []string {
	keys := make([]string, count)

	for i := 0; i < count; i++ {
		keys[i] = fmt.Sprintf("%s%06d", prefix, i)
	}

	return keys
}

// BenchmarkHelper provides utilities for benchmark tests.
type BenchmarkHelper struct {
	DB api.Database
}

// NewBenchmarkHelper creates a new benchmark helper.
func NewBenchmarkHelper(b *testing.B) *BenchmarkHelper {
	// Generate secure random suffix
	var suffix [8]byte
	_, err := rand.Read(suffix[:])
	if err != nil {
		b.Fatalf("Failed to generate random suffix: %v", err)
	}

	path := filepath.Join(os.TempDir(), fmt.Sprintf("bench-db-%d-%x", time.Now().UnixNano(), suffix))

	config := api.DefaultConfig()
	config.Path = path

	db, err := api.Open(path, config)
	if err != nil {
		b.Fatalf("Failed to create benchmark database: %v", err)
	}

	return &BenchmarkHelper{
		DB: db,
	}
}

// Close closes the benchmark database.
func (bh *BenchmarkHelper) Close() {
	if bh.DB != nil {
		_ = bh.DB.Close() // Ignore error on cleanup
	}
}

// PrepareData prepares test data for benchmarks.
func (bh *BenchmarkHelper) PrepareData(count int) {
	generator := NewTestDataGenerator(42) // Fixed seed for reproducible benchmarks

	for i := 0; i < count; i++ {
		key := []byte(fmt.Sprintf("bench-key-%06d", i))
		value := []byte(generator.GenerateValue(100))

		err := bh.DB.Put(key, value)
		if err != nil {
			panic(fmt.Sprintf("Failed to prepare benchmark data: %v", err))
		}
	}
}

// AssertNoError is a helper to assert that an error is nil.
func AssertNoError(t *testing.T, err error, message string) {
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}

// AssertError is a helper to assert that an error is not nil.
func AssertError(t *testing.T, err error, message string) {
	if err == nil {
		t.Fatalf("%s: expected error but got nil", message)
	}
}

// AssertEqual is a helper to assert that two values are equal.
func AssertEqual(t *testing.T, expected, actual any, message string) {
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", message, expected, actual)
	}
}

// IsKeyNotFoundError checks if an error indicates a key was not found.
func IsKeyNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr == "key not found" ||
		errStr == "database error in get (key: key not found" ||
		errStr == "database error in delete (key: key not found" ||
		bytes.Contains([]byte(errStr), []byte("key not found"))
}

// AssertBytesEqual is a helper to assert that two byte slices are equal.
func AssertBytesEqual(t *testing.T, expected, actual []byte, message string) {
	if !bytes.Equal(expected, actual) {
		t.Errorf("%s: expected %x, got %x", message, expected, actual)
	}
}

// AssertTrue is a helper to assert that a condition is true.
func AssertTrue(t *testing.T, condition bool, message string) {
	if !condition {
		t.Errorf("%s: expected true but got false", message)
	}
}

// AssertFalse is a helper to assert that a condition is false.
func AssertFalse(t *testing.T, condition bool, message string) {
	if condition {
		t.Errorf("%s: expected false but got true", message)
	}
}

// MeasureTime measures the execution time of a function.
func MeasureTime(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// RunConcurrent runs multiple functions concurrently and waits for completion.
func RunConcurrent(fns ...func()) {
	done := make(chan bool, len(fns))

	for _, fn := range fns {
		go func(f func()) {
			defer func() { done <- true }()
			f()
		}(fn)
	}

	for i := 0; i < len(fns); i++ {
		<-done
	}
}
