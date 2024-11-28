package apu

// Simple linear mixer for now
type Mixer struct {
    pulseTable []float32
}

func NewMixer() *Mixer {
    m := &Mixer{
        pulseTable: make([]float32, 31), // For two pulse channels (15 + 15 + 1)
    }
    
    // Initialize pulse mixing table
    for i := 0; i < 31; i++ {
        m.pulseTable[i] = float32(i) / 30.0
    }
    
    return m
}

func (m *Mixer) MixPulses(pulse1, pulse2 float32) float32 {
    // Simple mixing for now - just average the channels
    return (pulse1 + pulse2) * 0.5
}