package apu

import (
    "github.com/gordonklaus/portaudio"
    "sync"
    "zerojnt/apu/channels"
)

const (
    SampleRate = 44100
    BufferSize = 735 // ~60Hz frame rate (44100/60)
)

type APU struct {
    pulse1       *channels.PulseChannel
    pulse2       *channels.PulseChannel
    triangle     *channels.TriangleChannel
    noise        *channels.NoiseChannel
    buffer       []float32
    bufferIndex  int
    stream       *portaudio.Stream
    mutex        sync.Mutex
    enabled      bool
    sampleTimer  float64
    cyclesPerSample float64
    frameCounter int
    frameIRQ     bool
}

func NewAPU() (*APU, error) {
    apu := &APU{
        pulse1:   channels.NewPulseChannel(),
        pulse2:   channels.NewPulseChannel(),
        triangle: channels.NewTriangleChannel(),
        noise:    channels.NewNoiseChannel(),
        buffer:   make([]float32, BufferSize),
        enabled:  true,
        // CPU clock rate (1.789773 MHz) divided by sample rate (44.1 kHz)
        cyclesPerSample: 1789773.0 / 44100.0,
        frameCounter:    0,
    }

    // Initialize PortAudio
    err := portaudio.Initialize()
    if err != nil {
        return nil, err
    }

    // Create audio stream
    stream, err := portaudio.OpenDefaultStream(
        0,                  // input channels
        1,                  // output channels
        float64(SampleRate),
        len(apu.buffer),    // frames per buffer
        apu.buffer,         // buffer
    )
    if err != nil {
        return nil, err
    }

    apu.stream = stream
    err = stream.Start()
    if err != nil {
        return nil, err
    }

    return apu, nil
}

func (apu *APU) WriteRegister(addr uint16, value byte) {
    apu.mutex.Lock()
    defer apu.mutex.Unlock()

    switch addr {
    case 0x4000, 0x4001, 0x4002, 0x4003:
        apu.pulse1.WriteRegister(addr, value)
    case 0x4004, 0x4005, 0x4006, 0x4007:
        apu.pulse2.WriteRegister(addr-4, value)
    case 0x4008, 0x4009, 0x400A, 0x400B:
        apu.triangle.WriteRegister(addr, value)
    case 0x400C, 0x400D, 0x400E, 0x400F:
        apu.noise.WriteRegister(addr, value)
    case 0x4015: // Status register
        apu.pulse1.SetEnabled(value&1 != 0)
        apu.pulse2.SetEnabled(value&2 != 0)
        apu.triangle.SetEnabled(value&4 != 0)
        apu.noise.SetEnabled(value&8 != 0)
    case 0x4017: // Frame counter
        apu.frameCounter = 0
        apu.frameIRQ = (value & 0x40) == 0
        // Mode 0: 4-step sequence
        // Mode 1: 5-step sequence
        if (value & 0x80) != 0 {
            apu.clockFrameCounter() // Immediately clock on mode 1
        }
    }
}

func (apu *APU) Clock() {
    apu.mutex.Lock()
    defer apu.mutex.Unlock()

    if !apu.enabled {
        return
    }

    // Update sample timer
    apu.sampleTimer++

    // Check if it's time to generate a new sample
    if apu.sampleTimer >= apu.cyclesPerSample {
        apu.sampleTimer -= apu.cyclesPerSample

        // Generate new sample from all channels
        pulse1Sample := apu.pulse1.GetSample()
        pulse2Sample := apu.pulse2.GetSample()
        triangleSample := apu.triangle.GetSample()
        noiseSample := apu.noise.GetSample()
        
        // Mix samples using the NES non-linear mixing formula
        pulseOut := 0.00752 * (pulse1Sample + pulse2Sample)
        tndOut := 0.00851 * triangleSample + 0.00494 * noiseSample
        sample := pulseOut + tndOut

        // Apply simple limiter to prevent clipping
        if sample > 1.0 {
            sample = 1.0
        } else if sample < -1.0 {
            sample = -1.0
        }

        // Write sample to buffer
        apu.buffer[apu.bufferIndex] = sample
        apu.bufferIndex++

        // If buffer is full, send to audio device
        if apu.bufferIndex >= len(apu.buffer) {
            apu.stream.Write()
            apu.bufferIndex = 0
        }
    }

    // Clock all channels
    apu.pulse1.Clock()
    apu.pulse2.Clock()
    apu.triangle.Clock()
    apu.noise.Clock()

    // Clock frame counter
    apu.clockFrameCounter()
}

func (apu *APU) clockFrameCounter() {
    // Frame counter runs at 240Hz (CPU clock / 7457.5)
    apu.frameCounter++
    if apu.frameCounter >= 7457 {
        apu.frameCounter = 0
        
        // Clock length counters and sweep units
        apu.pulse1.Clock()
        apu.pulse2.Clock()
        apu.triangle.ClockLinearCounter()
        apu.noise.Clock()

        // Generate frame IRQ if enabled
        if apu.frameIRQ {
            // Signal IRQ to CPU (implementation needed)
        }
    }
}

func (apu *APU) Shutdown() {
    apu.mutex.Lock()
    defer apu.mutex.Unlock()

    if apu.stream != nil {
        apu.stream.Stop()
        apu.stream.Close()
    }
    portaudio.Terminate()
}
