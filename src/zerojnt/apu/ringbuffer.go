package apu

type ringBuf struct {
	data []float32
	mask uint32
	// Padding to prevent false sharing on multi-core CPUs
	_        [8]uint64
	writeIdx uint32    // Producer (CPU thread) writes, consumer reads
	_        [8]uint64
	readIdx  uint32    // Consumer (Audio thread) writes, producer reads
	_        [8]uint64
}

// newRing creates a new ring buffer with size rounded up to the next power of two
func newRing(minSize int) *ringBuf {
	if minSize <= 0 {
		panic("ring buffer minimum size must be positive")
	}
	size := 1
	for size < minSize {
		size <<= 1 // Find next power of two
		// Prevent overflow
		if size <= 0 {
			panic("requested ring buffer size too large, caused overflow")
		}
	}

	return &ringBuf{
		data: make([]float32, size),
		mask: uint32(size - 1),
		// writeIdx and readIdx default to 0 atomically
	}
}