package walship

import (
	"github.com/bft-labs/walship/pkg/log"
	"github.com/bft-labs/walship/pkg/sender"
	"github.com/bft-labs/walship/pkg/state"
	"github.com/bft-labs/walship/pkg/wal"
)

// Version information for the walship facade.
const (
	// Version is the current version of the walship package.
	Version = "1.0.0"

	// MinCompatibleVersion is the minimum version that is compatible with this version.
	MinCompatibleVersion = "1.0.0"
)

// ModuleVersions returns the versions of all sub-modules.
func ModuleVersions() map[string]string {
	return map[string]string{
		"walship": Version,
		"wal":     wal.Version,
		"sender":  sender.Version,
		"state":   state.Version,
		"log":     log.Version,
	}
}

// CompatibilityMatrix defines the minimum compatible versions for each module.
// These versions are validated during New() to ensure all modules work together.
var CompatibilityMatrix = map[string]string{
	"wal":    wal.MinCompatibleVersion,
	"sender": sender.MinCompatibleVersion,
	"state":  state.MinCompatibleVersion,
	"log":    log.MinCompatibleVersion,
}
