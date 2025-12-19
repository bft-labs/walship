package domain

// Batch is an aggregate of frames ready to be sent together.
// It maintains the invariant that Frames and CompressedData have the same length.
type Batch struct {
	// Frames contains the metadata for each frame in the batch
	Frames []Frame

	// CompressedData contains the raw compressed bytes for each frame
	CompressedData [][]byte

	// TotalBytes is the sum of all compressed data lengths
	TotalBytes int

	// IdxLineLengths stores the length of each index line for offset tracking
	IdxLineLengths []int
}

// NewBatch creates a new empty batch.
func NewBatch() *Batch {
	return &Batch{
		Frames:         make([]Frame, 0),
		CompressedData: make([][]byte, 0),
		IdxLineLengths: make([]int, 0),
	}
}

// Add appends a frame and its compressed data to the batch.
func (b *Batch) Add(frame Frame, compressed []byte, idxLineLen int) {
	b.Frames = append(b.Frames, frame)
	b.CompressedData = append(b.CompressedData, compressed)
	b.IdxLineLengths = append(b.IdxLineLengths, idxLineLen)
	b.TotalBytes += len(compressed)
}

// Size returns the number of frames in the batch.
func (b *Batch) Size() int {
	return len(b.Frames)
}

// Empty returns true if the batch has no frames.
func (b *Batch) Empty() bool {
	return len(b.Frames) == 0
}

// Reset clears the batch for reuse.
func (b *Batch) Reset() {
	b.Frames = b.Frames[:0]
	b.CompressedData = b.CompressedData[:0]
	b.IdxLineLengths = b.IdxLineLengths[:0]
	b.TotalBytes = 0
}

// TotalIdxAdvance returns the total number of bytes to advance in the index file
// after successfully sending this batch.
func (b *Batch) TotalIdxAdvance() int64 {
	var total int64
	for _, l := range b.IdxLineLengths {
		total += int64(l)
	}
	return total
}

// LastFrame returns the last frame in the batch, or nil if empty.
func (b *Batch) LastFrame() *Frame {
	if len(b.Frames) == 0 {
		return nil
	}
	return &b.Frames[len(b.Frames)-1]
}
