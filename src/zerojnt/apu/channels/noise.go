// File: apu/channels/noise.go
package channels

// NOISE_PERIOD_TABLE defines the NTSC periods for the noise channel.
var NOISE_PERIOD_TABLE = [16]uint16{
	4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 380, 508, 762, 1016, 2034, 4068,
}

// NoiseChannel represents the noise generator channel in the NES APU.
type NoiseChannel struct {
	enabled       bool
	mode          bool          // Determines the length of the shift register sequence (mode 0 = long, mode 1 = short)
	shiftRegister uint16        // The shift register used to generate the noise
	period        uint16        // Current period (from NOISE_PERIOD_TABLE)
	timer         uint16        // Timer to determine when to clock the shift register
	volume        byte          // Fixed volume (if not using envelope)
	lengthCount   byte          // Length counter to control note duration
	envelope      EnvelopeUnit  // Envelope generator for volume control
	useEnvelope   bool          // Flag to indicate whether to use envelope or fixed volume
	lastSample    float32       // For smoothing the output
}

// NewNoiseChannel creates and initializes a new NoiseChannel.
func NewNoiseChannel() *NoiseChannel {
	return &NoiseChannel{
		enabled:       false,
		mode:          false,
		shiftRegister: 1,     // Initialized to 1
		period:        0,
		timer:         0,
		volume:        15,
		lengthCount:   0,
		envelope: EnvelopeUnit{
			value:      15,
			divider:    0,
			counter:    0,
			decayLevel: 15,
		},
		useEnvelope: false,
		lastSample:  0.0,
	}
}

// WriteRegister handles writes to the noise channel's registers.
func (n *NoiseChannel) WriteRegister(addr uint16, value byte) {
	switch addr & 3 {
	case 0: // $400C: Envelope and length counter halt
		n.useEnvelope = (value & 0x10) == 0
		if n.useEnvelope {
			n.envelope.value = value & 0x0F
			n.envelope.SetStart(true)
		} else {
			n.volume = value & 0x0F
		}
		n.envelope.loop = (value & 0x20) != 0
	case 2: // $400E: Period and mode
		n.mode = (value & 0x80) != 0
		periodIndex := value & 0x0F
		n.period = NOISE_PERIOD_TABLE[periodIndex]
	case 3: // $400F: Length counter load and envelope restart
		if n.enabled {
			n.lengthCount = LengthTable[value>>3]
		}
		n.envelope.SetStart(true)
	}
}

// Clock advances the noise channel's internal state (timer and shift register).
func (n *NoiseChannel) Clock() {
	if n.timer > 0 {
		n.timer--
	}

	if n.timer == 0 {
		n.timer = n.period

		// Calculate feedback based on the mode
		var feedback uint16
		if n.mode {
			// Short mode (93-bit sequence): XOR bits 6 and 0
			feedback = ((n.shiftRegister >> 6) & 0x01) ^ (n.shiftRegister & 0x01)
		} else {
			// Long mode (32767-bit sequence): XOR bits 1 and 0
			feedback = ((n.shiftRegister >> 1) & 0x01) ^ (n.shiftRegister & 0x01)
		}

		// Update the shift register
		n.shiftRegister = (n.shiftRegister >> 1) | (feedback << 14)
	}
}

// GetSample returns the current output sample for the noise channel.
func (n *NoiseChannel) GetSample() float32 {
	if !n.enabled || n.lengthCount == 0 {
		// Smoothly fade to silence
		n.lastSample *= 0.99
		return n.lastSample
	}

	var targetSample float32
	// Output is based on the last bit of the shift register.
	if (n.shiftRegister & 0x01) == 0 {
		if n.useEnvelope {
			targetSample = float32(n.envelope.decayLevel) / 15.0
		} else {
			targetSample = float32(n.volume) / 15.0
		}
	} else {
		targetSample = 0.0
	}

	// Smooth the transition between samples
	n.lastSample = n.lastSample*0.7 + targetSample*0.3

	return n.lastSample
}

// SetEnabled enables or disables the noise channel.
func (n *NoiseChannel) SetEnabled(enabled bool) {
	if n.enabled != enabled {
		n.enabled = enabled
		if !enabled {
			n.lengthCount = 0
			n.lastSample = 0.0 // Reset sample on disable
		}
	}
}

// ClockLengthCounter decrements the length counter if enabled and not halted.
func (n *NoiseChannel) ClockLengthCounter() {
	if n.enabled && n.lengthCount > 0 && !n.envelope.loop {
		n.lengthCount--
	}
}

// ClockEnvelope clocks the envelope unit.
func (n *NoiseChannel) ClockEnvelope() {
	if n.useEnvelope {
		n.envelope.Clock()
	}
}

// IsEnabled returns whether the noise channel is currently enabled.
func (n *NoiseChannel) IsEnabled() bool {
	return n.enabled
}

// Reset initializes the noise channel to its default power-up state
func (n *NoiseChannel) Reset() {
	n.enabled = false
	n.mode = false
	n.shiftRegister = 1 // Initialized to 1
	n.period = 0
	n.timer = 0
	n.volume = 15
	n.lengthCount = 0
	n.envelope.Reset()
	n.useEnvelope = false
	n.lastSample = 0.0
}