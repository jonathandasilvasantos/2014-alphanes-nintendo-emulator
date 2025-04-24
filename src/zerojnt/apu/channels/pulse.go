// File: apu/channels/pulse.go
package channels

import (
	"math"
	// Removed to break import cycle
)

// Length counter lookup table (values are halt flags/counter load values)
// Shared by pulse, triangle, noise channels
var LengthTable = []byte{
	10, 254, 20, 2, 40, 4, 80, 6,
	160, 8, 60, 10, 14, 12, 26, 14,
	12, 16, 24, 18, 48, 20, 96, 22,
	192, 24, 72, 26, 16, 28, 32, 30,
}

// Duty cycle width definitions (fraction of the period)
var dutyCycleValues = [4]float64{
	0.125, // 12.5%
	0.25,  // 25%
	0.50,  // 50%
	0.75,  // 75% (often implemented as inverted 25%)
}

// PulseChannel represents a pulse wave channel in the NES APU, using PolyBLEP for band-limiting.
type PulseChannel struct {
	channelNum int // 1 or 2, used for sweep negate behavior
	enabled    bool

	// Register Values
	dutyMode     byte   // 0-3, selects duty cycle from dutyCycleValues
	lengthHalted bool   // Also envelope loop flag
	timerPeriod  uint16 // Period reload value from registers $4002/$4003

	// Phase Accumulator (Q0.32 fixed point, wraps at 2^32)
	// Represents phase from 0.0 to 1.0 (exclusive)
	phaseAccumulator uint32
	phaseIncrement   uint32 // Amount to increment phaseAccumulator per audio sample

	// Duty cycle threshold in phase accumulator terms (Q0.32)
	// Pre-calculated based on dutyMode for efficiency
	dutyThreshold uint32

	// Length Counter
	lengthCounter byte // Counts down to zero to silence channel

	// Envelope Generator
	envelope EnvelopeUnit

	// Sweep Unit
	sweep SweepUnit

	// State Flags
	recalculatePhaseInc bool // Flag to update phaseIncrement and dutyThreshold

	// System Constants (Passed Down)
	cpuClockSpeed float64 // Store CPU clock speed locally
	sampleRate    float64 // Store audio sample rate locally
}

// NewPulseChannel creates and initializes a new PulseChannel.
// Accepts cpuClock and sampleRate to avoid import cycle with parent apu package.
func NewPulseChannel(channelNum int, cpuClock float64, sampleRate float64) *PulseChannel {
	p := &PulseChannel{
		channelNum:          channelNum,
		sweep:               *NewSweepUnit(), // Initialize sweep unit
		recalculatePhaseInc: true,            // Ensure phase inc is calculated initially
		timerPeriod:         1,               // Avoid divide by zero if used before write
		cpuClockSpeed:       cpuClock,        // Store passed CPU clock speed
		sampleRate:          sampleRate,      // Store passed sample rate
	}
	p.sweep.channelNum = channelNum // Link sweep unit to this channel
	p.Reset()                       // Set initial state including phase calculations
	return p
}

// Reset initializes the pulse channel to its power-up state.
func (p *PulseChannel) Reset() {
	p.enabled = false
	p.lengthHalted = false
	p.dutyMode = 0
	p.timerPeriod = 1 // Use 1 to avoid potential div by zero in initial phase calc
	p.lengthCounter = 0
	p.envelope.Reset()
	p.sweep.Reset()

	p.phaseAccumulator = 0
	p.phaseIncrement = 0 // Will be calculated on first Output() or write
	p.dutyThreshold = 0
	p.recalculatePhaseInc = true // Mark for recalculation
}

// WriteRegister handles writes to the pulse channel's registers ($4000-$4003 or $4004-$4007).
func (p *PulseChannel) WriteRegister(addr uint16, value byte) {
	reg := addr & 3 // Register index (0-3)

	switch reg {
	case 0: // $4000 / $4004: Duty, Length Halt, Envelope settings
		newDutyMode := (value >> 6) & 3
		if newDutyMode != p.dutyMode {
			p.dutyMode = newDutyMode
			p.recalculatePhaseInc = true // Duty threshold changes
		}
		p.lengthHalted = (value & 0x20) != 0 // Bit 5: Length counter halt / Envelope loop
		p.envelope.loop = p.lengthHalted    // Envelope loop flag shares bit 5
		p.envelope.constant = (value & 0x10) != 0 // Bit 4: Constant volume / Envelope disable
		p.envelope.dividerPeriod = value & 0x0F    // Bits 0-3: Envelope period/divider reload value

	case 1: // $4001 / $4005: Sweep unit control
		p.sweep.Write(value)
		p.recalculatePhaseInc = true // Sweep settings change might affect target period calculation/muting

	case 2: // $4002 / $4006: Timer low bits
		newPeriod := (p.timerPeriod & 0xFF00) | uint16(value)
		if newPeriod != p.timerPeriod {
			p.timerPeriod = newPeriod
			p.recalculatePhaseInc = true // Period changes
		}

	case 3: // $4003 / $4007: Length counter load, Timer high bits, Envelope reset
		newPeriod := (p.timerPeriod & 0x00FF) | (uint16(value&0x07) << 8)
		if newPeriod != p.timerPeriod {
			p.timerPeriod = newPeriod
			p.recalculatePhaseInc = true // Period changes
		}

		if p.enabled {
			// Use the LengthTable defined within this package
			p.lengthCounter = LengthTable[(value>>3)&0x1F] // Load length counter if channel is enabled
		}
		// Writing to $4003/$4007 restarts the envelope and resets the duty phase
		p.envelope.start = true
		// Resetting duty phase is now implicit in the phase accumulator, no direct reset needed.

		// Also reloads the sweep timer
		p.sweep.reload = true
		p.recalculatePhaseInc = true // Sweep reload affects next target calc
	}
}

// updatePhaseIncrement calculates the phase increment per sample and duty threshold
// based on the current timer period and duty mode.
// Uses the stored cpuClockSpeed and sampleRate fields.
func (p *PulseChannel) updatePhaseIncrement() {
	// Prevent division by zero or excessively high frequencies if period is too low.
	effectivePeriod := float64(p.timerPeriod + 1)
	if effectivePeriod < 1e-9 || p.timerPeriod >= 0x7FF {
		p.phaseIncrement = 0
	} else {
		// Frequency (Hz) = CPU Clock / (16 * (Timer Period + 1))
		// Use stored constants instead of importing apu
		freq := p.cpuClockSpeed / (16.0 * effectivePeriod)

		// Phase increment per sample (normalized 0.0 to 1.0) = freq / sample_rate
		// Use stored constants instead of importing apu
		phaseIncNormalized := freq / p.sampleRate

		// Convert to Q0.32 fixed point (range 0 to 2^32 - 1 represents one cycle)
		p.phaseIncrement = uint32(phaseIncNormalized * math.Exp2(32))
	}

	// Calculate duty threshold based on current duty mode
	duty := dutyCycleValues[p.dutyMode]
	if p.dutyMode == 3 { // 75% duty uses 25% threshold internally
		duty = dutyCycleValues[1]
	}
	p.dutyThreshold = uint32(duty * math.Exp2(32))

	p.recalculatePhaseInc = false // Mark as updated
}

// polyBlepCorrection calculates the polynomial correction factor to subtract
// from a naive step function at a discontinuity point.
// phase: Current phase value (0 to 2^32 - 1) relative to the discontinuity.
// phaseInc: Phase increment per sample.
// Returns the correction value.
func polyBlepCorrection(phase uint32, phaseInc uint32) float32 {
	if phaseInc == 0 {
		return 0.0
	}

	dt_norm := float64(phaseInc) / math.Exp2(32)
	phase_norm := float64(phase) / math.Exp2(32)

	var correction float32 = 0.0

	if phase_norm < dt_norm { // Crossed 0 from below (potential rising edge)
		t_over_dt := phase_norm / dt_norm
		correction = float32(t_over_dt - t_over_dt*t_over_dt/2.0 - 0.5)
	} else if phase_norm > 1.0-dt_norm { // Approaching 0 from above (potential falling edge)
		t_minus_1_over_dt := (phase_norm - 1.0) / dt_norm
		correction = float32((t_minus_1_over_dt*t_minus_1_over_dt / 2.0) + t_minus_1_over_dt + 0.5)
	}

	// This function calculates the correction to *add* for a discontinuity at phase 0.
	// The calling function (Output) determines whether to add or subtract based on the edge type.
	return correction
}

// ClockTimer: Phase accumulation happens in Output(), so this is currently a no-op
// for the PolyBLEP implementation. Keep it for potential future compatibility or refactoring.
func (p *PulseChannel) ClockTimer() {
	// Intentionally left blank for PolyBLEP implementation.
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
	// Get the target period from the sweep unit BEFORE clocking it
	p.sweep.targetPeriod = p.timerPeriod
	// Clock the sweep unit, which returns the possibly updated period
	newPeriod := p.sweep.Clock(p.timerPeriod)
	// Apply the new period if it changed
	if newPeriod != p.timerPeriod {
		// Apply clamping / range checks
		if newPeriod > 0x7FF {
			// Sweep unit's isMuting check handles muting for > 0x7FF target,
			// but the actual period register likely saturates or wraps in hardware.
			// Let's clamp here for safety, though muting is the primary effect.
			newPeriod = 0x7FF
		}
		// Note: Period < 8 check is handled in Output/isMuting

		p.timerPeriod = newPeriod
		p.recalculatePhaseInc = true // Period changed, need to update phase increment
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

// Output calculates the current audio sample based on the channel's state using PolyBLEP.
func (p *PulseChannel) Output() float32 {
	// Muting conditions
	if !p.enabled || p.lengthCounter == 0 {
		return 0.0
	}

	if p.recalculatePhaseInc {
		p.updatePhaseIncrement()
	}

	if p.timerPeriod < 8 || p.phaseIncrement == 0 || p.sweep.isMuting(p.timerPeriod) {
		return 0.0
	}

	// Generate PolyBLEP Sample
	naiveOut := float32(0.0)
	is75Duty := (p.dutyMode == 3)
	currentDutyThreshold := p.dutyThreshold // For 75% duty, this holds the 25% threshold

	if is75Duty {
		if p.phaseAccumulator >= currentDutyThreshold {
			naiveOut = 1.0
		}
	} else {
		if p.phaseAccumulator < currentDutyThreshold {
			naiveOut = 1.0
		}
	}

	// Calculate PolyBLEP corrections
	correction0 := polyBlepCorrection(p.phaseAccumulator, p.phaseIncrement)
	phaseRelativeToDuty := p.phaseAccumulator - currentDutyThreshold // Wraps correctly due to uint32
	correctionDuty := polyBlepCorrection(phaseRelativeToDuty, p.phaseIncrement)

	// Combine naive output and corrections
	var combinedOut float32
	if is75Duty {
		// 75% duty: Rising edge at duty (25%), falling edge at phase 0
		combinedOut = naiveOut + correctionDuty - correction0
	} else {
		// 12.5, 25, 50% duty: Rising edge at phase 0, falling edge at duty
		combinedOut = naiveOut + correction0 - correctionDuty
	}

	// Advance phase accumulator
	p.phaseAccumulator += p.phaseIncrement

	// Apply Volume Envelope
	var volume byte
	if p.envelope.constant {
		volume = p.envelope.dividerPeriod
	} else {
		volume = p.envelope.decayLevel
	}

	// Apply volume
	finalOutput := combinedOut * (float32(volume) / 15.0)

	return finalOutput
}