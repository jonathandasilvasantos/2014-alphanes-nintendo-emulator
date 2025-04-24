// File: apu/mixer.go
package apu

// Mixer combines audio channel outputs using non-linear formulas
type Mixer struct {
	masterVolume    float32 // Master volume control (0.0 to 1.0)
	dcBlockingAlpha float32 // DC blocking filter coefficient
	dcOffset        float32 // Running average for DC blocking
}

// NewMixer creates and initializes a mixer with default settings
func NewMixer() *Mixer {
	return &Mixer{
		masterVolume:    1.0,
		dcBlockingAlpha: 0.995,
		dcOffset:        0.0,
	}
}

// MixChannels combines the outputs of individual APU channels
// Inputs should be normalized floats (0.0 to 1.0)
func (m *Mixer) MixChannels(p1, p2, tri, noise, dmc float32) float32 {
	// Calculate non-linear pulse mix
	pulseSum := float64(p1 + p2)
	var pulseOut float64
	if pulseSum > 1e-9 {
		pulseOut = 95.88 / ((541.8666666666667 / pulseSum) + 100.0)
	}

	// Calculate non-linear triangle/noise/DMC mix
	tTerm := float64(tri) / 548.4666666666667
	nTerm := float64(noise) / 816.0666666666667
	dTerm := float64(dmc) / 178.251968503937
	tndDenominatorSum := tTerm + nTerm + dTerm

	var tndOut float64
	if tndDenominatorSum > 1e-9 {
		tndOut = 159.79 / ((1.0 / tndDenominatorSum) + 100.0)
	}

	// Combine mixes
	mixRaw := float32(pulseOut + tndOut)

	// Apply DC blocking filter
	m.dcOffset = m.dcOffset*m.dcBlockingAlpha + mixRaw*(1.0-m.dcBlockingAlpha)
	mixFiltered := mixRaw - m.dcOffset

	// Apply master volume
	mixFinal := mixFiltered * m.masterVolume

	return mixFinal
}