// File: apu/channels/triangle.go
package channels

import "math"

// TriangleSequence is the 32-step linear counter output (values 0-15)
var TriangleSequence = [32]byte{
	15, 14, 13, 12, 11, 10, 9, 8,
	7, 6, 5, 4, 3, 2, 1, 0,
	0, 1, 2, 3, 4, 5, 6, 7,
	8, 9, 10, 11, 12, 13, 14, 15,
}

// Tuning constants for triangle channel
const (
	UpsampleFactor = 4
	CICStages      = 3
	FIRNumTaps     = 16

	phaseBits = 32
	phaseMask = uint32(31 << (phaseBits - 5))

	cpuClockSpeed = 1789773.0
	outputRate    = 44100.0
)

// FIR low-pass coefficients with unity DC gain
var firCoefficients = func() [FIRNumTaps]float32 {
	var out [FIRNumTaps]float32
	box := float32(1.0 / UpsampleFactor)
	for i := 0; i < UpsampleFactor && i < FIRNumTaps; i++ {
		out[i] = box
	}
	return out
}()

// TriangleChannel implements the NES triangle wave channel
type TriangleChannel struct {
	// Control registers
	enabled         bool
	timerPeriod     uint16
	lengthCounter   byte
	lengthHalted    bool
	linearCounter   byte
	linearReloadVal byte
	linearReloadReq bool

	// Phase generation
	phaseAcc        uint32
	phaseInc        uint32
	needPhaseRecalc bool

	// CIC filter state
	cicInt       [CICStages]int64
	cicCombDelay [CICStages][UpsampleFactor]int64
	cicIdx       [CICStages]uint8

	// FIR filter state
	firBuf [FIRNumTaps]float32
	firIdx int

	wasMuted bool
}

// NewTriangleChannel returns a new instance
func NewTriangleChannel() *TriangleChannel {
	t := &TriangleChannel{}
	t.Reset()
	return t
}

// Reset puts the channel in power-on state
func (t *TriangleChannel) Reset() {
	t.enabled = false
	t.timerPeriod = 0
	t.lengthCounter = 0
	t.lengthHalted = false
	t.linearCounter = 0
	t.linearReloadVal = 0
	t.linearReloadReq = false

	t.phaseAcc = 0
	t.phaseInc = 0
	t.needPhaseRecalc = true

	t.resetFilters()
	t.wasMuted = true
}

// resetFilters initializes the CIC/FIR filter state
func (t *TriangleChannel) resetFilters() {
	for i := range t.cicInt {
		t.cicInt[i] = 0
		t.cicIdx[i] = 0
		for j := 0; j < UpsampleFactor; j++ {
			t.cicCombDelay[i][j] = 0
		}
	}
	for i := 0; i < FIRNumTaps; i++ {
		t.firBuf[i] = 0
	}
	t.firIdx = 0
}

// WriteRegister handles APU register writes ($4008-$400B)
func (t *TriangleChannel) WriteRegister(addr uint16, value byte) {
	switch addr & 0x03 {
	case 0: // $4008 - Linear Counter Control / Length Halt
		t.lengthHalted = (value & 0x80) != 0
		t.linearReloadVal = value & 0x7F
	case 1: // $4009 - Unused
		// no-op
	case 2: // $400A - Timer Low
		newPeriod := (t.timerPeriod & 0xFF00) | uint16(value)
		if newPeriod != t.timerPeriod {
			t.timerPeriod = newPeriod
			t.needPhaseRecalc = true
		}
	case 3: // $400B - Length Counter Load / Timer High / Linear Counter Reload
		newPeriod := (t.timerPeriod & 0x00FF) | (uint16(value&0x07) << 8)
		if newPeriod != t.timerPeriod {
			t.timerPeriod = newPeriod
			t.needPhaseRecalc = true
		}

		if t.enabled {
			t.lengthCounter = LengthTable[(value>>3)&0x1F]
		}

		t.linearReloadReq = true
	}
}

// ClockLinearCounter updates linear counter state
func (t *TriangleChannel) ClockLinearCounter() {
	if t.linearReloadReq {
		t.linearCounter = t.linearReloadVal
	} else if t.linearCounter > 0 {
		t.linearCounter--
	}

	if !t.lengthHalted {
		t.linearReloadReq = false
	}
}

// ClockLengthCounter updates length counter state
func (t *TriangleChannel) ClockLengthCounter() {
	if !t.lengthHalted && t.lengthCounter > 0 {
		t.lengthCounter--
	}
}

// SetEnabled sets channel enable flag
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

// Output generates one filtered audio sample
func (t *TriangleChannel) Output() float32 {
	muted := !t.enabled || t.linearCounter == 0 || t.lengthCounter == 0 || t.timerPeriod < 2

	if muted {
		if !t.wasMuted {
			t.resetFilters()
			t.wasMuted = true
		}
		return 0.0
	}
	t.wasMuted = false

	if t.needPhaseRecalc {
		t.recalcPhaseInc()
		t.needPhaseRecalc = false
	}

	if t.phaseInc == 0 {
		return 0.0
	}

	var finalSample float32 = 0.0

	for u := 0; u < UpsampleFactor; u++ {
		sequenceIndex := (t.phaseAcc & phaseMask) >> (phaseBits - 5)
		rawValue := float32(TriangleSequence[sequenceIndex])

		normalizedSample := (rawValue - 7.5) / 7.5

		normalizedSample -= 2.0 * polyBlep(t.phaseAcc, t.phaseInc)
		normalizedSample += 2.0 * polyBlep(t.phaseAcc+phaseHalf(), t.phaseInc)

		t.phaseAcc += t.phaseInc

		cicInput := int64(normalizedSample * 32768.0)
		integratorOutput := cicInput
		for stage := 0; stage < CICStages; stage++ {
			t.cicInt[stage] += integratorOutput
			integratorOutput = t.cicInt[stage]
		}

		if u == (UpsampleFactor - 1) {
			combInput := integratorOutput
			combOutput := combInput
			for stage := 0; stage < CICStages; stage++ {
				combIndex := t.cicIdx[stage]
				delayedSample := t.cicCombDelay[stage][combIndex]
				t.cicCombDelay[stage][combIndex] = combOutput
				combOutput -= delayedSample
				t.cicIdx[stage] = (combIndex + 1) & (UpsampleFactor - 1)
			}

			cicGain := int64(1)
			for i := 0; i < CICStages; i++ {
				cicGain *= int64(UpsampleFactor)
			}
			normalizedComb := float32(combOutput) / float32(cicGain * 32768.0)

			t.firBuf[t.firIdx] = normalizedComb

			firOutput := float32(0.0)
			firTapIndex := t.firIdx
			for i := 0; i < FIRNumTaps; i++ {
				firOutput += t.firBuf[firTapIndex] * firCoefficients[i]
				firTapIndex--
				if firTapIndex < 0 {
					firTapIndex = FIRNumTaps - 1
				}
			}

			t.firIdx++
			if t.firIdx >= FIRNumTaps {
				t.firIdx = 0
			}

			finalSample = firOutput
		}
	}

	outputForMixer := (finalSample + 1.0) * 0.5
	if outputForMixer < 0.0 {
		outputForMixer = 0.0
	} else if outputForMixer > 1.0 {
		outputForMixer = 1.0
	}

	return outputForMixer
}

// recalcPhaseInc calculates the phase increment per upsampled step
func (t *TriangleChannel) recalcPhaseInc() {
	if t.timerPeriod < 2 {
		t.phaseInc = 0
		return
	}

	triangleFreq := cpuClockSpeed / (32.0 * (float64(t.timerPeriod + 1)))
	t.phaseInc = uint32((triangleFreq / (outputRate * float64(UpsampleFactor))) * math.Exp2(phaseBits))
}

// polyBlep calculates the polynomial correction factor for BLEP synthesis
func polyBlep(phase uint32, phaseInc uint32) float32 {
	if phaseInc == 0 {
		return 0.0
	}

	t := float32(phase & 0x7FFFFFFF) / float32(0x80000000)
	dt := float32(phaseInc) / float32(0xFFFFFFFF)

	if t < dt {
		return (t/dt - 1.0)
	}

	if t > (1.0 - dt) {
		return ((t - 1.0) / dt) + 1.0
	}

	return 0.0
}

// phaseHalf returns the value representing half the phase cycle
func phaseHalf() uint32 {
	return 0x80000000
}