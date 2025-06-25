package integration

import (
	"testing"
	"time"

	"github.com/romel/go-database/pkg/api"
	testutils "github.com/romel/go-database/test/utils"
)

// TestDatabaseLifecycle tests the complete database lifecycle.
func TestDatabaseLifecycle(t *testing.T) {
	testDB := testutils.NewTestDatabase(t)
	defer testDB.Close()

	// Test initial state
	stats, err := testDB.DB.Stats()
	testutils.AssertNoError(t, err, "Getting initial stats")
	testutils.AssertEqual(t, int64(0), stats.KeyCount, "Initial key count")

	// Test data operations
	testData := map[string]string{
		"user:1":       "john@example.com",
		"user:2":       "jane@example.com",
		"config:db":    "localhost:5432",
		"config:cache": "redis://localhost:6379",
	}

	testDB.PutTestData(testData)

	// Verify all data was stored
	for k, v := range testData {
		testDB.AssertKeyValue([]byte(k), []byte(v))
	}

	// Test stats after adding data
	stats, err = testDB.DB.Stats()
	testutils.AssertNoError(t, err, "Getting stats after adding data")
	testutils.AssertEqual(t, int64(len(testData)), stats.KeyCount, "Key count after adding data")

	// Test deletion
	err = testDB.DB.Delete([]byte("user:1"))
	testutils.AssertNoError(t, err, "Deleting user:1")
	testDB.AssertKeyNotExists([]byte("user:1"))

	// Test remaining data
	testDB.AssertKeyExists([]byte("user:2"))
	testDB.AssertKeyValue([]byte("user:2"), []byte("jane@example.com"))
}

// TestConcurrentDatabaseAccess tests concurrent access to the database.
func TestConcurrentDatabaseAccess(t *testing.T) {
	testDB := testutils.NewTestDatabase(t)
	defer testDB.Close()

	const numGoroutines = 10
	const operationsPerGoroutine = 100

	// Run concurrent operations
	testutils.RunConcurrent(
		// Writer goroutines
		func() {
			for i := 0; i < operationsPerGoroutine; i++ {
				key := []byte("writer1-key-" + string(rune('0'+i%10)))
				value := []byte("writer1-value-" + string(rune('0'+i%10)))
				err := testDB.DB.Put(key, value)
				if err != nil {
					t.Errorf("Writer1 Put failed: %v", err)
				}
			}
		},
		func() {
			for i := 0; i < operationsPerGoroutine; i++ {
				key := []byte("writer2-key-" + string(rune('0'+i%10)))
				value := []byte("writer2-value-" + string(rune('0'+i%10)))
				err := testDB.DB.Put(key, value)
				if err != nil {
					t.Errorf("Writer2 Put failed: %v", err)
				}
			}
		},
		// Reader goroutines
		func() {
			for i := 0; i < operationsPerGoroutine; i++ {
				key := []byte("writer1-key-" + string(rune('0'+i%10)))
				_, err := testDB.DB.Get(key)
				// Key might not exist yet due to concurrency, so we only check for serious errors
				if err != nil && !testutils.IsKeyNotFoundError(err) {
					t.Errorf("Reader1 Get failed: %v", err)
				}
			}
		},
		func() {
			for i := 0; i < operationsPerGoroutine; i++ {
				key := []byte("writer2-key-" + string(rune('0'+i%10)))
				_, err := testDB.DB.Get(key)
				// Key might not exist yet due to concurrency, so we only check for serious errors
				if err != nil && !testutils.IsKeyNotFoundError(err) {
					t.Errorf("Reader2 Get failed: %v", err)
				}
			}
		},
	)

	// Verify final state
	stats, err := testDB.DB.Stats()
	testutils.AssertNoError(t, err, "Getting final stats")

	// Should have some data (exact count may vary due to key overwrites)
	testutils.AssertTrue(t, stats.KeyCount > 0, "Should have some keys after concurrent operations")
}

// TestLargeDataOperations tests operations with large amounts of data.
func TestLargeDataOperations(t *testing.T) {
	testDB := testutils.NewTestDatabase(t)
	defer testDB.Close()

	generator := testutils.NewTestDataGenerator(42)

	// Test with 1000 key-value pairs
	const dataSize = 1000
	testData := generator.GenerateKeyValuePairs(dataSize)

	// Measure insertion time
	insertTime := testutils.MeasureTime(func() {
		testDB.PutTestData(testData)
	})

	t.Logf("Inserted %d key-value pairs in %v", dataSize, insertTime)

	// Verify data integrity
	verifyTime := testutils.MeasureTime(func() {
		for k, v := range testData {
			testDB.AssertKeyValue([]byte(k), []byte(v))
		}
	})

	t.Logf("Verified %d key-value pairs in %v", dataSize, verifyTime)

	// Test stats
	stats, err := testDB.DB.Stats()
	testutils.AssertNoError(t, err, "Getting stats after large data operations")
	testutils.AssertEqual(t, int64(dataSize), stats.KeyCount, "Key count after large data operations")
}

// TestDatabaseConfiguration tests different database configurations.
func TestDatabaseConfiguration(t *testing.T) {
	configs := []struct {
		name   string
		config *api.Config
	}{
		{
			name:   "Default",
			config: api.DefaultConfig(),
		},
		{
			name: "ReadOnly",
			config: &api.Config{
				Path:     "readonly-test.db",
				ReadOnly: true,
				Memory: api.MemoryConfig{
					BufferPoolSize: 32 * 1024 * 1024, // 32MB
				},
				Storage: api.StorageConfig{
					PageSize: 8192, // Required field
				},
				Transaction: api.TransactionConfig{
					MaxActiveTransactions: 1000, // Required field
					TransactionTimeout:    30 * time.Second,
				},
				Performance: api.PerformanceConfig{
					MaxConcurrentReads:  100, // Required field
					MaxConcurrentWrites: 50,  // Required field
				},
			},
		},
		{
			name: "LargeBuffer",
			config: &api.Config{
				Path: "large-buffer-test.db",
				Memory: api.MemoryConfig{
					BufferPoolSize: 128 * 1024 * 1024, // 128MB
				},
				Storage: api.StorageConfig{
					PageSize: 8192, // Required field
				},
				Transaction: api.TransactionConfig{
					MaxActiveTransactions: 1000, // Required field
					TransactionTimeout:    30 * time.Second,
				},
				Performance: api.PerformanceConfig{
					MaxConcurrentReads:  100, // Required field
					MaxConcurrentWrites: 50,  // Required field
				},
			},
		},
	}

	for _, tc := range configs {
		t.Run(tc.name, func(t *testing.T) {
			tc.config.Path = "config-test-" + tc.name + ".db"

			db, err := api.Open(tc.config.Path, tc.config)
			testutils.AssertNoError(t, err, "Opening database with "+tc.name+" config")
			defer db.Close()

			// Basic operations should work (except for read-only)
			if !tc.config.ReadOnly {
				err = db.Put([]byte("test"), []byte("value"))
				testutils.AssertNoError(t, err, "Put operation with "+tc.name+" config")

				value, err := db.Get([]byte("test"))
				testutils.AssertNoError(t, err, "Get operation with "+tc.name+" config")
				testutils.AssertBytesEqual(t, []byte("value"), value, "Value verification with "+tc.name+" config")
			}
		})
	}
}

// TestErrorHandling tests various error conditions.
func TestErrorHandling(t *testing.T) {
	testDB := testutils.NewTestDatabase(t)
	defer testDB.Close()

	// Test operations with invalid keys
	_, err := testDB.DB.Get(nil)
	testutils.AssertError(t, err, "Get with nil key should fail")

	_, err = testDB.DB.Get([]byte{})
	testutils.AssertError(t, err, "Get with empty key should fail")

	// Test operations with invalid values
	err = testDB.DB.Put([]byte("key"), nil)
	testutils.AssertError(t, err, "Put with nil value should fail")

	// Test operations on non-existent keys
	_, err = testDB.DB.Get([]byte("non-existent"))
	testutils.AssertError(t, err, "Get non-existent key should fail")

	err = testDB.DB.Delete([]byte("non-existent"))
	testutils.AssertError(t, err, "Delete non-existent key should fail")

	// Test operations after close
	err = testDB.DB.Close()
	if err != nil {
		t.Logf("First close returned error: %v", err)
	}

	_, err = testDB.DB.Get([]byte("key"))
	testutils.AssertError(t, err, "Get after close should fail")

	err = testDB.DB.Put([]byte("key"), []byte("value"))
	testutils.AssertError(t, err, "Put after close should fail")
}
