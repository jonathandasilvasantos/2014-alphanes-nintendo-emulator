package apu

import (
	"sync"
	"github.com/gordonklaus/portaudio"
	"zerojnt/apu/channels"
)

const (
	SampleRate      = 44100
	CpuClockSpeed   = 1789773         // 1.789773 MHz
	FrameRate       = 240             // 240Hz frame counter rate
	BufferSizeMs    = 20             // 20ms buffer (~1 frame at 60fps)
	BufferSize      = (SampleRate * BufferSizeMs) / 1000
	CpuCyclesPerSample = float64(CpuClockSpeed) / float64(SampleRate)
	CpuCyclesPerFrame = float64(CpuClockSpeed) / float64(FrameRate) // Now a float64 for accuracy
)

type APU struct {
	pulse1   *channels.PulseChannel
	pulse2   *channels.PulseChannel
	triangle *channels.TriangleChannel
	noise    *channels.NoiseChannel
	mixer    *Mixer

	// Double buffering system
	buffer        []float32
	backBuffer    []float32
	bufferIndex   int
	
	stream        *portaudio.Stream
	mutex         sync.Mutex
	
	// Timing management
	cycleCount       uint64  // Using uint64 for a global cycle counter
	frameCounter     float64 // Frame counter for 240Hz clock, can be fractional
	enabled          bool
	irqEnabled      bool
	irqFlag         bool
	inhibitIRQ      bool
	
	// Sample generation state
	lastSampleTime   float64
	mode5Step       bool
	frameSequence   int
	
	// Channel for buffer swapping
	bufferSwap      chan struct{}
}

// Added to expose IRQ status to CPU
func (apu *APU) IRQ() bool {
	apu.mutex.Lock()
	defer apu.mutex.Unlock()
	return apu.irqFlag
}

// Added to allow CPU to clear IRQ
func (apu *APU) ClearIRQ() {
	apu.mutex.Lock()
	defer apu.mutex.Unlock()
	apu.irqFlag = false
}

func NewAPU() (*APU, error) {
    apu := &APU{
        pulse1:      channels.NewPulseChannel(),
        pulse2:      channels.NewPulseChannel(),
        triangle:    channels.NewTriangleChannel(),
        noise:       channels.NewNoiseChannel(),
        mixer:       NewMixer(),
        buffer:      make([]float32, BufferSize),
        backBuffer:  make([]float32, BufferSize),
        enabled:     true,
        bufferSwap:  make(chan struct{}, 1),
    }

    // Initialize APU registers to documented power-on values
    apu.WriteRegister(0x4015, 0x00) // Disable all channels
    apu.WriteRegister(0x4017, 0x00) // 4-step sequence, IRQ disabled

    // Initialize each channel to its default state
    apu.pulse1.Reset()
    apu.pulse2.Reset()
    apu.triangle.Reset()
    apu.noise.Reset()

    err := portaudio.Initialize()
    if err != nil {
        return nil, err
    }

    stream, err := portaudio.OpenDefaultStream(
        0, // input channels
        1, // output channels
        float64(SampleRate),
        BufferSize, // frames per buffer
        apu.audioCallback,
    )
    if err != nil {
        portaudio.Terminate()
        return nil, err
    }

    apu.stream = stream
    if err := stream.Start(); err != nil {
        stream.Close()
        portaudio.Terminate()
        return nil, err
    }

    go apu.bufferManager()

    return apu, nil
}

func (apu *APU) audioCallback(out []float32) {
	apu.mutex.Lock()
	copy(out, apu.buffer)
	apu.mutex.Unlock()

	select {
	case apu.bufferSwap <- struct{}{}:
	default:
	}
}

func (apu *APU) bufferManager() {
	for range apu.bufferSwap {
		apu.mutex.Lock()
		apu.buffer, apu.backBuffer = apu.backBuffer, apu.buffer
		apu.bufferIndex = 0
		apu.mutex.Unlock()
	}
}

func (apu *APU) Clock(globalCycleCount uint64) {
	apu.mutex.Lock()
	defer apu.mutex.Unlock()

	apu.cycleCount++

    // Accurate 240Hz clock using a float64 frame counter
    apu.frameCounter += 1
    if apu.frameCounter >= CpuCyclesPerFrame {
        apu.frameCounter -= CpuCyclesPerFrame
        apu.clockFrameSequencer(globalCycleCount)
        
    }

	apu.lastSampleTime += 1
	if apu.lastSampleTime >= CpuCyclesPerSample {
		apu.lastSampleTime -= CpuCyclesPerSample
		apu.generateSample()
	}

	apu.pulse1.Clock()
	apu.pulse2.Clock()
	apu.triangle.Clock()
	apu.noise.Clock()
}

func (apu *APU) clockFrameSequencer(globalCycleCount uint64) {
    // Log each step of the frame sequencer
    if apu.mode5Step {
        switch apu.frameSequence % 5 {
        case 0, 2:
            apu.clockEnvelopes()
            
        case 1, 3:
            apu.clockEnvelopes()
            apu.clockLengthCounters()
            
        case 4:
            // Silent step
            
        }
        apu.frameSequence = (apu.frameSequence + 1) % 5
    } else {
        switch apu.frameSequence % 4 {
        case 0, 2:
            apu.clockEnvelopes()
            
        case 1, 3:
            apu.clockEnvelopes()
            apu.clockLengthCounters()
            if !apu.inhibitIRQ && apu.frameSequence == 3 {
                apu.irqFlag = true
            }
            
        }
        apu.frameSequence = (apu.frameSequence + 1) % 4
    }
}

func (apu *APU) clockEnvelopes() {
	apu.pulse1.ClockEnvelope()
	apu.pulse2.ClockEnvelope()
	apu.triangle.ClockLinearCounter()
	apu.noise.ClockEnvelope()
}

func (apu *APU) clockLengthCounters() {
	apu.pulse1.ClockLengthCounter()
	apu.pulse2.ClockLengthCounter()
	apu.triangle.ClockLengthCounter()
	apu.noise.ClockLengthCounter()

	apu.pulse1.ClockSweep()
	apu.pulse2.ClockSweep()
}

func (apu *APU) generateSample() {
	if !apu.enabled {
		return
	}

	sample := apu.mixer.MixChannels(
		apu.pulse1.GetSample(),
		apu.pulse2.GetSample(),
		apu.triangle.GetSample(),
		apu.noise.GetSample(),
		0,
	)

	if apu.bufferIndex < len(apu.backBuffer) {
		apu.backBuffer[apu.bufferIndex] = sample
		apu.bufferIndex++
	}
}

func (apu *APU) WriteRegister(addr uint16, value byte) {
	apu.mutex.Lock()
	defer apu.mutex.Unlock()

    

	switch addr {
	case 0x4000, 0x4001, 0x4002, 0x4003:
		apu.pulse1.WriteRegister(addr, value)
	case 0x4004, 0x4005, 0x4006, 0x4007:
		apu.pulse2.WriteRegister(addr, value)
	case 0x4008, 0x4009, 0x400A, 0x400B:
		apu.triangle.WriteRegister(addr, value)
	case 0x400C, 0x400D, 0x400E, 0x400F:
		apu.noise.WriteRegister(addr, value)
	case 0x4015:
        apu.pulse1.SetEnabled((value & 0x01) != 0)
        apu.pulse2.SetEnabled((value & 0x02) != 0)
        apu.triangle.SetEnabled((value & 0x04) != 0)
        apu.noise.SetEnabled((value & 0x08) != 0)
	case 0x4017:
		apu.mode5Step = (value & 0x80) != 0
		apu.inhibitIRQ = (value & 0x40) != 0
		if apu.inhibitIRQ {
			apu.irqFlag = false
		}
        
        // Handle immediate clocking of envelopes and length counters
        if apu.mode5Step {
            // 5-step mode
            apu.clockEnvelopes()
            apu.clockLengthCounters()
            apu.frameSequence = 1 // Start at step 1 to align with behavior
        } else {
            // 4-step mode
            apu.frameSequence = 0
        }
	}
}

func (apu *APU) ReadStatus() byte {
	apu.mutex.Lock()
	defer apu.mutex.Unlock()

	var status byte
	if apu.pulse1.IsEnabled() {
		status |= 0x01
	}
	if apu.pulse2.IsEnabled() {
		status |= 0x02
	}
	if apu.triangle.IsEnabled() {
		status |= 0x04
	}
	if apu.noise.IsEnabled() {
		status |= 0x08
	}
	if apu.irqFlag {
		status |= 0x40
	}

	apu.irqFlag = false // Reading $4015 clears the IRQ flag
	return status
}

func (apu *APU) Shutdown() {
	close(apu.bufferSwap)

	apu.mutex.Lock()
	defer apu.mutex.Unlock()

	if apu.stream != nil {
		apu.stream.Stop()
		apu.stream.Close()
	}
	portaudio.Terminate()
}