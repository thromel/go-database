# Go Database Engine Architecture Document

## Table of Contents

1. [System Overview](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#system-overview)
2. [Core Architecture Principles](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#core-architecture-principles)
3. [System Components](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#system-components)
4. [Data Flow Architecture](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#data-flow-architecture)
5. [Storage Engine Design](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#storage-engine-design)
6. [Transaction Architecture](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#transaction-architecture)
7. [Query Processing Architecture](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#query-processing-architecture)
8. [Concurrency Model](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#concurrency-model)
9. [Recovery System Design](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#recovery-system-design)
10. [Performance Considerations](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#performance-considerations)
11. [Security Architecture](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#security-architecture)
12. [Extensibility Framework](https://claude.ai/chat/43dec620-85ab-4bfd-9f45-1e02edacd013#extensibility-framework)

## 1. System Overview

### 1.1 Purpose and Vision

This document describes the architecture of a custom database engine built in Go, designed to serve as both an educational platform for understanding database internals and a production-ready embedded database solution. The system aims to provide a unique combination of traditional database reliability with modern innovations such as adaptive indexing and machine learning-driven optimization.

### 1.2 Key Design Goals

- **ACID Compliance**: Full transactional guarantees with configurable isolation levels
- **Performance**: Optimized for modern hardware (NVMe SSDs, multi-core CPUs)
- **Simplicity**: Easy to embed and use, similar to SQLite but native to Go
- **Innovation**: Incorporating cutting-edge features like adaptive indexing and time-travel queries
- **Extensibility**: Modular design allowing easy addition of new storage engines and features

### 1.3 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Layer                              │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐         │
│  │ Embedded API│  │ SQL Interface │  │ Wire Protocol  │         │
│  └─────────────┘  └──────────────┘  └────────────────┘         │
├─────────────────────────────────────────────────────────────────┤
│                    Query Processing Layer                         │
│  ┌─────────┐  ┌─────────┐  ┌───────────┐  ┌──────────┐        │
│  │ Parser  │→ │ Analyzer│→ │ Optimizer │→ │ Executor │        │
│  └─────────┘  └─────────┘  └───────────┘  └──────────┘        │
├─────────────────────────────────────────────────────────────────┤
│                    Transaction Manager                            │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐          │
│  │ Lock Manager │  │ MVCC Manager │  │ Log Manager │          │
│  └──────────────┘  └──────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────────┤
│                      Storage Engine                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │Index Manager │  │ Buffer Pool  │  │ Page Manager │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
├─────────────────────────────────────────────────────────────────┤
│                     Persistence Layer                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │ File Manager │  │  WAL Writer  │  │ Checkpointer │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
└─────────────────────────────────────────────────────────────────┘
```

## 2. Core Architecture Principles

### 2.1 Layered Architecture

The system follows a strict layered architecture where each layer only communicates with adjacent layers. This design ensures modularity and makes it easier to swap implementations at any layer without affecting others.

### 2.2 Interface-Driven Design

Every major component is defined by Go interfaces, allowing for multiple implementations and easy testing. For example:

```go
// StorageEngine defines the contract for any storage implementation
type StorageEngine interface {
    Get(key []byte) ([]byte, error)
    Put(key []byte, value []byte) error
    Delete(key []byte) error
    NewIterator(start, end []byte) Iterator
}
```

### 2.3 Concurrency-First Design

Given Go's strength in concurrent programming, the architecture embraces parallelism at multiple levels:

- Concurrent query execution
- Parallel I/O operations
- Background maintenance tasks
- Lock-free read paths where possible

### 2.4 Failure Resilience

Every component is designed with failure scenarios in mind:

- Write-ahead logging for durability
- Checksums at multiple levels
- Graceful degradation under resource pressure
- Comprehensive error propagation and handling

## 3. System Components

### 3.1 Client Layer

#### 3.1.1 Embedded API

The primary interface for Go applications to interact with the database:

```go
type Database struct {
    config   *Config
    storage  StorageEngine
    txnMgr   *TransactionManager
    queryProc *QueryProcessor
}

// Example usage
db, err := godb.Open("mydata.db", &Config{
    BufferPoolSize: 100 * 1024 * 1024, // 100MB
    EnableMVCC: true,
})

// Key-value operations
err = db.Put([]byte("key"), []byte("value"))

// Transaction operations
tx, err := db.Begin()
err = tx.Put([]byte("key1"), []byte("value1"))
err = tx.Put([]byte("key2"), []byte("value2"))
err = tx.Commit()

// SQL operations (Phase 2)
result, err := db.Query("SELECT * FROM users WHERE age > ?", 25)
```

#### 3.1.2 SQL Interface

A SQL compatibility layer that translates SQL queries into internal operations:

```go
type SQLEngine struct {
    parser    *SQLParser
    analyzer  *SemanticAnalyzer
    optimizer *QueryOptimizer
    executor  *QueryExecutor
}
```

### 3.2 Query Processing Layer

#### 3.2.1 Parser

Converts query strings into Abstract Syntax Trees (AST):

```go
type ASTNode interface {
    Type() NodeType
    String() string
}

type SelectStatement struct {
    Columns    []Expression
    From       TableReference
    Where      Expression
    GroupBy    []Expression
    Having     Expression
    OrderBy    []OrderByClause
    Limit      *int
}
```

#### 3.2.2 Query Analyzer

Performs semantic analysis and type checking:

- Validates table and column references
- Resolves data types
- Checks constraint violations
- Builds initial query plan

#### 3.2.3 Query Optimizer

Transforms logical plans into optimal physical plans:

```go
type QueryOptimizer struct {
    rules        []OptimizationRule
    costModel    CostModel
    statistics   *StatisticsManager
    adaptiveOpt  *AdaptiveOptimizer  // ML-enhanced optimization
}

type PhysicalPlan interface {
    Execute(ctx context.Context) (ResultSet, error)
    EstimateCost() Cost
    ExplainPlan() string
}
```

The optimizer implements both rule-based and cost-based optimization, with an optional machine learning component that learns from query execution history.

### 3.3 Storage Engine Design

#### 3.3.1 Page Management

The storage engine uses a page-based architecture with fixed-size pages:

```go
const PageSize = 8192  // 8KB pages

type Page struct {
    header PageHeader
    data   [PageSize - PageHeaderSize]byte
}

type PageHeader struct {
    PageID       PageID    // Unique page identifier
    PageType     PageType  // Leaf, Internal, Meta, Free
    LSN          uint64    // Log Sequence Number
    NumSlots     uint16    // Number of data slots
    FreeSpace    uint16    // Bytes of free space
    FreeSpacePtr uint16    // Offset to free space
    Checksum     uint32    // CRC32 checksum
}

type PageType uint8
const (
    PageTypeLeaf PageType = iota
    PageTypeInternal
    PageTypeMeta
    PageTypeFree
    PageTypeOverflow
)
```

#### 3.3.2 B+ Tree Implementation

The primary index structure is a B+ Tree optimized for range queries:

```go
type BPlusTree struct {
    root      PageID
    height    int
    numKeys   int64
    pageCache *PageCache
    
    // Concurrency control
    treeLatch sync.RWMutex  // Protects tree structure
}

type BPlusTreeNode struct {
    isLeaf   bool
    keys     [][]byte
    children []PageID  // For internal nodes
    values   [][]byte  // For leaf nodes
    next     PageID    // Next leaf (for range scans)
}
```

#### 3.3.3 Buffer Pool Manager

Manages pages in memory with sophisticated eviction policies:

```go
type BufferPool struct {
    frames    []Frame
    pageTable map[PageID]FrameID
    
    // Eviction policy
    evictionPolicy EvictionPolicy  // LRU, LRU-K, CLOCK
    
    // Concurrency
    poolMutex sync.Mutex
    
    // Statistics
    hits      atomic.Uint64
    misses    atomic.Uint64
}

type Frame struct {
    page      *Page
    pinCount  atomic.Int32
    dirty     atomic.Bool
    lastUsed  atomic.Int64
    pageLatch sync.RWMutex
}
```

### 3.4 Transaction Architecture

#### 3.4.1 Transaction Manager

Coordinates all aspects of transaction processing:

```go
type TransactionManager struct {
    nextTxnID  atomic.Uint64
    activeTxns map[TxnID]*Transaction
    lockMgr    *LockManager
    logMgr     *LogManager
    
    // For MVCC
    versionMgr *VersionManager
    
    // Deadlock detection
    deadlockDetector *DeadlockDetector
}

type Transaction struct {
    ID         TxnID
    State      TxnState
    IsolationLevel IsolationLevel
    StartTime  time.Time
    
    // Tracking changes
    readSet    *ReadSet
    writeSet   *WriteSet
    
    // Locking
    heldLocks  []*Lock
    
    // Undo information
    undoLog    []UndoRecord
}
```

#### 3.4.2 Concurrency Control

Initial implementation uses Two-Phase Locking (2PL) with future MVCC support:

```go
type LockManager struct {
    lockTable map[ResourceID]*LockQueue
    waitsFor  map[TxnID]TxnID  // For deadlock detection
    
    mu sync.Mutex
}

type Lock struct {
    resourceID ResourceID
    txnID      TxnID
    mode       LockMode
    grantTime  time.Time
}

type LockMode uint8
const (
    LockModeShared LockMode = iota
    LockModeExclusive
    LockModeIntentionShared
    LockModeIntentionExclusive
    LockModeSharedIntentionExclusive
)
```

#### 3.4.3 MVCC Architecture (Phase 2)

Multi-Version Concurrency Control for improved read performance:

```go
type VersionManager struct {
    versions  map[RecordID]*VersionChain
    gcWorker *GarbageCollector
}

type Version struct {
    data      []byte
    txnID     TxnID
    beginTime Timestamp
    endTime   *Timestamp  // nil for current version
    next      *Version
}
```

### 3.5 Write-Ahead Logging (WAL)

#### 3.5.1 Log Structure

The WAL ensures durability and enables recovery:

```go
type LogManager struct {
    logFile   *os.File
    buffer    *LogBuffer
    nextLSN   atomic.Uint64
    
    // Group commit optimization
    commitQueue chan *CommitRequest
}

type LogRecord struct {
    LSN       LSN
    Type      LogRecordType
    TxnID     TxnID
    PrevLSN   LSN  // Previous record for this transaction
    
    // Payload varies by type
    Payload   LogPayload
    
    Checksum  uint32
}

type LogRecordType uint8
const (
    LogRecordBegin LogRecordType = iota
    LogRecordCommit
    LogRecordAbort
    LogRecordUpdate
    LogRecordCompensation
    LogRecordCheckpoint
)

type UpdateLogRecord struct {
    TableID    TableID
    PageID     PageID
    Offset     uint16
    OldData    []byte
    NewData    []byte
}
```

#### 3.5.2 Recovery Protocol

Implements a simplified ARIES protocol:

```go
type RecoveryManager struct {
    logMgr     *LogManager
    bufferPool *BufferPool
    
    // Recovery state
    transactionTable map[TxnID]*TxnTableEntry
    dirtyPageTable   map[PageID]LSN
}

// Recovery phases
func (rm *RecoveryManager) Recover() error {
    // Phase 1: Analysis
    checkpoint, err := rm.findLastCheckpoint()
    if err != nil {
        return err
    }
    
    err = rm.analyzeLog(checkpoint)
    if err != nil {
        return err
    }
    
    // Phase 2: Redo
    err = rm.redoLog(rm.getRedoStartPoint())
    if err != nil {
        return err
    }
    
    // Phase 3: Undo
    err = rm.undoIncompleteTransactions()
    if err != nil {
        return err
    }
    
    return nil
}
```

### 3.6 Advanced Features

#### 3.6.1 Adaptive Indexing

Automatically creates and maintains indexes based on query patterns:

```go
type AdaptiveIndexManager struct {
    queryStats   *QueryStatistics
    indexAdvisor *IndexAdvisor
    builder      *IndexBuilder
    
    // Configuration
    threshold    int  // Queries before creating index
    maxIndexes   int  // Maximum auto-created indexes
}

type QueryStatistics struct {
    predicateFreq map[Predicate]int64
    selectivity   map[Predicate]float64
    executionTime map[Predicate]time.Duration
}
```

#### 3.6.2 Time-Travel Queries

Leverages MVCC infrastructure for historical queries:

```go
type TemporalQuerier struct {
    versionMgr *VersionManager
    snapshots  *SnapshotManager
}

// SQL Extension: AS OF SYSTEM TIME
// SELECT * FROM orders AS OF SYSTEM TIME '2024-01-01 00:00:00'
```

#### 3.6.3 Machine Learning Integration

Optional ML-enhanced query optimization:

```go
type MLOptimizer struct {
    model       MLModel
    features    *FeatureExtractor
    trainer     *OnlineTrainer
}

type QueryFeatures struct {
    TableSizes      []int64
    PredicateTypes  []PredicateType
    JoinTypes       []JoinType
    EstimatedCard   int64
}
```

## 4. Data Flow Architecture

### 4.1 Read Path

1. Query arrives at client interface
2. Parser creates AST
3. Analyzer validates and type-checks
4. Optimizer creates physical plan
5. Executor requests pages from buffer pool
6. Buffer pool checks cache or loads from disk
7. Results returned to client

### 4.2 Write Path

1. Write request arrives
2. Transaction manager assigns transaction
3. Lock manager acquires necessary locks
4. Changes logged to WAL
5. Pages modified in buffer pool
6. On commit, WAL flushed to disk
7. Dirty pages eventually flushed by background thread

### 4.3 Recovery Path

1. System detects unclean shutdown
2. Recovery manager reads WAL from last checkpoint
3. Analysis phase builds recovery state
4. Redo phase replays committed changes
5. Undo phase rolls back incomplete transactions
6. System ready for normal operation

## 5. Concurrency Model

### 5.1 Thread Architecture

The system uses goroutines for concurrent operations:

```go
type DatabaseRuntime struct {
    // Core workers
    mainLoop      goroutine  // Handles client requests
    walWriter     goroutine  // Dedicated WAL writer
    checkpointer  goroutine  // Periodic checkpointing
    
    // Background workers
    compactor     goroutine  // Page compaction
    statsCollector goroutine  // Statistics gathering
    deadlockMon   goroutine  // Deadlock detection
    
    // Worker pools
    queryWorkers  []goroutine  // Query execution
    ioWorkers     []goroutine  // Disk I/O operations
}
```

### 5.2 Synchronization Strategy

- **Page Latches**: Short-term locks for physical consistency
- **Transaction Locks**: Long-term locks for logical consistency
- **Lock-Free Structures**: Used for read-only operations and statistics

### 5.3 Deadlock Prevention

Combination of strategies:

- Wait-die or wound-wait protocols
- Timeout-based detection
- Cycle detection in wait-for graph

## 6. Performance Considerations

### 6.1 Memory Management

- Reuse allocations through object pools
- Minimize garbage collection pressure
- Use mmap for read-only data when appropriate

### 6.2 I/O Optimization

- Group commit for WAL writes
- Asynchronous page flushing
- Read-ahead for sequential scans
- Direct I/O for large operations

### 6.3 CPU Optimization

- SIMD operations for bulk data processing
- Cache-conscious data structures
- Branch prediction friendly code paths

### 6.4 Benchmarking Framework

```go
type Benchmark struct {
    name      string
    workload  Workload
    duration  time.Duration
    
    // Metrics collected
    throughput float64
    latency    LatencyDistribution
    cpuUsage   float64
    ioStats    IOStatistics
}
```

## 7. Security Architecture

### 7.1 Current Scope

- File-level encryption support
- Checksum verification at all levels
- SQL injection prevention through parameterized queries

### 7.2 Future Enhancements

- User authentication and authorization
- Row-level security
- Audit logging
- Network encryption for client connections

## 8. Extensibility Framework

### 8.1 Plugin System

```go
type Plugin interface {
    Name() string
    Version() string
    Initialize(db *Database) error
    Shutdown() error
}

type StoragePlugin interface {
    Plugin
    CreateEngine(config Config) (StorageEngine, error)
}
```

### 8.2 Custom Functions

```go
type FunctionRegistry struct {
    functions map[string]CustomFunction
}

type CustomFunction interface {
    Name() string
    ArgumentTypes() []DataType
    ReturnType() DataType
    Execute(args []Value) (Value, error)
}
```

## 9. Testing Architecture

### 9.1 Unit Testing

- Component isolation through interfaces
- Mock implementations for all major components
- Property-based testing for data structures

### 9.2 Integration Testing

- End-to-end transaction testing
- Crash recovery scenarios
- Concurrent workload testing

### 9.3 Performance Testing

- Microbenchmarks for critical paths
- Macro benchmarks with standard workloads
- Regression testing for performance

### 9.4 Chaos Testing

- Random fault injection
- Network partition simulation
- Resource exhaustion scenarios

## 10. Monitoring and Observability

### 10.1 Metrics Collection

```go
type MetricsCollector struct {
    // Performance metrics
    queryLatency    *Histogram
    txnThroughput   *Counter
    bufferHitRate   *Gauge
    
    // Resource metrics
    memoryUsage     *Gauge
    diskIORate      *Counter
    cpuUtilization  *Gauge
    
    // Health metrics
    activeConns     *Gauge
    errorRate       *Counter
    deadlockCount   *Counter
}
```

### 10.2 Logging Framework

- Structured logging with levels
- Query logging for analysis
- Slow query identification
- Error tracking and reporting

### 10.3 Debugging Tools

- Query plan visualization
- Lock contention analysis
- Buffer pool inspection
- Transaction state viewer

## 11. Configuration Management

### 11.1 Configuration Structure

```go
type Config struct {
    // Storage configuration
    DataDir         string
    PageSize        int
    MaxFileSize     int64
    
    // Memory configuration
    BufferPoolSize  int
    MaxConnections  int
    QueryCacheSize  int
    
    // Transaction configuration
    IsolationLevel  IsolationLevel
    LockTimeout     time.Duration
    EnableMVCC      bool
    
    // Performance tuning
    CheckpointInterval  time.Duration
    WALSyncMode        SyncMode
    CompactionThreshold float64
    
    // Advanced features
    EnableAdaptiveIndexing bool
    EnableMLOptimizer      bool
    EnableCompression      bool
}
```

### 11.2 Dynamic Configuration

Some parameters can be changed at runtime:

```go
type DynamicConfig struct {
    checkpointInterval atomic.Value
    lockTimeout        atomic.Value
    logLevel          atomic.Value
}
```

## 12. Deployment Considerations

### 12.1 Embedded Mode

Primary deployment model as a library:

- Single process, no network overhead
- Direct function calls for all operations
- Shared memory with application

### 12.2 Server Mode (Future)

Optional client-server architecture:

- TCP/IP or Unix socket connections
- Wire protocol for remote access
- Connection pooling and management

### 12.3 Resource Requirements

Minimum requirements:

- Memory: 64MB (configurable buffer pool)
- Disk: 10MB + data size
- CPU: 1 core (benefits from multiple cores)

## 13. Development Practices

### 13.1 Code Organization

```
godb/
├── cmd/              # Command-line tools
├── pkg/
│   ├── storage/      # Storage engine
│   ├── transaction/  # Transaction management
│   ├── query/        # Query processing
│   ├── recovery/     # Recovery system
│   ├── utils/        # Common utilities
│   └── api/          # Public API
├── internal/         # Internal packages
├── test/             # Test suites
└── docs/             # Documentation
```

### 13.2 Development Workflow

1. Design document for major features
2. Interface definition and review
3. Implementation with comprehensive tests
4. Performance benchmarking
5. Code review and optimization
6. Documentation and examples

### 13.3 Quality Standards

- Test coverage > 80%
- All public APIs documented
- Performance benchmarks for critical paths
- No race conditions (verified by race detector)
- Clean static analysis results

This architecture provides a solid foundation for building a production-quality database engine while maintaining flexibility for experimentation and learning. The modular design allows for incremental development and easy extension of functionality.