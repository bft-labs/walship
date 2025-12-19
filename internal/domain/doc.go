// Package domain contains the core domain entities and value objects for walship.
//
// This package represents the innermost layer of the Clean Architecture. It has
// no dependencies on infrastructure concerns (HTTP, file system, logging) and
// contains only pure business logic.
//
// # Entities
//
//   - [Frame]: A single WAL frame with metadata (file, offset, length, CRC, etc.)
//   - [Batch]: An aggregate of frames ready to be sent together
//   - [State]: Persistent state for crash recovery (index position, last sent frame)
//
// # Design Principles
//
// Domain entities are:
//   - Immutable after construction (where practical)
//   - Free of infrastructure dependencies
//   - Focused on business rules and invariants
//   - Testable without mocks or external systems
package domain
