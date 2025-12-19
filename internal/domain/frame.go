package domain

// Frame represents a single WAL frame with metadata.
// A frame is the atomic unit of data read from WAL index files.
type Frame struct {
	// File is the segment filename (e.g., "seg-000001.wal.gz")
	File string

	// FrameNumber is the sequence number within the segment
	FrameNumber uint64

	// Offset is the byte offset in the .gz file
	Offset uint64

	// Length is the byte length of the compressed data
	Length uint64

	// RecordCount is the number of records in this frame
	RecordCount uint32

	// FirstTimestamp is the earliest timestamp in unix nanoseconds
	FirstTimestamp int64

	// LastTimestamp is the latest timestamp in unix nanoseconds
	LastTimestamp int64

	// CRC32 is the checksum for data integrity verification
	CRC32 uint32
}

// FrameMeta is an alias for JSON serialization compatibility with the existing
// index file format used by memlogger.
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

// ToFrame converts FrameMeta to a Frame domain entity.
func (m FrameMeta) ToFrame() Frame {
	return Frame{
		File:           m.File,
		FrameNumber:    m.Frame,
		Offset:         m.Off,
		Length:         m.Len,
		RecordCount:    m.Recs,
		FirstTimestamp: m.FirstTS,
		LastTimestamp:  m.LastTS,
		CRC32:          m.CRC32,
	}
}

// ToMeta converts a Frame to FrameMeta for JSON serialization.
func (f Frame) ToMeta() FrameMeta {
	return FrameMeta{
		File:    f.File,
		Frame:   f.FrameNumber,
		Off:     f.Offset,
		Len:     f.Length,
		Recs:    f.RecordCount,
		FirstTS: f.FirstTimestamp,
		LastTS:  f.LastTimestamp,
		CRC32:   f.CRC32,
	}
}
