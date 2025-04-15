// File: apu/mixer.go (Simplified Version)
package apu

// Mixer handles combining the audio channel outputs.
// This is a simplified version focusing on basic scaling and summing.
type Mixer struct {
	// Scaling factors to balance the relative volume of each channel.
	// These values can be tuned to achieve a desired mix.
	pulseScale    float32
	triangleScale float32
	noiseScale    float32
	// dmcScale      float32 // Placeholder for Delta Modulation Channel
}

// NewMixer creates and initializes a new simplified Mixer.
func NewMixer() *Mixer {
	return &Mixer{
		// Initial scaling factors. These are common starting points,
		// aiming for a reasonable balance between channel loudness.
		// Derived from typical values seen in NES emulators.
		pulseScale:    0.00752 * 2, // Combine factor for two pulse channels
		triangleScale: 0.00851,
		noiseScale:    0.00494,
		// dmcScale:      0.00335,
	}
	// Note: The exact scaling might need further tuning based on how
	// channel outputs are normalized (e.g., are they 0-15 or 0.0-1.0?).
	// Assuming channel Output() methods return roughly 0.0 to 1.0.
	// Let's adjust based on the previous advanced mixer's tuned scale factors for a better start:
	/*
	   return &Mixer{
	       pulseScale:    0.28, // Factor per pulse channel
	       triangleScale: 0.27,
	       noiseScale:    0.13,
	       // dmcScale: 0.10, // Example
	   }
	*/
}

// MixChannels combines the outputs of the individual APU channels.
// pulse1, pulse2, triangle, noise should be the output sample from each channel (typically -1.0 to 1.0 or 0.0 to 1.0).
// dmc is a placeholder for the Delta Modulation Channel output.
func (m *Mixer) MixChannels(pulse1, pulse2, triangle, noise, dmc float32) float32 {

	// --- Simple Linear Mix ---
	// Apply individual scaling factors
	scaledPulse1 := pulse1 * m.pulseScale
	scaledPulse2 := pulse2 * m.pulseScale
	scaledTriangle := triangle * m.triangleScale
	scaledNoise := noise * m.noiseScale
	// scaledDMC := dmc * m.dmcScale // When DMC is implemented

	// Sum the scaled channel outputs
	// Using separate pulse scaling allows adjusting relative pulse volume easily.
	// For a simpler fixed mix often cited:
	// pulse_sum := m.pulseScale * (pulse1 + pulse2) // Use a combined pulse scale
	// mix := pulse_sum + scaledTriangle + scaledNoise // + scaledDMC

	// Mix using individual scales:
	mix := scaledPulse1 + scaledPulse2 + scaledTriangle + scaledNoise // + scaledDMC

	// --- Clamping ---
	// Ensure the final output stays within the standard audio range [-1.0, 1.0].
	// This prevents hard digital clipping artifacts if the mix exceeds the range.
	finalOutput := clamp(mix, -1.0, 1.0)

	return finalOutput
}

// clamp ensures a value stays within the specified minimum and maximum bounds.
func clamp(value, min, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}