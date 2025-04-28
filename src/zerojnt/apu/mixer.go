// File: apu/mixer.go
package apu

// Mixer combines audio channel outputs using non-linear formulas
// and applies appropriate analog filtering to match the NES hardware
type Mixer struct {
    masterVolume float32 // Master volume control (0.0 to 1.0)

    // HPF / LPF memories
    hp1Mem, hp2Mem float32 // High-pass filter memory states
    lpMem          float32 // Low-pass filter memory
}

// NewMixer creates and initializes a mixer with default settings
func NewMixer() *Mixer {
    return &Mixer{
        masterVolume: 1.0,
        // Memories start at 0
    }
}

// MixChannels combines the outputs of individual APU channels
// Inputs should be normalized floats (0.0 to 1.0)
func (m *Mixer) MixChannels(p1, p2, tri, noise, dmc float32) float32 {
    // -------- 1.  Non-linear hardware mixer -----------
    pulseSum := float64(p1 + p2)
    var pulseOut float64
    if pulseSum > 1e-9 {
        pulseOut = 95.88 / ((541.8666667 / pulseSum) + 100.0)
    }

    tnd := float64(tri)/548.4666667 +
           float64(noise)/816.0666667 +
           float64(dmc)/178.2519685
    var tndOut float64
    if tnd > 1e-9 {
        tndOut = 159.79 / ((1.0 / tnd) + 100.0)
    }

    mixRaw := float32(pulseOut + tndOut)

    // -------- 2.  Analog output filters ---------------
    // High-pass 90 Hz  (α ≈ e^(-2π·90/44100) ≈ 0.987)
    const hp1A = float32(0.987)
    hp1 := mixRaw - m.hp1Mem + hp1A*m.hp1Mem
    m.hp1Mem = mixRaw + hp1A*m.hp1Mem - hp1A*m.hp1Mem // update memory

    // High-pass 440 Hz (α ≈ 0.882)
    const hp2A = float32(0.882)
    hp2 := hp1 - m.hp2Mem + hp2A*m.hp2Mem
    m.hp2Mem = hp1 + hp2A*m.hp2Mem - hp2A*m.hp2Mem

    // Low-pass 14 kHz (α = e^(-2π·14 000/44 100) ≈ 0.529)
    const lpA = float32(0.529)
    lp := (1-lpA)*hp2 + lpA*m.lpMem
    m.lpMem = lp

    // -------- 3.  Master volume -----------------------
    return lp * m.masterVolume
}