package agent

import "time"

// FrameMeta matches tools/memlogger/writer.go schema for index lines.
// Fields are used to locate and read gzip members from the .gz file.
type FrameMeta struct {
    File    string `json:"file"`
    Frame   uint64 `json:"frame"`
    Off     uint64 `json:"off"`
    Len     uint64 `json:"len"`
    Recs    uint32 `json:"recs"`
    FirstTS int64  `json:"first_ts"`
    LastTS  int64  `json:"last_ts"`
    CRC32   uint32 `json:"crc32"`
}

type Config struct {
    Root   string
    NodeID string
    WALDir string

    RemoteURL string
    RemoteBase string
    Network    string
    RemoteNode string
    AuthKey   string

    PollInterval time.Duration
    SendInterval time.Duration
    HardInterval time.Duration
    HTTPTimeout  time.Duration

    CPUThreshold     float64
    NetThreshold     float64
    Iface            string
    IfaceSpeedMbps   int
    MaxBatchBytes    int
    StateDir         string
    Verify           bool
    Meta             bool
    Once             bool
}

