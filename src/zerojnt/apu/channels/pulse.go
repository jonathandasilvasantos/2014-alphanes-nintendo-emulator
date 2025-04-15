// File: apu/channels/pulse.go
package channels

// Length counter lookup table (values are halt flags/counter load values)
var LengthTable = []byte{
	10, 254, 20, 2, 40, 4, 80, 6,
	160, 8, 60, 10, 14, 12, 26, 14,
	12, 16, 24, 18, 48, 20, 96, 22,
	192, 24, 72, 26, 16, 28, 32, 30,
}

// Duty cycle waveforms (8 steps per cycle)
var DutyTable = [4][8]byte{
	{0, 1, 0, 0, 0, 0, 0, 0}, // 12.5% ( _-______ )
	{0, 1, 1, 0, 0, 0, 0, 0}, // 25%   ( _--_____ )
	{0, 1, 1, 1, 1, 0, 0, 0}, // 50%   ( _----___ )
	{1, 0, 0, 1, 1, 1, 1, 1}, // 75%   ( -__----- ) (inverted 25%)
}

// PulseChannel represents a pulse wave channel in the NES APU.
type PulseChannel struct {
	channelNum    int // 1 or 2, used for sweep negate behavior
	enabled       bool
	lengthHalted  bool // Also envelope loop flag
	dutyMode      byte // 0-3, selects waveform from DutyTable
	dutyPosition  byte // Current step in the 8-step duty cycle (0-7)

	// Timer/Period
	timerPeriod uint16 // Period reload value from registers $4002/$4003
	timerValue  uint16 // Current timer countdown value

	// Length Counter
	lengthCounter byte // Counts down to zero to silence channel

	// Envelope Generator
	envelope EnvelopeUnit

	// Sweep Unit
	sweep SweepUnit

	// Output cache for mixer
	lastOutput float32
}

// NewPulseChannel creates and initializes a new PulseChannel.
func NewPulseChannel(channelNum int) *PulseChannel {
	p := &PulseChannel{
		channelNum: channelNum,
		sweep:      *NewSweepUnit(), // Initialize sweep unit
	}
	p.sweep.channelNum = channelNum // Link sweep unit to this channel
	p.Reset()
	return p
}

// Reset initializes the pulse channel to its power-up state.
func (p *PulseChannel) Reset() {
	p.enabled = false
	p.lengthHalted = false
	p.dutyMode = 0
	p.dutyPosition = 0
	p.timerPeriod = 0
	p.timerValue = 0
	p.lengthCounter = 0
	p.envelope.Reset()
	p.sweep.Reset()
	p.lastOutput = 0.0
}

// WriteRegister handles writes to the pulse channel's registers ($4000-$4003 or $4004-$4007).
func (p *PulseChannel) WriteRegister(addr uint16, value byte) {
	reg := addr & 3 // Register index (0-3)

	switch reg {
	case 0: // $4000 / $4004: Duty, Length Halt, Envelope settings
		p.dutyMode = (value >> 6) & 3
		p.lengthHalted = (value & 0x20) != 0 // Bit 5: Length counter halt / Envelope loop
		p.envelope.loop = p.lengthHalted    // Envelope loop flag shares bit 5
		p.envelope.constant = (value & 0x10) != 0 // Bit 4: Constant volume / Envelope disable
		p.envelope.dividerPeriod = value & 0x0F    // Bits 0-3: Envelope period/divider reload value

	case 1: // $4001 / $4005: Sweep unit control
		p.sweep.Write(value)

	case 2: // $4002 / $4006: Timer low bits
		p.timerPeriod = (p.timerPeriod & 0xFF00) | uint16(value)

	case 3: // $4003 / $4007: Length counter load, Timer high bits, Envelope reset
		p.timerPeriod = (p.timerPeriod & 0x00FF) | (uint16(value&0x07) << 8)
		if p.enabled {
			p.lengthCounter = LengthTable[(value>>3)&0x1F] // Load length counter if channel is enabled
		}
		// Writing to $4003/$4007 restarts the envelope and resets the duty phase
		p.envelope.start = true
		p.dutyPosition = 0
		// Also reloads the sweep timer
		p.sweep.reload = true
	}
	// Update sweep target period whenever timer period changes
	p.sweep.targetPeriod = p.timerPeriod
}

// ClockTimer advances the channel's timer by one APU clock cycle (which is half a CPU cycle).
// When the timer reaches zero, it reloads and advances the duty cycle position.
func (p *PulseChannel) ClockTimer() {
	if p.timerValue == 0 {
		p.timerValue = p.timerPeriod // Reload timer
		p.dutyPosition = (p.dutyPosition + 1) % 8 // Advance duty cycle step
	} else {
		p.timerValue--
	}
}

// ClockEnvelope advances the envelope generator state. Called by frame counter.
func (p *PulseChannel) ClockEnvelope() {
	p.envelope.Clock()
}

// ClockLengthCounter advances the length counter state. Called by frame counter.
func (p *PulseChannel) ClockLengthCounter() {
	if !p.lengthHalted && p.lengthCounter > 0 {
		p.lengthCounter--
	}
}

// ClockSweep advances the sweep unit state and potentially updates the timer period. Called by frame counter.
func (p *PulseChannel) ClockSweep() {
	// Update the sweep unit's target period based on the channel's current period
	p.sweep.targetPeriod = p.timerPeriod
	// Clock the sweep unit, which returns the possibly updated period
	newPeriod := p.sweep.Clock(p.timerPeriod)
	// Apply the new period if it changed
	if newPeriod != p.timerPeriod {
		p.timerPeriod = newPeriod
		p.sweep.targetPeriod = newPeriod // Ensure sweep target matches
	}
}

// SetEnabled enables or disables the channel.
func (p *PulseChannel) SetEnabled(enabled bool) {
	p.enabled = enabled
	if !enabled {
		p.lengthCounter = 0 // Disabling clears the length counter
	}
}

// IsLengthCounterActive returns true if the length counter is non-zero. Used for $4015 status.
func (p *PulseChannel) IsLengthCounterActive() bool {
	return p.lengthCounter > 0
}

// Output calculates the current audio sample based on the channel's state.
func (p *PulseChannel) Output() float32 {
	// --- Muting conditions ---
	// 1. Channel disabled
	if !p.enabled {
		p.lastOutput = 0.0 // Gradually fade out disabled channels? Or instant mute? Let's do instant.
		return 0.0
	}
	// 2. Length counter is zero
	if p.lengthCounter == 0 {
		p.lastOutput = 0.0
		return 0.0
	}
	// 3. Timer period is invalid (< 8 results in silence)
	if p.timerPeriod < 8 {
		p.lastOutput = 0.0
		return 0.0
	}
	// 4. Sweep unit is muting (target period > $7FF)
	if p.sweep.isMuting(p.timerPeriod) {
		p.lastOutput = 0.0
		return 0.0
	}
	// 5. Duty cycle output is 0
	if DutyTable[p.dutyMode][p.dutyPosition] == 0 {
		p.lastOutput = 0.0
		return 0.0
	}

	// --- Calculate Volume ---
	var volume byte
	if p.envelope.constant {
		volume = p.envelope.dividerPeriod // Use constant volume level
	} else {
		volume = p.envelope.decayLevel // Use envelope decay level
	}

	// --- Smoothing (optional, can reduce clicking) ---
	// Simple low-pass filter: output = current * alpha + last * (1 - alpha)
	targetOutput := float32(volume) / 15.0 // Normalize volume to 0.0 - 1.0
	p.lastOutput = targetOutput*0.25 + p.lastOutput*0.75 // Adjust alpha as needed

	return p.lastOutput
}