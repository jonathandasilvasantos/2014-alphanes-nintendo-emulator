// File: apu/channels/triangle.go
package channels

// TriangleChannel represents the triangle wave channel in the NES APU
type TriangleChannel struct {
	enabled       bool
	period        uint16
	lengthCount   byte
	linearCount   byte
	linearReload  byte
	linearControl bool
	sequencePos   byte
	timer         uint16
	reloadFlag    bool
	sequence      [32]byte
	lastSample    float32 // For smooth transitions
	outputLevel   float32 // For volume control
}

// NewTriangleChannel creates and initializes a new TriangleChannel.
func NewTriangleChannel() *TriangleChannel {
	t := &TriangleChannel{
		enabled:     false,
		sequencePos: 0,
		reloadFlag:  false,
		lastSample:  0.0,
		outputLevel: 1.0,
	}

	// Initialize the triangle sequence (15, 14, 13,...0, 0, 1, 2,...15)
	for i := 0; i < 16; i++ {
		t.sequence[i] = byte(15 - i)
		t.sequence[31-i] = byte(i)
	}

	return t
}

// WriteRegister handles writes to the triangle channel's registers.
func (t *TriangleChannel) WriteRegister(addr uint16, value byte) {
	switch addr {
	case 0x4008: // Linear counter load and control flag
		t.linearReload = value & 0x7F
		t.linearControl = value&0x80 != 0
		t.reloadFlag = true
	case 0x400A: // Period low
		t.period = (t.period & 0xFF00) | uint16(value)
		// Reset timer if it's out of sync with the new period
		if t.timer > t.period {
			t.timer = t.period
		}
	case 0x400B: // Length counter load and period high
		t.period = (t.period & 0x00FF) | (uint16(value&0x07) << 8)
		if t.enabled {
			t.lengthCount = LengthTable[value>>3]
		}
		t.reloadFlag = true
		t.timer = t.period // Reset timer on period high write
	}
}

// ClockTimer clocks the triangle channel's timer and advances the sequence.
func (t *TriangleChannel) ClockTimer() {
	if !t.enabled {
		t.outputLevel *= 0.995 // Smooth fade out when disabled (adjustable factor)
		return
	}

	// Ultrasonic frequencies (period < 2) are inaudible and muted.
	if t.period < 2 {
		t.outputLevel = 0
		return
	}

	// Only advance sequence if both length counter and linear counter are > 0.
	if t.lengthCount > 0 && t.linearCount > 0 {
		if t.timer == 0 {
			t.timer = t.period
			t.sequencePos = (t.sequencePos + 1) & 0x1F // Advance sequence (wraps around at 32)
			t.outputLevel = 1.0 // Restore full volume when active
		} else {
			t.timer--
		}
	} else {
		t.outputLevel *= 0.995 // Smooth fade when counters expire (adjustable factor)
	}
}

// ClockLinearCounter clocks the linear counter.
func (t *TriangleChannel) ClockLinearCounter() {
	if t.reloadFlag {
		t.linearCount = t.linearReload
	} else if t.linearCount > 0 {
		t.linearCount--
	}

	if !t.linearControl {
		t.reloadFlag = false
	}
}

// ClockLengthCounter clocks the length counter.
func (t *TriangleChannel) ClockLengthCounter() {
	if t.enabled && t.lengthCount > 0 && !t.linearControl {
		t.lengthCount--
	}
}

// GetSample returns the current output sample of the triangle channel.
func (t *TriangleChannel) GetSample() float32 {
	if !t.enabled || t.lengthCount == 0 || t.linearCount == 0 || t.period < 2 {
		// Smooth fade to silence
		t.lastSample *= 0.995 // Adjustable smoothing factor
		return t.lastSample
	}

	// Get the raw sequence value
	raw := float32(t.sequence[t.sequencePos])

	// Scale to 0.0 - 0.8 range (avoid clipping)
	targetSample := raw / 15.0 * 0.8

	// Apply smooth transition between samples
	t.lastSample = t.lastSample*0.7 + targetSample*0.3 // Adjustable smoothing factors

	// Apply output level for volume control
	finalSample := t.lastSample * t.outputLevel

	// Scale to 0.0 - 1.0 range with soft clipping
	finalSample = (finalSample + 1.0) * 0.5

	// Soft clipping to reduce harshness (adjustable thresholds)
	if finalSample > 0.95 {
		finalSample = 0.95 + (finalSample-0.95)*0.05
	} else if finalSample < 0.05 {
		finalSample = 0.05 + (finalSample-0.05)*0.05
	}

	return finalSample
}

// SetEnabled enables or disables the triangle channel.
func (t *TriangleChannel) SetEnabled(enabled bool) {
	if t.enabled != enabled {
		t.enabled = enabled
		if !enabled {
			t.lengthCount = 0
			t.linearCount = 0
			t.outputLevel = 0.0 // Fade out when disabled
			t.lastSample = 0.0  // Reset last sample
		} else {
			t.outputLevel = 1.0 // Restore volume when enabled
		}
	}
}


// Clock clocks the triangle channel's components.
func (t *TriangleChannel) Clock() {
	t.ClockTimer()
}

// IsEnabled returns whether the triangle channel is currently enabled.
func (t *TriangleChannel) IsEnabled() bool {
	return t.enabled
}

// Reset resets the triangle channel to its initial state.
func (t *TriangleChannel) Reset() {
	t.enabled = false
	t.period = 0
	t.lengthCount = 0
	t.linearCount = 0
	t.linearReload = 0
	t.linearControl = false
	t.sequencePos = 0
	t.timer = 0
	t.reloadFlag = false
	t.lastSample = 0.0
	t.outputLevel = 0.0
}