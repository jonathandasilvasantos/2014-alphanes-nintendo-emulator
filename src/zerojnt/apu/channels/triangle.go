package channels

// TriangleSequence is the 32-step waveform output (values 0-15)
var TriangleSequence = [32]byte{
	15, 14, 13, 12, 11, 10, 9, 8, // High to Mid
	7, 6, 5, 4, 3, 2, 1, 0, // Mid to Low
	0, 1, 2, 3, 4, 5, 6, 7, // Low to Mid
	8, 9, 10, 11, 12, 13, 14, 15, // Mid to High
}

// TriangleChannel implements the NES triangle wave channel
type TriangleChannel struct {
	enabled bool

	// Timer
	timerPeriod uint16
	timerValue  uint16

	// Sequencer
	sequenceCounter byte

	// Length Counter
	lengthCounter byte
	lengthHalted  bool

	// Linear Counter
	linearCounter   byte
	linearReloadVal byte
	linearReloadReq bool
}

// NewTriangleChannel returns a new triangle channel instance
func NewTriangleChannel(cpuClock float64) *TriangleChannel {
	t := &TriangleChannel{}
	t.Reset()
	return t
}

// Reset puts the channel in its initial state
func (t *TriangleChannel) Reset() {
	t.enabled = false
	t.timerPeriod = 0
	t.timerValue = 0
	t.sequenceCounter = 0
	t.lengthCounter = 0
	t.lengthHalted = false
	t.linearCounter = 0
	t.linearReloadVal = 0
	t.linearReloadReq = false
}

// WriteRegister handles APU register writes
func (t *TriangleChannel) WriteRegister(addr uint16, value byte) {
	switch addr & 0x03 {
	case 0: // $4008 - Linear Counter Control / Length Halt
		t.lengthHalted = (value & 0x80) != 0
		t.linearReloadVal = value & 0x7F
	case 1: // $4009 - Unused
		// no-op
	case 2: // $400A - Timer Low 8 bits
		t.timerPeriod = (t.timerPeriod & 0xFF00) | uint16(value)
	case 3: // $400B - Length Counter Load / Timer High 3 bits / Linear Counter Reload trigger
		t.timerPeriod = (t.timerPeriod & 0x00FF) | (uint16(value&0x07) << 8)

		// Load Length Counter if channel is enabled
		if t.enabled {
			t.lengthCounter = LengthTable[(value>>3)&0x1F]
		}

		t.linearReloadReq = true
	}
}

// ClockTimer advances the triangle channel's internal timer
func (t *TriangleChannel) ClockTimer() {
	// Only run if both counters are non-zero
	if t.linearCounter == 0 || t.lengthCounter == 0 {
		return
	}

	if t.timerValue > 0 {
		t.timerValue--
	} else {
		t.timerValue = t.timerPeriod
		t.sequenceCounter = (t.sequenceCounter + 1) % 32
	}
}

// ClockLinearCounter updates the linear counter state
func (t *TriangleChannel) ClockLinearCounter() {
    // 1. Reload or decrement
    if t.linearReloadReq {
        t.linearCounter = t.linearReloadVal
    } else if t.linearCounter > 0 {
        t.linearCounter--
    }
    // 2. Clear reload request if length-halt flag is 0
    if !t.lengthHalted {
        t.linearReloadReq = false
    }
}

// ClockLengthCounter updates the length counter state
func (t *TriangleChannel) ClockLengthCounter() {
	if !t.lengthHalted && t.lengthCounter > 0 {
		t.lengthCounter--
	}
}

// SetEnabled enables or disables the channel
func (t *TriangleChannel) SetEnabled(enabled bool) {
	t.enabled = enabled
	if !enabled {
		t.lengthCounter = 0
	}
}

// IsLengthCounterActive returns true if the length counter is non-zero
func (t *TriangleChannel) IsLengthCounterActive() bool {
	return t.lengthCounter > 0
}

// Output returns the current sample value
func (t *TriangleChannel) Output() float32 {
	if !t.enabled || t.lengthCounter == 0 || t.linearCounter == 0 || t.timerPeriod < 2 {
		return 0.0
	}

	sampleValue := TriangleSequence[t.sequenceCounter]
	return float32(sampleValue) / 15.0
}