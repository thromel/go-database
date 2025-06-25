package performance

import (
	"fmt"
	"testing"

	"github.com/romel/go-database/test/utils"
)

// BenchmarkDatabaseOperations benchmarks basic database operations.
func BenchmarkDatabaseOperations(b *testing.B) {
	benchmarks := []struct {
		name string
		fn   func(*testing.B, *testutils.BenchmarkHelper)
	}{
		{"Put/Single", benchmarkPutSingle},
		{"Put/Batch100", benchmarkPutBatch100},
		{"Put/Batch1000", benchmarkPutBatch1000},
		{"Get/Single", benchmarkGetSingle},
		{"Get/Random", benchmarkGetRandom},
		{"Delete/Single", benchmarkDeleteSingle},
		{"Exists/Single", benchmarkExistsSingle},
		{"Mixed/ReadWrite", benchmarkMixedReadWrite},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			helper := testutils.NewBenchmarkHelper(b)
			defer helper.Close()
			
			bm.fn(b, helper)
		})
	}
}

func benchmarkPutSingle(b *testing.B, helper *testutils.BenchmarkHelper) {
	key := []byte("benchmark-key")
	value := []byte("benchmark-value-with-some-reasonable-length-data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := helper.DB.Put(key, value)
		if err != nil {
			b.Fatalf("Put failed: %v", err)
		}
	}
}

func benchmarkPutBatch100(b *testing.B, helper *testutils.BenchmarkHelper) {
	keys := make([][]byte, 100)
	values := make([][]byte, 100)
	
	for i := 0; i < 100; i++ {
		keys[i] = []byte(fmt.Sprintf("batch-key-%03d", i))
		values[i] = []byte(fmt.Sprintf("batch-value-%03d-with-some-data", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			err := helper.DB.Put(keys[j], values[j])
			if err != nil {
				b.Fatalf("Put failed: %v", err)
			}
		}
	}
}

func benchmarkPutBatch1000(b *testing.B, helper *testutils.BenchmarkHelper) {
	keys := make([][]byte, 1000)
	values := make([][]byte, 1000)
	
	for i := 0; i < 1000; i++ {
		keys[i] = []byte(fmt.Sprintf("large-batch-key-%04d", i))
		values[i] = []byte(fmt.Sprintf("large-batch-value-%04d-with-more-substantial-data", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			err := helper.DB.Put(keys[j], values[j])
			if err != nil {
				b.Fatalf("Put failed: %v", err)
			}
		}
	}
}

func benchmarkGetSingle(b *testing.B, helper *testutils.BenchmarkHelper) {
	key := []byte("benchmark-key")
	value := []byte("benchmark-value-with-some-reasonable-length-data")

	// Setup
	err := helper.DB.Put(key, value)
	if err != nil {
		b.Fatalf("Setup Put failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := helper.DB.Get(key)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

func benchmarkGetRandom(b *testing.B, helper *testutils.BenchmarkHelper) {
	// Prepare test data
	const dataSize = 1000
	helper.PrepareData(dataSize)

	keys := make([][]byte, dataSize)
	for i := 0; i < dataSize; i++ {
		keys[i] = []byte(fmt.Sprintf("bench-key-%06d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		keyIndex := i % dataSize
		_, err := helper.DB.Get(keys[keyIndex])
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

func benchmarkDeleteSingle(b *testing.B, helper *testutils.BenchmarkHelper) {
	value := []byte("benchmark-value-with-some-reasonable-length-data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := []byte(fmt.Sprintf("delete-key-%d", i))
		
		// Setup
		err := helper.DB.Put(key, value)
		if err != nil {
			b.Fatalf("Setup Put failed: %v", err)
		}
		
		// Benchmark delete
		err = helper.DB.Delete(key)
		if err != nil {
			b.Fatalf("Delete failed: %v", err)
		}
	}
}

func benchmarkExistsSingle(b *testing.B, helper *testutils.BenchmarkHelper) {
	key := []byte("benchmark-key")
	value := []byte("benchmark-value-with-some-reasonable-length-data")

	// Setup
	err := helper.DB.Put(key, value)
	if err != nil {
		b.Fatalf("Setup Put failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := helper.DB.Exists(key)
		if err != nil {
			b.Fatalf("Exists failed: %v", err)
		}
	}
}

func benchmarkMixedReadWrite(b *testing.B, helper *testutils.BenchmarkHelper) {
	// Prepare initial data
	const initialData = 100
	helper.PrepareData(initialData)

	keys := make([][]byte, initialData)
	for i := 0; i < initialData; i++ {
		keys[i] = []byte(fmt.Sprintf("bench-key-%06d", i))
	}

	value := []byte("mixed-benchmark-value-with-reasonable-length")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		keyIndex := i % initialData
		key := keys[keyIndex]

		switch i % 4 {
		case 0: // 25% reads
			_, err := helper.DB.Get(key)
			if err != nil {
				b.Fatalf("Get failed: %v", err)
			}
		case 1: // 25% writes
			err := helper.DB.Put(key, value)
			if err != nil {
				b.Fatalf("Put failed: %v", err)
			}
		case 2: // 25% exists checks
			_, err := helper.DB.Exists(key)
			if err != nil {
				b.Fatalf("Exists failed: %v", err)
			}
		case 3: // 25% new writes
			newKey := []byte(fmt.Sprintf("new-key-%d", i))
			err := helper.DB.Put(newKey, value)
			if err != nil {
				b.Fatalf("Put new failed: %v", err)
			}
		}
	}
}

// BenchmarkConcurrentOperations benchmarks concurrent database operations.
func BenchmarkConcurrentOperations(b *testing.B) {
	helper := testutils.NewBenchmarkHelper(b)
	defer helper.Close()

	const numGoroutines = 10
	value := []byte("concurrent-benchmark-value")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := []byte(fmt.Sprintf("concurrent-key-%d", b.N))
			
			err := helper.DB.Put(key, value)
			if err != nil {
				b.Fatalf("Concurrent Put failed: %v", err)
			}
			
			_, err = helper.DB.Get(key)
			if err != nil {
				b.Fatalf("Concurrent Get failed: %v", err)
			}
		}
	})
}

// BenchmarkMemoryUsage benchmarks memory usage patterns.
func BenchmarkMemoryUsage(b *testing.B) {
	benchmarks := []struct {
		name      string
		keySize   int
		valueSize int
		count     int
	}{
		{"SmallKV", 16, 64, 1000},
		{"MediumKV", 32, 256, 1000},
		{"LargeKV", 64, 1024, 1000},
		{"VeryLargeKV", 128, 4096, 100},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			helper := testutils.NewBenchmarkHelper(b)
			defer helper.Close()

			generator := testutils.NewTestDataGenerator(42)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j := 0; j < bm.count; j++ {
					key := []byte(generator.GenerateKey(bm.keySize))
					value := []byte(generator.GenerateValue(bm.valueSize))
					
					err := helper.DB.Put(key, value)
					if err != nil {
						b.Fatalf("Put failed: %v", err)
					}
				}
			}
		})
	}
}