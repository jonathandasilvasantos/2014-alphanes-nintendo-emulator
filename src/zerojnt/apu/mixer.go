// File: apu/mixer.go
package apu

// Mixer handles combining the audio channel outputs.
type Mixer struct {
	// Scaling factors to balance the relative volume of each channel.
	pulseScale    float32
	triangleScale float32
	noiseScale    float32
	// dmcScale      float32 // Reserved for future Delta Modulation Channel
	
	// Hard limit to prevent audio clipping
	useHardLimit  bool
	hardLimit     float32
	
	// DC blocking filter to remove constant offset (improve audio quality)
	dcBlockingAlpha float32
	dcOffset        float32
}

// NewMixer creates and initializes a mixer with balanced scaling factors.
func NewMixer() *Mixer {
	return &Mixer{
		// Properly balanced channel volumes without the *2 multiplier on pulse
		// These values were tuned for a better audio mix
		pulseScale:      0.15,     // Was 0.00752 * 2, now properly scaled
		triangleScale:   0.20,     // Was 0.00851, increased for better balance
		noiseScale:      0.10,     // Was 0.00494, adjusted for better balance
		// dmcScale:     0.12,     // Reserved for future DMC implementation
		
		// Enable hard limiting to prevent distortion
		useHardLimit:    true,
		hardLimit:       0.95,     // Allow some headroom before full scale
		
		// DC blocking filter to remove unwanted low frequency noise
		dcBlockingAlpha: 0.995,    // Filter coefficient (near 1.0 for slight effect)
		dcOffset:        0.0,      // Running average of signal, initialized at 0
	}
}

// MixChannels combines the outputs of the individual APU channels.
// Each channel parameter should provide a normalized output in the range 0.0 to 1.0.
func (m *Mixer) MixChannels(pulse1, pulse2, triangle, noise, dmc float32) float32 {
	// Apply individual scaling factors to each channel
	scaledPulse1 := pulse1 * m.pulseScale
	scaledPulse2 := pulse2 * m.pulseScale
	scaledTriangle := triangle * m.triangleScale
	scaledNoise := noise * m.noiseScale
	// scaledDMC := dmc * m.dmcScale // Reserved for future DMC implementation

	// Sum the scaled outputs
	mix := scaledPulse1 + scaledPulse2 + scaledTriangle + scaledNoise // + scaledDMC

	// Apply DC blocking filter to remove constant offsets (improves audio quality)
	// This is a simple high-pass filter that removes very low frequencies
	m.dcOffset = m.dcOffset*m.dcBlockingAlpha + mix*(1.0-m.dcBlockingAlpha)
	mix = mix - m.dcOffset

	// Apply hard limiting if enabled to prevent distortion
	if m.useHardLimit {
		if mix > m.hardLimit {
			mix = m.hardLimit
		} else if mix < -m.hardLimit {
			mix = -m.hardLimit
		}
	} else {
		// Ensure output is in the range [-1.0, 1.0] regardless
		mix = clamp(mix, -1.0, 1.0)
	}

	return mix
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