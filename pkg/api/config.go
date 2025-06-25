package api

import (
	"errors"
	"time"
)

// Config holds the configuration parameters for the database engine.
type Config struct {
	// Database file path (required for persistent storage)
	Path string

	// ReadOnly opens the database in read-only mode
	ReadOnly bool

	// Memory configuration
	Memory MemoryConfig

	// Storage configuration
	Storage StorageConfig

	// Transaction configuration
	Transaction TransactionConfig

	// Performance configuration
	Performance PerformanceConfig

	// Logging configuration
	Logging LoggingConfig
}

// MemoryConfig configures memory usage parameters.
type MemoryConfig struct {
	// BufferPoolSize is the size of the buffer pool in bytes (default: 64MB)
	BufferPoolSize int64

	// MaxMemoryUsage is the maximum memory usage in bytes (0 = unlimited)
	MaxMemoryUsage int64

	// CacheSize is the size of the query result cache in bytes (default: 16MB)
	CacheSize int64

	// EnableMemoryProfiling enables memory usage profiling
	EnableMemoryProfiling bool
}

// StorageConfig configures storage engine parameters.
type StorageConfig struct {
	// PageSize is the size of each page in bytes (default: 8KB)
	PageSize int

	// MaxFileSize is the maximum database file size in bytes (0 = unlimited)
	MaxFileSize int64

	// SyncWrites ensures all writes are synced to disk immediately
	SyncWrites bool

	// CompressionEnabled enables data compression
	CompressionEnabled bool

	// ChecksumEnabled enables page checksums for corruption detection
	ChecksumEnabled bool

	// BackupEnabled enables automatic backups
	BackupEnabled bool

	// BackupInterval is the interval between automatic backups
	BackupInterval time.Duration
}

// TransactionConfig configures transaction behavior.
type TransactionConfig struct {
	// DefaultIsolationLevel is the default isolation level for transactions
	DefaultIsolationLevel string

	// MaxActiveTransactions is the maximum number of concurrent transactions
	MaxActiveTransactions int

	// TransactionTimeout is the default timeout for transactions
	TransactionTimeout time.Duration

	// DeadlockDetectionEnabled enables automatic deadlock detection
	DeadlockDetectionEnabled bool

	// DeadlockDetectionInterval is how often to check for deadlocks
	DeadlockDetectionInterval time.Duration

	// RetryPolicy configures automatic retry behavior
	RetryPolicy RetryPolicyConfig
}

// RetryPolicyConfig configures automatic retry behavior for transactions.
type RetryPolicyConfig struct {
	// Enabled enables automatic retry on conflicts
	Enabled bool

	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// InitialDelay is the initial delay between retry attempts
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retry attempts
	MaxDelay time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64
}

// PerformanceConfig configures performance-related settings.
type PerformanceConfig struct {
	// MaxConcurrentReads is the maximum number of concurrent read operations
	MaxConcurrentReads int

	// MaxConcurrentWrites is the maximum number of concurrent write operations
	MaxConcurrentWrites int

	// QueryTimeout is the default timeout for queries
	QueryTimeout time.Duration

	// IndexCacheSize is the size of the index cache in bytes
	IndexCacheSize int64

	// StatisticsEnabled enables query statistics collection
	StatisticsEnabled bool

	// StatisticsInterval is how often to update statistics
	StatisticsInterval time.Duration
}

// LoggingConfig configures logging behavior.
type LoggingConfig struct {
	// Level is the logging level (DEBUG, INFO, WARN, ERROR)
	Level string

	// File is the log file path (empty = stdout)
	File string

	// MaxSize is the maximum log file size in MB
	MaxSize int

	// MaxBackups is the maximum number of log file backups to keep
	MaxBackups int

	// MaxAge is the maximum age of log files in days
	MaxAge int

	// Compress enables compression of rotated log files
	Compress bool

	// EnableQueryLogging enables logging of all queries
	EnableQueryLogging bool

	// EnableSlowQueryLogging enables logging of slow queries
	EnableSlowQueryLogging bool

	// SlowQueryThreshold is the threshold for slow query logging
	SlowQueryThreshold time.Duration
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ReadOnly: false,
		Memory: MemoryConfig{
			BufferPoolSize:        64 * 1024 * 1024, // 64MB
			MaxMemoryUsage:        0,                // unlimited
			CacheSize:             16 * 1024 * 1024, // 16MB
			EnableMemoryProfiling: false,
		},
		Storage: StorageConfig{
			PageSize:           8192, // 8KB
			MaxFileSize:        0,    // unlimited
			SyncWrites:         false,
			CompressionEnabled: false,
			ChecksumEnabled:    true,
			BackupEnabled:      false,
			BackupInterval:     24 * time.Hour,
		},
		Transaction: TransactionConfig{
			DefaultIsolationLevel:     "READ_COMMITTED",
			MaxActiveTransactions:     1000,
			TransactionTimeout:        30 * time.Second,
			DeadlockDetectionEnabled:  true,
			DeadlockDetectionInterval: 1 * time.Second,
			RetryPolicy: RetryPolicyConfig{
				Enabled:           true,
				MaxRetries:        3,
				InitialDelay:      10 * time.Millisecond,
				MaxDelay:          1 * time.Second,
				BackoffMultiplier: 2.0,
			},
		},
		Performance: PerformanceConfig{
			MaxConcurrentReads:  100,
			MaxConcurrentWrites: 50,
			QueryTimeout:        30 * time.Second,
			IndexCacheSize:      32 * 1024 * 1024, // 32MB
			StatisticsEnabled:   true,
			StatisticsInterval:  5 * time.Minute,
		},
		Logging: LoggingConfig{
			Level:                  "INFO",
			File:                   "",
			MaxSize:                100, // 100MB
			MaxBackups:             3,
			MaxAge:                 28, // 28 days
			Compress:               true,
			EnableQueryLogging:     false,
			EnableSlowQueryLogging: true,
			SlowQueryThreshold:     1 * time.Second,
		},
	}
}

// Validate checks if the configuration is valid and returns an error if not.
func (c *Config) Validate() error {
	// Path is required for persistent storage
	if c.Path == "" {
		return ErrConfigPathRequired
	}

	// Memory configuration validation
	if c.Memory.BufferPoolSize <= 0 {
		return ErrInvalidBufferPoolSize
	}

	// Storage configuration validation
	if c.Storage.PageSize <= 0 || c.Storage.PageSize > 65536 {
		return ErrInvalidPageSize
	}

	// Transaction configuration validation
	if c.Transaction.MaxActiveTransactions <= 0 {
		return ErrInvalidMaxActiveTransactions
	}

	if c.Transaction.TransactionTimeout <= 0 {
		return ErrInvalidTransactionTimeout
	}

	// Performance configuration validation
	if c.Performance.MaxConcurrentReads <= 0 {
		return ErrInvalidMaxConcurrentReads
	}

	if c.Performance.MaxConcurrentWrites <= 0 {
		return ErrInvalidMaxConcurrentWrites
	}

	return nil
}

// Configuration validation errors
var (
	ErrConfigPathRequired           = errors.New("config: path is required")
	ErrInvalidBufferPoolSize        = errors.New("config: buffer pool size must be positive")
	ErrInvalidPageSize              = errors.New("config: page size must be between 1 and 65536 bytes")
	ErrInvalidMaxActiveTransactions = errors.New("config: max active transactions must be positive")
	ErrInvalidTransactionTimeout    = errors.New("config: transaction timeout must be positive")
	ErrInvalidMaxConcurrentReads    = errors.New("config: max concurrent reads must be positive")
	ErrInvalidMaxConcurrentWrites   = errors.New("config: max concurrent writes must be positive")
)
