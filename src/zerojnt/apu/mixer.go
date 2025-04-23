// File: apu/mixer.go
package apu

import (
	"math" // Needed for math.Tanh
)

// Mixer handles combining the audio channel outputs and applying final processing.
type Mixer struct {
	// --- Configuration ---

	// Master Volume Control (Applied before clipping)
	masterVolume float32

	// Scaling factors based roughly on common linear approximations, adjusted for balance.
	// These determine the relative contribution of each channel *before* master volume.
	// Goal: Avoid clipping *most* of the time with typical game audio at masterVolume = 1.0
	pulseScale    float32
	triangleScale float32
	noiseScale    float32
	// dmcScale      float32 // Reserved for future Delta Modulation Channel

	// Soft Clipping Strength (Higher value -> harder clipping near limits)
	// A value around 1.0 to 3.0 is typical. 1.0 gives a gentle curve.
	softClipFactor float32

	// DC Blocking Filter configuration (removes constant offset)
	// Alpha close to 1.0 means slow adaptation (less low-frequency filtering).
	// Value around 0.995 is common.
	dcBlockingAlpha float32

	// --- State ---
	dcOffset float32 // Running average of the signal for DC blocking
}

// NewMixer creates and initializes a mixer with refined settings.
func NewMixer() *Mixer {
	return &Mixer{
		// Configuration: These values can be tuned further based on testing.
		masterVolume: 1.0, // Start at full volume

		// Adjusted scaling factors - Aim for slightly lower levels than before
		// to give headroom before hitting the soft clipper frequently.
		// Relative balance: Triangle often perceived loudest, Noise quietest.
		pulseScale:    0.09, // Was 0.15
		triangleScale: 0.14, // Was 0.20
		noiseScale:    0.07, // Was 0.10
		// dmcScale:     0.08, // Placeholder for future DMC

		softClipFactor: 1.5,   // Moderate soft clipping curve
		dcBlockingAlpha: 0.995, // Standard DC blocking coefficient

		// State
		dcOffset: 0.0, // Initialize DC offset estimate
	}
}

// MixChannels combines the outputs of the individual APU channels.
// Inputs (pulse1, pulse2, triangle, noise, dmc) should be normalized (e.g., 0.0 to 1.0).
func (m *Mixer) MixChannels(pulse1, pulse2, triangle, noise, dmc float32) float32 {
	// 1. Apply individual scaling factors
	// Note: Pulse channels combine linearly before contributing
	pulseSum := (pulse1 + pulse2) * m.pulseScale
	triangleOut := triangle * m.triangleScale
	noiseOut := noise * m.noiseScale
	// dmcOut := dmc * m.dmcScale // Reserved for future DMC implementation

	// 2. Sum the scaled channel outputs
	// This is a linear mix. Real NES hardware is non-linear, but this is a common approximation.
	mix := pulseSum + triangleOut + noiseOut // + dmcOut

	// 3. Apply DC Blocking Filter (High-pass filter)
	// Removes unwanted constant voltage offset that can cause clicks on start/stop.
	// Calculation: y[n] = x[n] - x[n-1] + alpha * y[n-1] (common form)
	// Simplified IIR: offset = offset*alpha + mix*(1-alpha); output = mix - offset
	m.dcOffset = m.dcOffset*m.dcBlockingAlpha + mix*(1.0-m.dcBlockingAlpha)
	mix = mix - m.dcOffset

	// 4. Apply Master Volume
	mix *= m.masterVolume

	// 5. Apply Soft Clipping using Hyperbolic Tangent (tanh)
	// tanh maps the input range (-inf, +inf) to (-1.0, 1.0) smoothly.
	// The softClipFactor scales the input to control how quickly it saturates.
	// We use float64 for math.Tanh as it's often faster/more standard.
	mix = float32(math.Tanh(float64(mix * m.softClipFactor)))

	// 6. Final safety clamp (optional, tanh should keep it within [-1, 1])
	// mix = clamp(mix, -1.0, 1.0) // Usually not needed after tanh

	return mix
}

// clamp ensures a value stays within the specified minimum and maximum bounds.
// (Keeping this utility function in case it's needed elsewhere or for hard clipping option)
func clamp(value, min, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}