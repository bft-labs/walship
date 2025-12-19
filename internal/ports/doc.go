// Package ports defines the interfaces (ports) that connect the application
// layer to infrastructure adapters.
//
// In Clean Architecture / Hexagonal Architecture, ports are the boundaries
// between the application core and the outside world. They define what the
// application needs from external systems without specifying how those needs
// are fulfilled.
//
// # Port Interfaces
//
//   - [FrameReader]: Reads WAL frames from index files
//   - [FrameSender]: Sends batches of frames to the remote service
//   - [StateRepository]: Persists and loads agent state
//   - [Logger]: Structured logging abstraction
//   - [HTTPClient]: HTTP request abstraction for dependency injection
//
// # Usage
//
// The application layer (internal/app) depends only on these interfaces.
// Infrastructure adapters (internal/adapters) implement these interfaces
// with concrete implementations (file system, HTTP, zerolog, etc.).
//
// This separation enables:
//   - Testing application logic with mock implementations
//   - Swapping infrastructure without changing business logic
//   - Clear boundaries and dependency direction
package ports
