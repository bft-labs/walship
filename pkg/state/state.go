package state

import "time"

// State represents persistent state for crash recovery.
// This state is saved to disk after each successful batch send.
type State struct {
	// IdxPath is the current index file path
	IdxPath string `json:"idx_path"`

	// IdxOffset is the current read position in the index file
	IdxOffset int64 `json:"idx_offset"`

	// CurGz is the current .gz filename being read
	CurGz string `json:"cur_gz"`

	// LastFile is the last file that was successfully sent
	LastFile string `json:"last_file"`

	// LastFrame is the last frame number that was successfully sent
	LastFrame uint64 `json:"last_frame"`

	// LastCommitAt is the timestamp of the last successful send
	LastCommitAt time.Time `json:"last_commit_at"`

	// LastSendAt is the timestamp of the last send attempt
	LastSendAt time.Time `json:"last_send_at"`
}

// IsEmpty returns true if the state has not been initialized.
func (s State) IsEmpty() bool {
	return s.IdxPath == ""
}

// UpdateAfterSend updates the state after a successful batch send.
func (s *State) UpdateAfterSend(idxAdvance int64, lastFile string, lastFrame uint64) {
	s.IdxOffset += idxAdvance
	s.LastFile = lastFile
	s.LastFrame = lastFrame
	now := time.Now()
	s.LastCommitAt = now
	s.LastSendAt = now
}

// UpdatePosition updates the index position without a send.
func (s *State) UpdatePosition(idxPath string, idxOffset int64, curGz string) {
	s.IdxPath = idxPath
	s.IdxOffset = idxOffset
	s.CurGz = curGz
}
