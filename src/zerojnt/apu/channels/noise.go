// File: apu/channels/noise.go
package channels

// Noise channel period lookup table (NTSC values)
// Indexed by the lower 4 bits of register $400E.
var NoisePeriodTable = [16]uint16{
	4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 380, 508, 762, 1016, 2034, 4068,
} // These are CPU cycles / 2 (APU cycles)

// NoiseChannel represents the noise generator channel in the NES APU.
type NoiseChannel struct {
	enabled       bool
	mode          bool // False: 15-bit (mode 0), True: 7-bit (mode 1, period 93)
	shiftRegister uint16 // 15-bit linear feedback shift register (LFSR)

	// Timer/Period
	timerPeriod uint16 // Period reload value from NoisePeriodTable
	timerValue  uint16 // Current timer countdown value

	// Length Counter
	lengthCounter byte // Counts down to zero to silence channel
	lengthHalted  bool // Also envelope loop flag

	// Envelope Generator
	envelope EnvelopeUnit

	// Output cache
	lastOutput float32
}

// NewNoiseChannel creates and initializes a new NoiseChannel.
func NewNoiseChannel() *NoiseChannel {
	n := &NoiseChannel{}
	n.Reset()
	return n
}

// Reset initializes the noise channel to its power-up state.
func (n *NoiseChannel) Reset() {
	n.enabled = false
	n.mode = false
	n.shiftRegister = 1 // LFSR must be initialized to a non-zero value, 1 is standard.
	n.timerPeriod = NoisePeriodTable[0]
	n.timerValue = n.timerPeriod
	n.lengthCounter = 0
	n.lengthHalted = false
	n.envelope.Reset()
	n.lastOutput = 0.0
}

// WriteRegister handles writes to the noise channel's registers ($400C-$400F).
func (n *NoiseChannel) WriteRegister(addr uint16, value byte) {
	reg := addr & 3 // Register index (0-3)

	switch reg {
	case 0: // $400C: Length Halt, Envelope settings
		n.lengthHalted = (value & 0x20) != 0 // Bit 5: Length counter halt / Envelope loop
		n.envelope.loop = n.lengthHalted    // Envelope loop flag shares bit 5
		n.envelope.constant = (value & 0x10) != 0 // Bit 4: Constant volume / Envelope disable
		n.envelope.dividerPeriod = value & 0x0F    // Bits 0-3: Envelope period/divider reload value

	case 1: // $400D: Unused

	case 2: // $400E: Mode, Period select
		n.mode = (value & 0x80) != 0 // Bit 7: Mode flag
		periodIndex := value & 0x0F
		n.timerPeriod = NoisePeriodTable[periodIndex]

	case 3: // $400F: Length counter load, Envelope reset
		if n.enabled {
			n.lengthCounter = LengthTable[(value>>3)&0x1F] // Load length counter if channel is enabled
		}
		// Writing to $400F restarts the envelope
		n.envelope.start = true
	}
}

// ClockTimer advances the channel's timer by one APU clock cycle (half a CPU cycle).
// When the timer reaches zero, it reloads and clocks the shift register.
func (n *NoiseChannel) ClockTimer() {
	if n.timerValue == 0 {
		n.timerValue = n.timerPeriod // Reload timer
		n.clockShiftRegister()       // Clock the LFSR
	} else {
		n.timerValue--
	}
}

// clockShiftRegister advances the state of the 15-bit LFSR.
func (n *NoiseChannel) clockShiftRegister() {
	// Calculate feedback bit based on mode
	var feedbackBit uint16
	if n.mode { // Mode 1 (period 93 taps)
		// Feedback is XOR of bits 0 and 6
		feedbackBit = (n.shiftRegister & 0x0001) ^ ((n.shiftRegister >> 6) & 0x0001)
	} else { // Mode 0 (period 32767 taps)
		// Feedback is XOR of bits 0 and 1
		feedbackBit = (n.shiftRegister & 0x0001) ^ ((n.shiftRegister >> 1) & 0x0001)
	}

	// Shift the register right by 1
	n.shiftRegister >>= 1

	// Place the feedback bit into bit 14
	n.shiftRegister |= (feedbackBit << 14)
}

// ClockEnvelope advances the envelope generator state. Called by frame counter.
func (n *NoiseChannel) ClockEnvelope() {
	n.envelope.Clock()
}

// ClockLengthCounter advances the length counter state. Called by frame counter.
func (n *NoiseChannel) ClockLengthCounter() {
	if !n.lengthHalted && n.lengthCounter > 0 {
		n.lengthCounter--
	}
}

// SetEnabled enables or disables the channel.
func (n *NoiseChannel) SetEnabled(enabled bool) {
	n.enabled = enabled
	if !enabled {
		n.lengthCounter = 0 // Disabling clears the length counter
	}
}

// IsLengthCounterActive returns true if the length counter is non-zero. Used for $4015 status.
func (n *NoiseChannel) IsLengthCounterActive() bool {
	return n.lengthCounter > 0
}

// Output calculates the current audio sample based on the channel's state.
func (n *NoiseChannel) Output() float32 {
	// --- Muting conditions ---
	// 1. Channel disabled
	if !n.enabled {
		return 0.0
	}
	// 2. Length counter is zero
	if n.lengthCounter == 0 {
		return 0.0
	}
	// 3. LFSR bit 0 is 1 (output is 0 when bit 0 is 1)
	if (n.shiftRegister & 0x0001) != 0 {
		return 0.0
	}

	// --- Calculate Volume ---
	var volume byte
	if n.envelope.constant {
		volume = n.envelope.dividerPeriod // Use constant volume level
	} else {
		volume = n.envelope.decayLevel // Use envelope decay level
	}

	// Noise channel output is digital (either 0 or volume)
	output := float32(volume) / 15.0 // Normalize volume to 0.0 - 1.0

	// No smoothing usually applied to noise
	n.lastOutput = output

	return n.lastOutput
}