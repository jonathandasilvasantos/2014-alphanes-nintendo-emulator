// File: apu/channels/triangle.go
package channels

// Triangle waveform sequence (32 steps, values 0-15)
var TriangleSequence = [32]byte{
	15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
}

// TriangleChannel represents the triangle wave channel in the NES APU.
type TriangleChannel struct {
	enabled bool

	// Timer/Period
	timerPeriod uint16 // Period reload value from registers $400A/$400B
	timerValue  uint16 // Current timer countdown value

	// Length Counter
	lengthCounter byte // Counts down to zero to silence channel
	lengthHalted  bool // Also linear counter control flag

	// Linear Counter (controls duration/volume override)
	linearCounter       byte // Counts down when clocked by frame counter
	linearReloadValue   byte // Value to reload linear counter from $4008
	linearReloadRequest bool // Flag to reload linear counter on next clock

	// Sequencer
	sequencePosition byte // Current step in the 32-step TriangleSequence (0-31)

	// Output cache
	lastOutput float32
}

// NewTriangleChannel creates and initializes a new TriangleChannel.
func NewTriangleChannel() *TriangleChannel {
	t := &TriangleChannel{}
	t.Reset()
	return t
}

// Reset initializes the triangle channel to its power-up state.
func (t *TriangleChannel) Reset() {
	t.enabled = false
	t.timerPeriod = 0
	t.timerValue = 0
	t.lengthCounter = 0
	t.lengthHalted = false
	t.linearCounter = 0
	t.linearReloadValue = 0
	t.linearReloadRequest = false
	t.sequencePosition = 0
	t.lastOutput = 0.0
}

// WriteRegister handles writes to the triangle channel's registers ($4008-$400B).
func (t *TriangleChannel) WriteRegister(addr uint16, value byte) {
	reg := addr & 3 // Register index (0-3, though only 0, 2, 3 are used)

	switch reg {
	case 0: // $4008: Linear counter control, length counter halt
		t.lengthHalted = (value & 0x80) != 0 // Bit 7: Length halt / Linear control
		t.linearReloadValue = value & 0x7F   // Bits 0-6: Linear counter reload value

	case 1: // $4009: Unused

	case 2: // $400A: Timer low bits
		t.timerPeriod = (t.timerPeriod & 0xFF00) | uint16(value)

	case 3: // $400B: Length counter load, Timer high bits
		t.timerPeriod = (t.timerPeriod & 0x00FF) | (uint16(value&0x07) << 8)
		if t.enabled {
			t.lengthCounter = LengthTable[(value>>3)&0x1F] // Load length counter if channel is enabled
		}
		// Writing to $400B sets the linear counter reload flag
		t.linearReloadRequest = true
	}
}

// ClockTimer advances the channel's timer by one CPU cycle.
// When the timer reaches zero, it reloads and advances the waveform sequencer position,
// but only if both the linear counter and length counter are non-zero.
func (t *TriangleChannel) ClockTimer() {
	if !t.enabled { // Should we clock timer if disabled? NESDev implies yes.
		// return // Let's allow timer to clock even if disabled
	}

	// Don't clock timer if period is too low (prevents excessive sequence advancement)
	// Some sources say period 0 or 1 can cause issues. Period 0 effectively halts.
	if t.timerPeriod == 0 {
		return
	}

	if t.timerValue == 0 {
		t.timerValue = t.timerPeriod // Reload timer
		// Advance sequence ONLY if linear and length counters are active
		if t.linearCounter > 0 && t.lengthCounter > 0 {
			t.sequencePosition = (t.sequencePosition + 1) % 32
		}
	} else {
		t.timerValue--
	}
}

// ClockLinearCounter advances the linear counter state. Called by frame counter.
func (t *TriangleChannel) ClockLinearCounter() {
	if t.linearReloadRequest {
		t.linearCounter = t.linearReloadValue // Reload the counter
	} else if t.linearCounter > 0 {
		t.linearCounter-- // Decrement if not reloading
	}

	// If the control flag (length halt bit) is clear, clear the reload request flag
	if !t.lengthHalted {
		t.linearReloadRequest = false
	}
}

// ClockLengthCounter advances the length counter state. Called by frame counter.
func (t *TriangleChannel) ClockLengthCounter() {
	if !t.lengthHalted && t.lengthCounter > 0 {
		t.lengthCounter--
	}
}

// SetEnabled enables or disables the channel.
func (t *TriangleChannel) SetEnabled(enabled bool) {
	t.enabled = enabled
	if !enabled {
		t.lengthCounter = 0 // Disabling clears the length counter
		t.linearCounter = 0 // Disabling might also clear the linear counter? Unclear, let's clear it.
	}
}

// IsLengthCounterActive returns true if the length counter is non-zero. Used for $4015 status.
func (t *TriangleChannel) IsLengthCounterActive() bool {
	return t.lengthCounter > 0
}

// Output calculates the current audio sample based on the channel's state.
// The triangle channel outputs its sequence value directly (0-15).
func (t *TriangleChannel) Output() float32 {
	// Muting conditions:
	// 1. Channel disabled (handled by APU potentially, or check here?) Let's check here.
	if !t.enabled {
		return 0.0
	}
	// 2. Linear counter is zero OR Length counter is zero
	if t.linearCounter == 0 || t.lengthCounter == 0 {
		return 0.0
	}
	// 3. Timer period too low? Some sources say < 2 mutes.
	if t.timerPeriod < 2 { // Let's mute for periods 0 and 1.
		return 0.0
	}

	// Get the sample value from the sequence table
	sampleValue := TriangleSequence[t.sequencePosition]

	// Normalize to 0.0 - 1.0 range
	output := float32(sampleValue) / 15.0

	// Smoothing? Triangle wave is pure, maybe no smoothing needed?
	// Let's skip smoothing for simplicity/purity unless artifacts are bad.
	// t.lastOutput = output * 0.1 + t.lastOutput * 0.9 // Example smoothing
	t.lastOutput = output // No smoothing for now

	return t.lastOutput
}