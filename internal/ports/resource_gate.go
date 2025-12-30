package ports

// ResourceGate checks system resources before allowing batch sends.
// When resources are constrained, it returns false to delay sending.
//
// This is a core feature that ensures walship never impacts node performance.
// Implementations should monitor system metrics (CPU, network, etc.) and
// return false when the system is under heavy load.
type ResourceGate interface {
	// OK returns true if system resources allow sending, false otherwise.
	//
	// Returns false when:
	//   - CPU usage exceeds the configured threshold
	//   - Network utilization exceeds the configured threshold
	//   - Any other resource constraint that would impact node performance
	//
	// When false, the caller should delay sending until either:
	//   - OK() returns true on a subsequent check, or
	//   - HardInterval is exceeded (forced send regardless of resource state)
	OK() bool
}
