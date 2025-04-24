// File: apu/mixer.go
package apu

// Mixer handles combining the audio channel outputs using canonical non-linear formulas.
type Mixer struct {
	// Configuration
	masterVolume float32 // Master volume control (typically 0.0 to 1.0)

	// DC Blocking Filter configuration
	// Alpha close to 1.0 means slow adaptation (less low-frequency filtering).
	// Value around 0.995 is common.
	dcBlockingAlpha float32

	// State
	dcOffset float32 // Running average of the signal for DC blocking
}

// NewMixer creates and initializes a mixer with canonical settings.
func NewMixer() *Mixer {
	return &Mixer{
		// Configuration:
		masterVolume:    1.0,   // Start at full volume. Adjust down slightly (e.g., 0.9) if clipping occurs.
		dcBlockingAlpha: 0.995, // Standard DC blocking coefficient

		// State:
		dcOffset: 0.0, // Initialize DC offset estimate
	}
}

// MixChannels combines the outputs of the individual APU channels using non-linear formulas.
// Inputs (p1, p2, tri, noise, dmc) should be normalized floats (0.0 to 1.0).
// DMC input corresponds to its 0-127 level normalized to 0.0-1.0.
func (m *Mixer) MixChannels(p1, p2, tri, noise, dmc float32) float32 {

	// Calculate Non-linear Pulse Mix
	// Use float64 for intermediate calculations to preserve precision.
	pulseSum := float64(p1 + p2) // Max value 2.0
	var pulseOut float64
	if pulseSum > 1e-9 { // Epsilon check to avoid division by zero / instability
		// Original formula: 95.88 / (8128 / (P1 + P2) + 100)
		// Adapted for normalized inputs p1, p2 (where P1=15*p1, P2=15*p2):
		// Denominator = 8128 / (15 * (p1 + p2)) + 100 = (8128/15) / (p1+p2) + 100
		// Constant = 8128.0 / 15.0 = 541.8666...
		pulseOut = 95.88 / ((541.8666666666667 / pulseSum) + 100.0)
	}

	// Calculate Non-linear Triangle/Noise/DMC Mix
	// Inputs tri, noise are 0.0-1.0 (from 0-15). Input dmc is 0.0-1.0 (from 0-127).
	var tndOut float64
	// Original: 1 / (T/8227 + N/12241 + D/22638)
	// Adapted: 1 / ( (15*tri)/8227 + (15*noise)/12241 + (127*dmc)/22638 )
	// T_Term = tri / (8227/15) = tri / 548.4666...
	// N_Term = noise / (12241/15) = noise / 816.0666...
	// D_Term = dmc / (22638/127) = dmc / 178.2519...
	tTerm := float64(tri) / 548.4666666666667
	nTerm := float64(noise) / 816.0666666666667
	dTerm := float64(dmc) / 178.251968503937 // DMC placeholder term

	tndDenominatorSum := tTerm + nTerm + dTerm

	if tndDenominatorSum > 1e-9 { // Epsilon check
		// Original formula: 159.79 / (1 / TND_Sum + 100)
		tndOut = 159.79 / ((1.0 / tndDenominatorSum) + 100.0)
	}

	// Combine Mixes
	// The final mix is a simple sum of the two mixer outputs.
	mixRaw := float32(pulseOut + tndOut) // Convert back to float32

	// Apply DC Blocking Filter (High-pass filter)
	// This remains the same, prevents DC offset buildup.
	m.dcOffset = m.dcOffset*m.dcBlockingAlpha + mixRaw*(1.0-m.dcBlockingAlpha)
	mixFiltered := mixRaw - m.dcOffset

	// Apply Master Volume
	mixFinal := mixFiltered * m.masterVolume

	// Optional: Hard Clipping
	// The non-linear formulas should prevent most clipping, but this is a safety measure.
	// Enable if needed for problematic ROMs or if masterVolume > 1 is used.
	/*
	   if mixFinal > 1.0 {
	       mixFinal = 1.0
	   } else if mixFinal < -1.0 {
	       mixFinal = -1.0
	   }
	*/

	return mixFinal
}