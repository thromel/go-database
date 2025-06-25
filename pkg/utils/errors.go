package utils

import (
	"errors"
	"fmt"
)

// Common database errors
var (
	// ErrKeyNotFound is returned when a requested key does not exist
	ErrKeyNotFound = errors.New("key not found")

	// ErrKeyExists is returned when trying to create a key that already exists
	ErrKeyExists = errors.New("key already exists")

	// ErrDatabaseClosed is returned when attempting operations on a closed database
	ErrDatabaseClosed = errors.New("database is closed")

	// ErrDatabaseCorrupted is returned when database corruption is detected
	ErrDatabaseCorrupted = errors.New("database corrupted")

	// ErrInvalidKey is returned when a key is invalid (e.g., nil or empty)
	ErrInvalidKey = errors.New("invalid key")

	// ErrInvalidValue is returned when a value is invalid
	ErrInvalidValue = errors.New("invalid value")

	// ErrKeyTooLarge is returned when a key exceeds the maximum allowed size
	ErrKeyTooLarge = errors.New("key too large")

	// ErrValueTooLarge is returned when a value exceeds the maximum allowed size
	ErrValueTooLarge = errors.New("value too large")
)

// Transaction-related errors
var (
	// ErrTransactionNotFound is returned when a transaction ID is not found
	ErrTransactionNotFound = errors.New("transaction not found")

	// ErrTransactionClosed is returned when attempting operations on a closed transaction
	ErrTransactionClosed = errors.New("transaction is closed")

	// ErrTransactionCommitted is returned when attempting operations on a committed transaction
	ErrTransactionCommitted = errors.New("transaction already committed")

	// ErrTransactionRolledBack is returned when attempting operations on a rolled back transaction
	ErrTransactionRolledBack = errors.New("transaction already rolled back")

	// ErrTransactionConflict is returned when a transaction conflict is detected
	ErrTransactionConflict = errors.New("transaction conflict")

	// ErrTransactionDeadlock is returned when a deadlock is detected
	ErrTransactionDeadlock = errors.New("transaction deadlock detected")

	// ErrTransactionTimeout is returned when a transaction exceeds its timeout
	ErrTransactionTimeout = errors.New("transaction timeout")

	// ErrTransactionReadOnly is returned when attempting write operations on a read-only transaction
	ErrTransactionReadOnly = errors.New("transaction is read-only")
)

// Storage-related errors
var (
	// ErrStorageFull is returned when storage space is exhausted
	ErrStorageFull = errors.New("storage full")

	// ErrStorageCorrupted is returned when storage corruption is detected
	ErrStorageCorrupted = errors.New("storage corrupted")

	// ErrStorageLocked is returned when storage is locked by another process
	ErrStorageLocked = errors.New("storage is locked")

	// ErrStorageReadOnly is returned when attempting write operations on read-only storage
	ErrStorageReadOnly = errors.New("storage is read-only")

	// ErrStorageUnavailable is returned when storage is temporarily unavailable
	ErrStorageUnavailable = errors.New("storage unavailable")
)

// Iterator-related errors
var (
	// ErrIteratorClosed is returned when attempting operations on a closed iterator
	ErrIteratorClosed = errors.New("iterator is closed")

	// ErrIteratorInvalid is returned when the iterator is in an invalid state
	ErrIteratorInvalid = errors.New("iterator is invalid")
)

// Configuration-related errors
var (
	// ErrInvalidConfig is returned when configuration is invalid
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrConfigRequired is returned when required configuration is missing
	ErrConfigRequired = errors.New("configuration required")
)

// DatabaseError represents a structured database error with additional context
type DatabaseError struct {
	// Op is the operation that caused the error
	Op string

	// Path is the database path (if applicable)
	Path string

	// Key is the key involved in the operation (if applicable)
	Key []byte

	// Err is the underlying error
	Err error
}

// Error implements the error interface
func (e *DatabaseError) Error() string {
	if e.Path != "" && e.Key != nil {
		return fmt.Sprintf("database error in %s (path: %s, key: %x): %v", e.Op, e.Path, e.Key, e.Err)
	}
	if e.Path != "" {
		return fmt.Sprintf("database error in %s (path: %s): %v", e.Op, e.Path, e.Err)
	}
	if e.Key != nil {
		return fmt.Sprintf("database error in %s (key: %x): %v", e.Op, e.Key, e.Err)
	}
	return fmt.Sprintf("database error in %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error for error chain support
func (e *DatabaseError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target error
func (e *DatabaseError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// NewDatabaseError creates a new DatabaseError
func NewDatabaseError(op string, err error) *DatabaseError {
	return &DatabaseError{
		Op:  op,
		Err: err,
	}
}

// NewDatabaseErrorWithPath creates a new DatabaseError with path context
func NewDatabaseErrorWithPath(op, path string, err error) *DatabaseError {
	return &DatabaseError{
		Op:   op,
		Path: path,
		Err:  err,
	}
}

// NewDatabaseErrorWithKey creates a new DatabaseError with key context
func NewDatabaseErrorWithKey(op string, key []byte, err error) *DatabaseError {
	return &DatabaseError{
		Op:  op,
		Key: key,
		Err:  err,
	}
}

// NewDatabaseErrorWithContext creates a new DatabaseError with full context
func NewDatabaseErrorWithContext(op, path string, key []byte, err error) *DatabaseError {
	return &DatabaseError{
		Op:   op,
		Path: path,
		Key:  key,
		Err:  err,
	}
}

// IsKeyNotFound checks if an error indicates a key was not found
func IsKeyNotFound(err error) bool {
	return errors.Is(err, ErrKeyNotFound)
}

// IsTransactionConflict checks if an error indicates a transaction conflict
func IsTransactionConflict(err error) bool {
	return errors.Is(err, ErrTransactionConflict)
}

// IsTransactionDeadlock checks if an error indicates a transaction deadlock
func IsTransactionDeadlock(err error) bool {
	return errors.Is(err, ErrTransactionDeadlock)
}

// IsStorageError checks if an error is storage-related
func IsStorageError(err error) bool {
	return errors.Is(err, ErrStorageFull) ||
		errors.Is(err, ErrStorageCorrupted) ||
		errors.Is(err, ErrStorageLocked) ||
		errors.Is(err, ErrStorageReadOnly) ||
		errors.Is(err, ErrStorageUnavailable)
}

// IsRetryableError checks if an error indicates the operation can be retried
func IsRetryableError(err error) bool {
	return errors.Is(err, ErrTransactionConflict) ||
		errors.Is(err, ErrTransactionDeadlock) ||
		errors.Is(err, ErrStorageUnavailable)
}