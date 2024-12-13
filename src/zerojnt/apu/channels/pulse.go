// File: apu/channels/pulse.go
package channels

var LengthTable = []byte{
	10, 254, 20, 2, 40, 4, 80, 6,
	160, 8, 60, 10, 14, 12, 26, 14,
	12, 16, 24, 18, 48, 20, 96, 22,
	192, 24, 72, 26, 16, 28, 32, 30,
}

// Duty cycle table - defines the wave pattern for each duty cycle
var DutyTable = [4][8]byte{
	{0, 1, 0, 0, 0, 0, 0, 0}, // 12.5%
	{0, 1, 1, 0, 0, 0, 0, 0}, // 25%
	{0, 1, 1, 1, 1, 0, 0, 0}, // 50%
	{1, 0, 0, 1, 1, 1, 1, 1}, // 75% (inverted 25%)
}

// PulseChannel represents a pulse wave channel in the NES APU
type PulseChannel struct {
	enabled       bool
	dutyMode      byte
	dutyPos       byte // Current position in the duty cycle
	volume        byte // Fixed volume (if not using envelope)
	period        uint16
	lengthCount   byte
	timer         uint16
	envelope      EnvelopeUnit
	useEnvelope   bool
	sweep         SweepUnit
	lastSample    float32 // For smoothing
	outputLevel   float32 // Current output level (for volume control and muting)
	sweepMuting   bool    // Indicates whether sweep is currently muting the channel
}

// NewPulseChannel creates and initializes a new PulseChannel.
func NewPulseChannel() *PulseChannel {
	return &PulseChannel{
		enabled:     false,
		dutyMode:    0,
		volume:      15,
		period:      0,
		lastSample:  0.0,
		outputLevel: 1.0, // Start with full volume
		envelope: EnvelopeUnit{
			value:      15,
			divider:    0,
			counter:    0,
			decayLevel: 15,
		},
		sweep: *NewSweepUnit(),
	}
}

// WriteRegister handles writes to the pulse channel's registers.
func (p *PulseChannel) WriteRegister(addr uint16, value byte) {
	switch addr & 3 {
	case 0: // $4000/$4004: Duty, Length Counter Halt, Envelope Settings
		p.dutyMode = (value >> 6) & 3
		p.envelope.loop = (value & 0x20) != 0
		p.envelope.constant = (value & 0x10) != 0
		p.volume = value & 0x0F
		p.envelope.value = p.volume // Set divider reload value
		p.envelope.SetStart(true)   // Restart the envelope
	case 1: // $4001/$4005: Sweep Unit
		p.sweep.Write(value)
		p.updateSweepMutingStatus()
	case 2: // $4002/$4006: Timer Low
		p.period = (p.period & 0xFF00) | uint16(value)
		p.updateSweepMutingStatus()
	case 3: // $4003/$4007: Length Counter Load, Timer High, Envelope Start
		p.period = (p.period & 0x00FF) | (uint16(value&7) << 8)
		if p.enabled {
			p.lengthCount = LengthTable[value>>3]
		}
		p.dutyPos = 0 // Reset duty cycle position
		p.envelope.SetStart(true)
		p.updateSweepMutingStatus()
	}
}

// updateSweepMutingStatus updates the sweep muting flag based on the current period.
func (p *PulseChannel) updateSweepMutingStatus() {
	p.sweepMuting = p.period < 8 || p.period > 0x7FF
}

// Clock clocks the pulse channel's internal components.
func (p *PulseChannel) Clock() {
	if p.timer > 0 {
		p.timer--
	} else {
		p.timer = p.period // Reset timer
		p.dutyPos = (p.dutyPos + 1) & 7
	}
}

// GetSample returns the current output sample for the pulse channel.
func (p *PulseChannel) GetSample() float32 {
	if !p.enabled || p.lengthCount == 0 || p.sweepMuting {
		// Smoothly fade out when muted or disabled
		p.outputLevel *= 0.995
		p.lastSample *= 0.995
		return p.lastSample
	}

	// Get the current output from the duty cycle, applying volume and envelope
	var targetSample float32
	if DutyTable[p.dutyMode][p.dutyPos] != 0 {
		if p.envelope.constant {
			targetSample = float32(p.volume)
		} else {
			targetSample = float32(p.envelope.decayLevel)
		}
	} else {
		targetSample = 0
	}
	targetSample /= 15.0 // Normalize to 0.0 - 1.0 range

	// Apply smoothing with previous sample
	p.lastSample = p.lastSample*0.7 + targetSample*0.3

	// Apply soft clipping to prevent harshness
	if p.lastSample > 0.95 {
		p.lastSample = 0.95 + (p.lastSample-0.95)*0.05
	} else if p.lastSample < 0.05 {
		p.lastSample = 0.05 + (p.lastSample-0.05)*0.05
	}

	return p.lastSample
}

// SetEnabled enables or disables the pulse channel.
func (p *PulseChannel) SetEnabled(enabled bool) {
	if p.enabled != enabled {
		p.enabled = enabled
		if !enabled {
			p.lengthCount = 0
			p.outputLevel = 0.0 // Immediately mute when disabled
		}
	}
}

// ClockLengthCounter clocks the length counter, decrementing if enabled and not halted.
func (p *PulseChannel) ClockLengthCounter() {
	if p.enabled && p.lengthCount > 0 && !p.envelope.loop {
		p.lengthCount--
	}
}

// ClockEnvelope clocks the envelope unit.
func (p *PulseChannel) ClockEnvelope() {
	p.envelope.Clock()
}

// ClockSweep clocks the sweep unit, adjusting the period if enabled.
func (p *PulseChannel) ClockSweep() {
	if p.sweep.counter == 0 && p.sweep.enabled && p.sweep.shift > 0 && !p.sweepMuting {
		newPeriod := p.period
		change := p.period >> p.sweep.shift
		if p.sweep.negate {
			newPeriod -= change
			// Special case for pulse 2
			if p.sweep.negate {
				newPeriod -= 1
			}
		} else {
			newPeriod += change
		}

		if newPeriod >= 8 && newPeriod <= 0x7FF {
			p.period = newPeriod
		}
	}

	if p.sweep.counter > 0 {
		p.sweep.counter--
	}

	if p.sweep.reload {
		p.sweep.counter = p.sweep.divider
		p.sweep.reload = false
	}
}

// IsEnabled returns whether the pulse channel is currently enabled.
func (p *PulseChannel) IsEnabled() bool {
	return p.enabled
}


// Reset initializes the pulse channel to its default power-up state
func (p *PulseChannel) Reset() {
	p.enabled = false
	p.dutyMode = 0
	p.dutyPos = 0
	p.volume = 15
	p.period = 0
	p.lengthCount = 0
	p.timer = 0
	p.envelope.Reset()
	p.sweep.Reset()
	p.lastSample = 0.0
	p.outputLevel = 0.0
	p.sweepMuting = false
}