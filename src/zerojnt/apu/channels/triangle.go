// File: apu/channels/triangle.go
package channels

import (
	"math"
)

// Triangle waveform sequence (32 steps, values 0-15). Used for naive value calculation.
var TriangleSequence = [32]byte{
	15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
}

// Assumed to be defined elsewhere in the 'channels' package (e.g., pulse.go or lengthcounter.go)
// var LengthTable = []byte{ ... }

const (
	// Upsampling and Filtering Configuration
	UpsampleFactor = 4  // How many internal samples per output sample (2 or 4 recommended)
	CICStages      = 3  // Number of CIC stages (e.g., 3)
	FIRNumTaps     = 16 // Number of FIR taps (adjust based on design)

	phaseFullCycle   = uint32(0xFFFFFFFF) // Represents 1.0 in Q0.32
	phaseHalfCycle   = uint32(0x80000000) // Represents 0.5 in Q0.32
	cpuClockSpeed    = 1789773.0          // NTSC CPU Clock Speed (Needed for freq calc)
	outputSampleRate = 44100.0            // Target output sample rate (Needed for freq calc)
)

// Pre-calculated FIR coefficients for low-pass filter (Decimation by 4).
// These are placeholders - Replace with coefficients from a filter design tool
var firCoefficients = calculateExampleFirCoefficients(FIRNumTaps, UpsampleFactor)

// calculateExampleFirCoefficients generates simple placeholder coefficients.
// Replace this with actual filter design logic or precomputed coefficients.
func calculateExampleFirCoefficients(numTaps int, decimationFactor int) []float32 {
	coeffs := make([]float32, numTaps)
	// This creates a very basic boxcar/averaging filter - NOT ideal for audio.
	boxcarWidth := decimationFactor
	if boxcarWidth > numTaps {
		boxcarWidth = numTaps
	}
	coeffVal := 1.0 / float32(boxcarWidth)
	for i := 0; i < boxcarWidth; i++ {
		coeffs[i] = coeffVal
	}
	return coeffs
}

// TriangleChannel represents the triangle wave channel in the NES APU.
// Uses Upsampling, PolyBLEP, and CIC+FIR filtering for anti-aliasing.
type TriangleChannel struct {
	enabled bool

	// Timer/Period (Used to calculate frequency/phase increment)
	timerPeriod uint16 // Period reload value from registers $400A/$400B

	// Length Counter
	lengthCounter byte // Counts down to zero to silence channel
	lengthHalted  bool // Also linear counter control flag

	// Linear Counter (controls duration/volume override)
	linearCounter       byte // Counts down when clocked by frame counter
	linearReloadValue   byte // Value to reload linear counter from $4008
	linearReloadRequest bool // Flag to reload linear counter on next clock

	// Phase Accumulator for Upsampled Generation
	phaseAccumulator    uint32 // Q0.32 fixed-point phase accumulator
	phaseIncrement      uint32 // Phase step per *upsampled* internal clock
	recalculatePhaseInc bool   // Flag to recalculate phaseIncrement

	// Filtering State
	// CIC Filter State
	cicIntegrators [CICStages]int64 // Integrator stage accumulators
	// Using float64 for CIC accumulators/combs to avoid integer scaling issues
	cicIntegratorAcc [CICStages]float64                   // Integrator stage accumulators (float version)
	cicCombAcc       [CICStages]float64                   // Comb stage outputs (float version)
	cicDelay         [CICStages][UpsampleFactor]float64   // Delay lines for comb stages (float version)
	cicDelayIndex    [CICStages]int

	// FIR Filter State
	firBuffer     [FIRNumTaps]float32 // Delay line for FIR input
	firBufferIndex int

	// Keep track if muted to reset filters and prevent pops
	wasMuted bool
}

// NewTriangleChannel creates and initializes a new TriangleChannel.
func NewTriangleChannel() *TriangleChannel {
	t := &TriangleChannel{}
	t.Reset()
	// Ensure coefficients are calculated/available
	if len(firCoefficients) != FIRNumTaps {
		panic("FIR coefficient array size mismatch")
	}
	return t
}

// Reset initializes the triangle channel to its power-up state.
func (t *TriangleChannel) Reset() {
	t.enabled = false
	t.timerPeriod = 0
	t.lengthCounter = 0
	t.lengthHalted = false
	t.linearCounter = 0
	t.linearReloadValue = 0
	t.linearReloadRequest = false

	t.phaseAccumulator = 0
	t.phaseIncrement = 0
	t.recalculatePhaseInc = true

	// Reset filter states explicitly
	t.resetFilters()
	t.wasMuted = true // Assume starts muted
}

// resetFilters clears the state of CIC and FIR filters.
func (t *TriangleChannel) resetFilters() {
	for i := range t.cicIntegratorAcc {
		t.cicIntegratorAcc[i] = 0
		t.cicCombAcc[i] = 0
		t.cicDelayIndex[i] = 0
		for j := range t.cicDelay[i] {
			t.cicDelay[i][j] = 0
		}
	}
	for i := range t.firBuffer {
		t.firBuffer[i] = 0
	}
	t.firBufferIndex = 0
}

// WriteRegister handles writes to the triangle channel's registers ($4008-$400B).
func (t *TriangleChannel) WriteRegister(addr uint16, value byte) {
	reg := addr & 3 // Register index (0-3)

	switch reg {
	case 0: // $4008: Linear counter control, length counter halt
		t.lengthHalted = (value & 0x80) != 0 // Bit 7: Length halt / Linear control
		t.linearReloadValue = value & 0x7F   // Bits 0-6: Linear counter reload value

	case 1: // $4009: Unused

	case 2: // $400A: Timer low bits
		newPeriod := (t.timerPeriod & 0xFF00) | uint16(value)
		if newPeriod != t.timerPeriod {
			t.timerPeriod = newPeriod
			t.recalculatePhaseInc = true
		}

	case 3: // $400B: Length counter load, Timer high bits
		newPeriod := (t.timerPeriod & 0x00FF) | (uint16(value&0x07) << 8)
		if newPeriod != t.timerPeriod {
			t.timerPeriod = newPeriod
			t.recalculatePhaseInc = true
		}

		if t.enabled {
			// Use LengthTable assumed to be defined in the 'channels' package scope
			t.lengthCounter = LengthTable[(value>>3)&0x1F]
		}
		// Writing to $400B sets the linear counter reload flag
		t.linearReloadRequest = true
	}
}

// updatePhaseIncrement calculates the phase increment per upsampled sample.
func (t *TriangleChannel) updatePhaseIncrement() {
	if t.timerPeriod < 2 { // Periods 0 and 1 effectively halt or produce silence.
		t.phaseIncrement = 0
	} else {
		// Frequency (Hz) = CPU Clock / (32 * (Timer Period + 1))
		waveFreq := cpuClockSpeed / (32.0 * float64(t.timerPeriod+1))

		// Phase increment per *upsampled* sample
		phaseIncNormalized := waveFreq / (outputSampleRate * float64(UpsampleFactor))

		// Convert to Q0.32 fixed point
		t.phaseIncrement = uint32(phaseIncNormalized * math.Exp2(32))
	}
	t.recalculatePhaseInc = false
}

// ClockTimer - No longer used for sample generation timing. Keep for frame counter logic if needed.
func (t *TriangleChannel) ClockTimer() {
	// Superseded by phase accumulator logic in Output()
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
		t.linearCounter = 0 // Clear linear counter too? NESdev suggests yes.
	}
}

// IsLengthCounterActive returns true if the length counter is non-zero. Used for $4015 status.
func (t *TriangleChannel) IsLengthCounterActive() bool {
	return t.lengthCounter > 0
}

// Output generates one audio sample using upsampling, PolyBLEP, and filtering.
func (t *TriangleChannel) Output() float32 {
	// Muting Conditions
	muted := !t.enabled || t.linearCounter == 0 || t.lengthCounter == 0 || t.timerPeriod < 2

	if muted {
		if !t.wasMuted {
			// If we just became muted, reset filters to prevent pops when unmuted
			t.resetFilters()
			t.wasMuted = true
		}
		return 0.0
	}
	if t.wasMuted {
         // Could force phase reset here if needed on unmute, but usually letting it continue is fine.
         t.wasMuted = false
    }

	// Update Phase Increment if Needed
	if t.recalculatePhaseInc {
		t.updatePhaseIncrement()
		if t.phaseIncrement == 0 && t.timerPeriod >= 2 {
			 t.resetFilters()
			 t.wasMuted = true
			 return 0.0
		}
	}

	var finalSample float32 = 0.0

	// Upsampling Loop
	for i := 0; i < UpsampleFactor; i++ {
		// Calculate Naive Triangle Value
		seqIdx := (t.phaseAccumulator >> 27) & 31
		naiveValue := float32(TriangleSequence[seqIdx]) // 0 to 15
		normalizedNaive := (naiveValue - 7.5) / 7.5 // Approx -1.0 to 1.0

		// PolyBLEP Correction (Integrated BLEP simulation)
		// Use the polyBlepCorrection defined elsewhere in the package
		phaseRelativeToPeak := t.phaseAccumulator - phaseHalfCycle
		peakCorrection := polyBlepCorrection(phaseRelativeToPeak, t.phaseIncrement)
		blepCorrectedValue := normalizedNaive - 2.0*peakCorrection

		phaseRelativeToTrough := t.phaseAccumulator
		troughCorrection := polyBlepCorrection(phaseRelativeToTrough, t.phaseIncrement)
		blepCorrectedValue = blepCorrectedValue + 2.0*troughCorrection

		// Advance Phase Accumulator
		t.phaseAccumulator += t.phaseIncrement

        // CIC Filter - Using float64 accumulators
        integratedSample := float64(blepCorrectedValue) // Start with the input sample
		for stage := 0; stage < CICStages; stage++ {
			t.cicIntegratorAcc[stage] += integratedSample
			integratedSample = t.cicIntegratorAcc[stage] // Output of integrator is input to next stage
		}
		cicInputSample := integratedSample // Input to comb stage is output of last integrator

		// Decimation Point & CIC Filter - Comb Stage
		if (i+1)%UpsampleFactor == 0 { // Process on the last upsampled sample
			combSample := cicInputSample // Start with the output of the last integrator
			for stage := 0; stage < CICStages; stage++ {
				 delayedSample := t.cicDelay[stage][t.cicDelayIndex[stage]]
				 t.cicDelay[stage][t.cicDelayIndex[stage]] = combSample // Store current integrator output for future delay
				 t.cicCombAcc[stage] = combSample - delayedSample       // Comb filter operation: y[n] = x[n] - x[n-R]
				 combSample = t.cicCombAcc[stage]                       // Output of this stage is input to next
				 t.cicDelayIndex[stage] = (t.cicDelayIndex[stage] + 1) % UpsampleFactor
			}

            // Simplified Scaling Approach (Using float64 intermediate)
            // Gain = UpsampleFactor^CICStages
            cicGain := math.Pow(float64(UpsampleFactor), float64(CICStages))
            // Use the final comb output directly
            scaledCicOutput := float32(t.cicCombAcc[CICStages-1] / cicGain)

			// FIR Filter Stage
			t.firBuffer[t.firBufferIndex] = scaledCicOutput

			firOutput := float32(0.0)
			readIdx := t.firBufferIndex
			for tap := 0; tap < FIRNumTaps; tap++ {
				firOutput += t.firBuffer[readIdx] * firCoefficients[tap]
				readIdx--
				if readIdx < 0 {
					readIdx = FIRNumTaps - 1 // Wrap around buffer
				}
			}

			t.firBufferIndex = (t.firBufferIndex + 1) % FIRNumTaps
			finalSample = firOutput // This is our decimated, filtered output sample
		}
	}

    // Rescale filtered output (-1 to 1) back to the mixer's expected 0 to 1 range.
	outputForMixer := (finalSample + 1.0) * 0.5
	if outputForMixer < 0.0 { outputForMixer = 0.0 }
	if outputForMixer > 1.0 { outputForMixer = 1.0 }

	return outputForMixer
}