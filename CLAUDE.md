# CLAUDE.md - Instructions for AI Agents

## What This Project Is
This is a **Go Database Engine** implementation project - building a complete, production-ready database engine from scratch in Go. This is NOT a typical web application or simple tool. It's a deep systems programming project that implements core computer science concepts.

## Current State: PLANNING COMPLETE, IMPLEMENTATION STARTING
- ✅ **Architecture designed** - Comprehensive technical specifications exist
- ✅ **Sprint planning complete** - 15 detailed sprints with user stories and tasks
- ✅ **Documentation exists** - Architecture docs and research materials available
- 🚀 **Ready to code** - No implementation exists yet, starting from Sprint 1

## Your Role as an AI Agent
You are helping implement a database engine that will:
1. **Store data persistently** using B+ trees and page-based storage
2. **Provide ACID transactions** with proper concurrency control
3. **Parse and execute SQL** with cost-based optimization
4. **Recover from crashes** using write-ahead logging
5. **Include advanced features** like adaptive indexing and time-travel queries

## Critical Context: This is EDUCATIONAL + PRODUCTION
- **Educational**: Learn database internals by building everything from scratch
- **Production**: Create a real, usable database engine that works correctly
- **No shortcuts**: Don't use existing database libraries - implement core algorithms yourself

## Project Structure You'll Be Working With
```
/Users/romel/Documents/GitHub/go-database/
├── docs/
│   ├── architecture.md      # Technical architecture (READ THIS)
│   └── research.md          # Design decisions and research
├── sprints/
│   ├── README.md           # Sprint methodology overview
│   ├── sprint-01-foundation.md    # Current sprint to start
│   ├── sprint-02-storage-engine.md
│   └── ... (15 sprint files total)
├── pkg/                    # Main packages (TO BE CREATED)
├── internal/              # Private packages (TO BE CREATED)
├── cmd/                   # CLI tools (TO BE CREATED)
└── test/                  # Integration tests (TO BE CREATED)
```

## ALWAYS START HERE: Sprint 1 Foundation
**File**: `/Users/romel/Documents/GitHub/go-database/sprints/sprint-01-foundation.md`

**Sprint 1 Key Tasks**:
1. **Project Setup**: Initialize Go module, create package structure
2. **Core Interfaces**: Define Database, StorageEngine, Transaction interfaces  
3. **In-Memory KV Store**: Basic key-value operations with thread safety
4. **Testing Framework**: Unit tests, CI/CD pipeline
5. **Documentation**: README, development guidelines

## Key Architecture Layers (What You'll Build)

### 1. Storage Engine
- **B+ Tree**: Custom implementation for efficient key-value storage
- **Page Management**: 8KB fixed pages with headers and checksums
- **Buffer Pool**: Memory management with LRU eviction
- **File System**: Single-file database with atomic operations

### 2. Transaction Manager
- **ACID Properties**: Atomicity, Consistency, Isolation, Durability
- **Concurrency Control**: Two-Phase Locking (2PL) initially, then MVCC
- **Deadlock Detection**: Automatic detection and resolution
- **Undo/Redo Logging**: For transaction rollback and recovery

### 3. Query Processor
- **SQL Parser**: Convert SQL strings to Abstract Syntax Trees
- **Semantic Analyzer**: Type checking and schema validation
- **Query Optimizer**: Cost-based optimization with statistics
- **Execution Engine**: Iterator-based physical operators

### 4. Recovery System
- **Write-Ahead Logging**: ARIES protocol for crash recovery
- **Checkpointing**: Periodic snapshots to limit recovery time
- **Three-Phase Recovery**: Analysis, Redo, Undo phases

## Technical Standards You Must Follow

### Go Code Requirements
- **Go 1.21+** - Use modern Go features
- **Interfaces First** - Define clear contracts between components
- **Thread Safety** - All public APIs must be concurrent-safe
- **Error Handling** - Proper Go error patterns, wrap with context
- **Testing** - >80% code coverage, unit + integration tests

### Performance Targets
- **Storage**: <1ms cached reads, >10K keys/sec scans
- **Transactions**: <100μs begin, <1ms commit, >10K txn/sec  
- **Queries**: <1ms simple queries, >10K queries/sec
- **Concurrency**: Support >1000 active transactions

### Database Requirements
- **ACID Compliance** - Must pass standard transaction tests
- **Crash Safety** - Zero data loss on unexpected shutdown
- **SQL Support** - Subset of SQL with SELECT, INSERT, UPDATE, DELETE
- **Performance** - Competitive with SQLite for similar workloads

## How to Work on This Project

### 1. Always Read Sprint Files First
Each sprint has:
- **User Stories** with acceptance criteria
- **Technical Tasks** with implementation details
- **Definition of Done** checklist
- **Performance targets** and success metrics

### 2. Follow Sprint Order
Don't jump ahead. Each sprint builds on previous ones:
- Sprint 1-3: Foundation and storage
- Sprint 4-6: Transactions and concurrency  
- Sprint 7-9: Query processing
- Sprint 10-12: Advanced features
- Sprint 13-15: Production readiness

### 3. Reference Architecture Documents
- `docs/architecture.md` - Comprehensive technical design
- `docs/research.md` - Design rationale and alternatives
- Sprint files - Detailed implementation requirements

### 4. Use This Package Structure
```go
// Core interfaces in pkg/api/
type Database interface {
    Open(path string, config *Config) error
    Close() error
    Begin() (Transaction, error)
    Put(key []byte, value []byte) error
    Get(key []byte) ([]byte, error)
    Delete(key []byte) error
}

type StorageEngine interface {
    Get(key []byte) ([]byte, error)
    Put(key []byte, value []byte) error
    Delete(key []byte) error
    NewIterator(start, end []byte) Iterator
}

type Transaction interface {
    Put(key []byte, value []byte) error
    Get(key []byte) ([]byte, error)
    Delete(key []byte) error
    Commit() error
    Rollback() error
}
```

### 5. Testing Strategy
- **Unit Tests**: Test each component in isolation
- **Integration Tests**: End-to-end scenarios
- **Concurrency Tests**: Use Go race detector
- **Recovery Tests**: Simulate crashes and verify recovery
- **Performance Tests**: Benchmark critical paths

## What NOT to Do

### ❌ Don't Use External Database Libraries
- No SQLite, BoltDB, BadgerDB for core functionality
- No ORMs or query builders
- Build B+ trees, WAL, parser yourself
- Exception: Use libraries for crypto, compression, JSON

### ❌ Don't Skip Testing
- Database correctness is critical
- Write tests as you implement features
- Test failure scenarios (crashes, corruption)
- Verify ACID properties with concurrent tests

### ❌ Don't Ignore Concurrency
- Design for concurrent access from the start
- Use proper locking and synchronization
- Test with multiple goroutines
- Avoid race conditions

### ❌ Don't Optimize Prematurely
- Get correctness first, then performance
- Focus on algorithmic efficiency
- Profile before optimizing
- Meet performance targets from sprint files

## Key Learning Goals
By building this database engine, you'll master:
1. **Storage Systems**: B+ trees, page management, buffer pools
2. **Transaction Processing**: ACID properties, concurrency control
3. **Query Processing**: Parsing, optimization, execution
4. **Recovery Systems**: WAL, checkpointing, crash recovery
5. **Systems Programming**: Go concurrency, memory management, I/O

## Implementation Tips

### Start Simple, Then Extend
- Begin with basic functionality that works correctly
- Add complexity incrementally
- Test thoroughly at each step
- Refactor when patterns become clear

### Focus on Interfaces
- Define clear contracts between components
- Mock interfaces for testing
- Allow multiple implementations
- Make components swappable

### Leverage Go's Strengths
- Use goroutines for concurrent operations
- Use channels for communication
- Use sync package for low-level synchronization
- Take advantage of Go's garbage collector

### Study Existing Systems
- Read SQLite source code for reference
- Study BoltDB for Go-specific patterns
- Reference PostgreSQL documentation for algorithms
- Learn from academic papers (ARIES, etc.)

## Success Criteria
You'll know you're succeeding when:
- ✅ All sprint acceptance criteria are met
- ✅ Tests pass consistently
- ✅ Performance targets are achieved
- ✅ Code is clean and well-documented
- ✅ ACID properties are verified
- ✅ System recovers correctly from crashes

## When You Get Stuck
1. **Check sprint files** for detailed requirements
2. **Read architecture.md** for design context
3. **Look at test scenarios** to understand expected behavior
4. **Study referenced algorithms** from academic sources
5. **Break complex problems** into smaller pieces

## Remember: This is Hard but Rewarding
Building a database engine is one of the most challenging projects in computer science. You're implementing decades of research and engineering. Take time to understand the concepts deeply, not just implement features. The goal is both a working system AND deep learning of database internals.

## Quick Start Checklist
To begin working on this project:
- [ ] Read `/Users/romel/Documents/GitHub/go-database/docs/architecture.md`
- [ ] Review `/Users/romel/Documents/GitHub/go-database/sprints/sprint-01-foundation.md`
- [ ] Initialize Go module: `go mod init github.com/romel/go-database`
- [ ] Create basic package structure under `pkg/`
- [ ] Start with Sprint 1, Story GODB-001 (Project Setup)
- [ ] Follow the Definition of Done for each story

Good luck building an amazing database engine! 🚀