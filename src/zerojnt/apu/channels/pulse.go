// File: apu/channels/pulse.go
package channels

import (
	"math"
)

// Duty cycle width definitions
var dutyCycleValues = [4]float64{
	0.125, // 12.5 % (00000001)
	0.25,  // 25 %  (00000011)
	0.50,  // 50 %  (00001111)
	0.25,  // DUTY 3 uses 25% width (mirrored)
}

// PulseChannel represents a pulse wave channel in the NES APU
type PulseChannel struct {
	channelNum int
	enabled    bool

	dutyMode     byte
	lengthHalted bool
	timerPeriod  uint16

	phaseAccumulator uint32
	phaseIncrement   uint32
	dutyThreshold    uint32

	lengthCounter byte

	envelope EnvelopeUnit
	sweep    SweepUnit

	recalculatePhaseInc bool

	cpuClockSpeed float64
	sampleRate    float64
}

// NewPulseChannel creates and initializes a new PulseChannel
func NewPulseChannel(channelNum int, cpuClock float64, sampleRate float64) *PulseChannel {
	p := &PulseChannel{
		channelNum:          channelNum,
		sweep:               *NewSweepUnit(),
		recalculatePhaseInc: true,
		timerPeriod:         1,
		cpuClockSpeed:       cpuClock,
		sampleRate:          sampleRate,
	}
	p.sweep.channelNum = channelNum
	p.Reset()
	return p
}

// Reset initializes the pulse channel to its power-up state
func (p *PulseChannel) Reset() {
	p.enabled = false
	p.lengthHalted = false
	p.dutyMode = 0
	p.timerPeriod = 1
	p.lengthCounter = 0
	p.envelope.Reset()
	p.sweep.Reset()

	p.phaseAccumulator = 0
	p.phaseIncrement = 0
	p.dutyThreshold = 0
	p.recalculatePhaseInc = true
}

// WriteRegister handles writes to the pulse channel's registers
func (p *PulseChannel) WriteRegister(addr uint16, value byte) {
	reg := addr & 3

	switch reg {
	case 0: // Duty, Length Halt, Envelope settings
		newDutyMode := (value >> 6) & 3
		if newDutyMode != p.dutyMode {
			p.dutyMode = newDutyMode
			p.recalculatePhaseInc = true
		}
		p.lengthHalted = (value & 0x20) != 0
		p.envelope.loop = p.lengthHalted
		p.envelope.constant = (value & 0x10) != 0
		p.envelope.dividerPeriod = value & 0x0F

	case 1: // Sweep unit control
		p.sweep.Write(value)
		p.recalculatePhaseInc = true

	case 2: // Timer low bits
		newPeriod := (p.timerPeriod & 0xFF00) | uint16(value)
		if newPeriod != p.timerPeriod {
			p.timerPeriod = newPeriod
			p.recalculatePhaseInc = true
		}

	case 3: // Length counter load, Timer high bits, Envelope reset
		newPeriod := (p.timerPeriod & 0x00FF) | (uint16(value&0x07) << 8)
		if newPeriod != p.timerPeriod {
			p.timerPeriod = newPeriod
			p.recalculatePhaseInc = true
		}

		if p.enabled {
			p.lengthCounter = LengthTable[(value>>3)&0x1F]
		}
		p.envelope.start = true

		p.sweep.reload = true
		p.recalculatePhaseInc = true
	}
}

// updatePhaseIncrement calculates the phase increment and duty threshold
func (p *PulseChannel) updatePhaseIncrement() {
	effectivePeriod := float64(p.timerPeriod + 1)
	if effectivePeriod < 1e-9 || p.timerPeriod >= 0x7FF {
		p.phaseIncrement = 0
	} else {
		freq := p.cpuClockSpeed / (16.0 * effectivePeriod)
		phaseIncNormalized := freq / p.sampleRate
		p.phaseIncrement = uint32(phaseIncNormalized * math.Exp2(32))
	}

	duty := dutyCycleValues[p.dutyMode]
	p.dutyThreshold = uint32(duty * math.Exp2(32))

	p.recalculatePhaseInc = false
}

// ClockTimer - phase accumulation happens in Output()
func (p *PulseChannel) ClockTimer() {
	// No implementation needed
}

// ClockEnvelope advances the envelope generator state
func (p *PulseChannel) ClockEnvelope() {
	p.envelope.Clock()
}

// ClockLengthCounter advances the length counter state
func (p *PulseChannel) ClockLengthCounter() {
	if !p.lengthHalted && p.lengthCounter > 0 {
		p.lengthCounter--
	}
}

// ClockSweep advances the sweep unit state and updates the timer period
func (p *PulseChannel) ClockSweep() {
	p.sweep.targetPeriod = p.timerPeriod
	newPeriod := p.sweep.Clock(p.timerPeriod)
	if newPeriod != p.timerPeriod {
		if newPeriod > 0x7FF {
			newPeriod = 0x7FF
		}
		p.timerPeriod = newPeriod
		p.recalculatePhaseInc = true
	}
}

// SetEnabled enables or disables the channel
func (p *PulseChannel) SetEnabled(enabled bool) {
	p.enabled = enabled
	if !enabled {
		p.lengthCounter = 0
	}
}

// IsLengthCounterActive returns true if the length counter is non-zero
func (p *PulseChannel) IsLengthCounterActive() bool {
	return p.lengthCounter > 0
}

// Output calculates the current audio sample using PolyBLEP
func (p *PulseChannel) Output() float32 {
	if !p.enabled || p.lengthCounter == 0 {
		return 0.0
	}

	if p.recalculatePhaseInc {
		p.updatePhaseIncrement()
	}

	if p.timerPeriod < 8 || p.phaseIncrement == 0 || p.sweep.isMuting(p.timerPeriod) {
		return 0.0
	}

	naiveOut := float32(0.0)
	high := p.phaseAccumulator < p.dutyThreshold
	if p.dutyMode == 3 {
		high = !high // Mirror for duty mode 3
	}
	if high {
		naiveOut = 1.0
	}

	correction0 := polyBlepCorrection(p.phaseAccumulator, p.phaseIncrement)
	phaseRelativeToDuty := p.phaseAccumulator - p.dutyThreshold
	correctionDuty := polyBlepCorrection(phaseRelativeToDuty, p.phaseIncrement)

	var combinedOut float32
	if p.dutyMode == 3 {
		combinedOut = naiveOut - correction0 + correctionDuty
	} else {
		combinedOut = naiveOut + correction0 - correctionDuty
	}

	p.phaseAccumulator += p.phaseIncrement

	var volume byte
	if p.envelope.constant {
		volume = p.envelope.dividerPeriod
	} else {
		volume = p.envelope.decayLevel
	}

	finalOutput := combinedOut * (float32(volume) / 15.0)

	return finalOutput
}