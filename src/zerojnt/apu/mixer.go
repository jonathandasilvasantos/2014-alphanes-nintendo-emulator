package apu

// Mixer handles combining audio channels with advanced audio processing
type Mixer struct {
    // Channel volume balancing with finer control
    pulseScale    float32
    triangleScale float32
    noiseScale    float32
    
    // DC offset removal
    dcOffset      float32
    
    // Lowpass filter state variables
    lastOutput    float32
    lastLP        float32
    
    // Highpass filter state variables
    lastHP        float32
    lastHPInput   float32
    
    // Compression parameters
    compThreshold float32
    compRatio     float32
    compGain      float32
    
    // Moving average for dynamic range control
    avgLevel      float32
}

// NewMixer creates and initializes a new Mixer with optimized settings
func NewMixer() *Mixer {
    return &Mixer{
        // Fine-tuned channel volumes for optimal balance
        pulseScale:    0.28,    // Slightly reduced from 0.30 for less harshness
        triangleScale: 0.27,    // Slightly increased from 0.25 for better bass
        noiseScale:    0.13,    // Reduced from 0.15 for cleaner mix
        
        // Initialize filter states
        dcOffset:      0.0,
        lastOutput:    0.0,
        lastLP:        0.0,
        lastHP:        0.0,
        lastHPInput:   0.0,
        
        // Compression settings
        compThreshold: 0.6,
        compRatio:     0.5,
        compGain:      1.1,
        
        avgLevel:      0.0,
    }
}

// applyLowPass applies a simple first-order lowpass filter
func (m *Mixer) applyLowPass(input float32) float32 {
    // Cutoff frequency factor (0-1)
    alpha := float32(0.2)
    
    m.lastLP = m.lastLP + alpha*(input-m.lastLP)
    return m.lastLP
}

// applyHighPass applies a simple first-order highpass filter
func (m *Mixer) applyHighPass(input float32) float32 {
    // Cutoff frequency factor (0-1)
    alpha := float32(0.95)
    
    m.lastHP = alpha * (m.lastHP + input - m.lastHPInput)
    m.lastHPInput = input
    return m.lastHP
}

// updateDCOffset tracks and removes DC offset
func (m *Mixer) updateDCOffset(input float32) float32 {
    // Very slow tracking of DC offset
    m.dcOffset = m.dcOffset*0.999 + input*0.001
    return input - m.dcOffset
}

// applyCompression implements soft knee compression
func (m *Mixer) applyCompression(input float32) float32 {
    // Update average level
    m.avgLevel = m.avgLevel*0.99 + abs(input)*0.01
    
    if m.avgLevel < m.compThreshold {
        return input * m.compGain
    }
    
    // Calculate compression
    overThreshold := m.avgLevel - m.compThreshold
    compression := overThreshold * (1.0 - m.compRatio)
    return input * (m.compGain * (m.compThreshold + compression) / m.avgLevel)
}

// softClip implements smooth saturation
func softClip(input float32) float32 {
    // Smoother sigmoid-like clipping
    if input > 0.9 {
        excess := input - 0.9
        return 0.9 + excess/(1.0+excess)
    } else if input < -0.9 {
        excess := -input - 0.9
        return -(0.9 + excess/(1.0+excess))
    }
    return input
}

// abs returns absolute value of float32
func abs(x float32) float32 {
    if x < 0 {
        return -x
    }
    return x
}

// MixChannels combines all channel outputs with improved processing
func (m *Mixer) MixChannels(pulse1, pulse2, triangle, noise, _ float32) float32 {
    // Scale each channel with improved balance
    p1 := pulse1 * m.pulseScale
    p2 := pulse2 * m.pulseScale
    tri := triangle * m.triangleScale
    noi := noise * m.noiseScale

    // Initial mix
    mix := p1 + p2 + tri + noi
    
    // Apply audio processing chain
    mix = m.updateDCOffset(mix)    // Remove DC offset
    mix = m.applyHighPass(mix)     // Remove sub-bass rumble
    mix = m.applyLowPass(mix)      // Smooth harsh frequencies
    mix = m.applyCompression(mix)  // Control dynamic range
    mix = softClip(mix)            // Soft saturation
    
    // Final output smoothing
    output := mix*0.7 + m.lastOutput*0.3
    m.lastOutput = output
    
    return clamp(output, -1.0, 1.0)
}

// clamp ensures a value stays within bounds
func clamp(value, min, max float32) float32 {
    if value < min {
        return min
    }
    if value > max {
        return max
    }
    return value
}