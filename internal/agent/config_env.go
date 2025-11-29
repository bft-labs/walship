package agent

import "os"

// ApplyEnvConfig applies configuration from environment variables (WALSHIP_*).
// It respects flags that have been explicitly set (changed map).
// Returns error if any environment variable has an invalid format.
func ApplyEnvConfig(cfg *Config, changed map[string]bool) error {
	s := newConfigSetter(changed)

	s.setString("node-home", os.Getenv("WALSHIP_NODE_HOME"), &cfg.NodeHome)
	s.setString("node-id", os.Getenv("WALSHIP_NODE_ID"), &cfg.NodeID)
	s.setString("wal-dir", os.Getenv("WALSHIP_WAL_DIR"), &cfg.WALDir)
	s.setString("service-url", os.Getenv("WALSHIP_SERVICE_URL"), &cfg.ServiceURL)
	s.setString("auth-key", os.Getenv("WALSHIP_AUTH_KEY"), &cfg.AuthKey)
	s.setString("iface", os.Getenv("WALSHIP_IFACE"), &cfg.Iface)
	s.setString("state-dir", os.Getenv("WALSHIP_STATE_DIR"), &cfg.StateDir)

	if err := s.setDuration("poll", os.Getenv("WALSHIP_POLL_INTERVAL"), &cfg.PollInterval); err != nil {
		return err
	}
	if err := s.setDuration("send-interval", os.Getenv("WALSHIP_SEND_INTERVAL"), &cfg.SendInterval); err != nil {
		return err
	}
	if err := s.setDuration("hard-interval", os.Getenv("WALSHIP_HARD_INTERVAL"), &cfg.HardInterval); err != nil {
		return err
	}
	if err := s.setDuration("timeout", os.Getenv("WALSHIP_HTTP_TIMEOUT"), &cfg.HTTPTimeout); err != nil {
		return err
	}

	if err := s.setFloatFromString("cpu-threshold", os.Getenv("WALSHIP_CPU_THRESHOLD"), &cfg.CPUThreshold); err != nil {
		return err
	}
	if err := s.setFloatFromString("net-threshold", os.Getenv("WALSHIP_NET_THRESHOLD"), &cfg.NetThreshold); err != nil {
		return err
	}

	if err := s.setIntFromString("iface-speed", os.Getenv("WALSHIP_IFACE_SPEED_MBPS"), &cfg.IfaceSpeedMbps); err != nil {
		return err
	}
	if err := s.setIntFromString("max-batch-bytes", os.Getenv("WALSHIP_MAX_BATCH_BYTES"), &cfg.MaxBatchBytes); err != nil {
		return err
	}

	s.setBoolFromString("verify", os.Getenv("WALSHIP_VERIFY"), &cfg.Verify)
	s.setBoolFromString("meta", os.Getenv("WALSHIP_META"), &cfg.Meta)
	s.setBoolFromString("once", os.Getenv("WALSHIP_ONCE"), &cfg.Once)

	return nil
}
