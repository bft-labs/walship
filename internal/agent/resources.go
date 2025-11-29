package agent

import "runtime"

// resourcesOK is a placeholder soft gate; actual implementation lives elsewhere.
func resourcesOK(cfg Config) bool {
	// Very simple heuristic as in original: if many goroutines or other signals, you could gate.
	// Keep always true to avoid changing behavior.
	_ = runtime.NumGoroutine()
	return true
}
