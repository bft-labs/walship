package ports

// ResourceGate checks system resources before allowing batch sends.
// When resources are constrained, it returns false to delay sending.
type ResourceGate interface {
	// OK returns true if system resources allow sending.
	// When false, the caller should delay sending until HardInterval.
	OK() bool
}
