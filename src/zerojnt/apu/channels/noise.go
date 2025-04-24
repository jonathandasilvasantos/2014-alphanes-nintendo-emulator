// File: apu/channels/noise.go
package channels

// Noise channel period lookup table (NTSC values)
var NoisePeriodTable = [16]uint16{
	4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 380, 508, 762, 1016, 2034, 4068,
}

// NoiseChannel represents the noise generator channel in the NES APU.
type NoiseChannel struct {
	enabled       bool
	mode          bool
	shiftRegister uint16

	timerPeriod uint16
	timerValue  uint16

	lengthCounter byte
	lengthHalted  bool

	envelope EnvelopeUnit

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
	n.shiftRegister = 1
	n.timerPeriod = NoisePeriodTable[0]
	n.timerValue = n.timerPeriod
	n.lengthCounter = 0
	n.lengthHalted = false
	n.envelope.Reset()
	n.lastOutput = 0.0
}

// WriteRegister handles writes to the noise channel's registers ($400C-$400F).
func (n *NoiseChannel) WriteRegister(addr uint16, value byte) {
	reg := addr & 3

	switch reg {
	case 0: // $400C: Length Halt, Envelope settings
		n.lengthHalted = (value & 0x20) != 0
		n.envelope.loop = n.lengthHalted
		n.envelope.constant = (value & 0x10) != 0
		n.envelope.dividerPeriod = value & 0x0F

	case 1: // $400D: Unused

	case 2: // $400E: Mode, Period select
		n.mode = (value & 0x80) != 0
		periodIndex := value & 0x0F
		n.timerPeriod = NoisePeriodTable[periodIndex]

	case 3: // $400F: Length counter load, Envelope reset
		if n.enabled {
			n.lengthCounter = LengthTable[(value>>3)&0x1F]
		}
		n.envelope.start = true
	}
}

// ClockTimer advances the channel's timer by one APU clock cycle.
func (n *NoiseChannel) ClockTimer() {
	if n.timerValue == 0 {
		n.timerValue = n.timerPeriod
		n.clockShiftRegister()
	} else {
		n.timerValue--
	}
}

// clockShiftRegister advances the state of the LFSR.
func (n *NoiseChannel) clockShiftRegister() {
	var feedbackBit uint16
	if n.mode {
		feedbackBit = (n.shiftRegister & 0x0001) ^ ((n.shiftRegister >> 6) & 0x0001)
	} else {
		feedbackBit = (n.shiftRegister & 0x0001) ^ ((n.shiftRegister >> 1) & 0x0001)
	}

	n.shiftRegister >>= 1
	n.shiftRegister |= (feedbackBit << 14)
}

// ClockEnvelope advances the envelope generator state.
func (n *NoiseChannel) ClockEnvelope() {
	n.envelope.Clock()
}

// ClockLengthCounter advances the length counter state.
func (n *NoiseChannel) ClockLengthCounter() {
	if !n.lengthHalted && n.lengthCounter > 0 {
		n.lengthCounter--
	}
}

// SetEnabled enables or disables the channel.
func (n *NoiseChannel) SetEnabled(enabled bool) {
	n.enabled = enabled
	if !enabled {
		n.lengthCounter = 0
	}
}

// IsLengthCounterActive returns true if the length counter is non-zero.
func (n *NoiseChannel) IsLengthCounterActive() bool {
	return n.lengthCounter > 0
}

// Output calculates the current audio sample based on the channel's state.
func (n *NoiseChannel) Output() float32 {
	if !n.enabled {
		return 0.0
	}
	if n.lengthCounter == 0 {
		return 0.0
	}
	if (n.shiftRegister & 0x0001) != 0 {
		return 0.0
	}

	var volume byte
	if n.envelope.constant {
		volume = n.envelope.dividerPeriod
	} else {
		volume = n.envelope.decayLevel
	}

	output := float32(volume) / 15.0
	n.lastOutput = output

	return n.lastOutput
}