package sender

// Metadata provides context for the send operation.
// This information is included in HTTP headers for server-side tracking.
type Metadata struct {
	// ChainID is the blockchain chain identifier
	ChainID string

	// NodeID is the node identifier
	NodeID string

	// Hostname is the agent's hostname
	Hostname string

	// OSArch is the operating system and architecture (e.g., "linux/amd64")
	OSArch string

	// AuthKey is the API authentication key
	AuthKey string

	// ServiceURL is the base URL of the ingestion service
	ServiceURL string
}
