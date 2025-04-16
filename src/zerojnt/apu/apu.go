package apu

import (
	"log"
	"sync"
	"time"
	"github.com/gordonklaus/portaudio"
	"zerojnt/apu/channels"
)

const (
	// Audio System Configuration
	SampleRate        = 44100 // Standard audio sample rate
	BufferSizeSamples = 1764  // ~40ms buffer at 44.1kHz - DOUBLED for better stability
	
	// Debug flags
	DebugAudio        = false // Set to true to enable audio debug logging

	// NES Timing Constants (NTSC)
	CpuClockSpeed     = 1789773.0 // Base CPU clock frequency
	FrameCounterRate  = 240.0     // Frequency of the frame counter clock (4 steps or 5 steps)
	CpuCyclesPerAudioSample = CpuClockSpeed / SampleRate // CPU cycles between audio samples
	CpuCyclesPerFrameStep   = CpuClockSpeed / FrameCounterRate // CPU cycles between frame counter steps
)

// APU represents the NES Audio Processing Unit.
type APU struct {
	pulse1   *channels.PulseChannel
	pulse2   *channels.PulseChannel
	triangle *channels.TriangleChannel
	noise    *channels.NoiseChannel
	mixer    *Mixer

	// Audio Output Buffering
	buffer        []float32      // Buffer passed to audio callback
	backBuffer    []float32      // Buffer being filled with new samples
	bufferIndex   int            // Current position in backBuffer
	bufferSwap    chan struct{}  // Signals when a buffer swap is needed
	stream        *portaudio.Stream
	mutex         sync.Mutex     // Protects shared APU state (buffers, timing, flags)
	bufferStats   struct {
		underruns int            // Count of buffer underruns (debug)
		overruns  int            // Count of buffer overruns (debug)
		lastReport time.Time     // Time of last stats report
	}

	// Frame Counter / Sequencer State
	frameCounterCycles float64 // Tracks CPU cycles towards the next frame counter step
	frameSequenceStep  int     // Current step in the 4 or 5-step sequence (0-3 or 0-4)
	sequenceMode5Step  bool    // False: 4-step sequence, True: 5-step sequence
	inhibitIRQ         bool    // True: Frame counter IRQ is disabled ($4017 bit 6)
	irqPending         bool    // True: Frame counter IRQ has been triggered

	// Sample Generation Timing
	sampleGenCycles float64 // Tracks CPU cycles towards the next audio sample generation
	
	// Low-pass filter state for noise reduction
	previousSample float32 // Previous output sample for simple low-pass filter

	// CPU Synchronization
	cpuCycleCounter uint64 // Tracks total CPU cycles processed by APU
}

// NewAPU creates and initializes a new APU instance.
func NewAPU() (*APU, error) {
	log.Println("Initializing APU...")

	apu := &APU{
		pulse1:     channels.NewPulseChannel(1),
		pulse2:     channels.NewPulseChannel(2),
		triangle:   channels.NewTriangleChannel(),
		noise:      channels.NewNoiseChannel(),
		mixer:      NewMixer(),
		buffer:     make([]float32, BufferSizeSamples),
		backBuffer: make([]float32, BufferSizeSamples),
		bufferSwap: make(chan struct{}, 1), // Buffered channel (size 1)
		bufferStats: struct {
			underruns  int
			overruns   int
			lastReport time.Time
		}{
			lastReport: time.Now(),
		},
	}

	// Initialize APU registers to documented power-on values
	apu.WriteRegister(0x4017, 0x00) // 4-step sequence, IRQ disabled

	// Ensure channels start in their reset state
	apu.pulse1.Reset()
	apu.pulse2.Reset()
	apu.triangle.Reset()
	apu.noise.Reset()

	// Initialize and zero out buffers for clean audio start
	for i := range apu.buffer {
		apu.buffer[i] = 0.0
		apu.backBuffer[i] = 0.0
	}

	// Initialize PortAudio
	err := portaudio.Initialize()
	if err != nil {
		log.Printf("PortAudio Initialization Error: %v", err)
		return nil, err
	}

	// Open Default Audio Stream
	stream, err := portaudio.OpenDefaultStream(
		0, // No input channels
		1, // Mono output
		SampleRate,
		BufferSizeSamples,
		apu.audioCallback, // Function to provide audio data
	)
	if err != nil {
		log.Printf("PortAudio Open Stream Error: %v", err)
		portaudio.Terminate()
		return nil, err
	}

	apu.stream = stream

	// Start the audio stream
	if err := stream.Start(); err != nil {
		log.Printf("PortAudio Start Stream Error: %v", err)
		stream.Close()
		portaudio.Terminate()
		return nil, err
	}
	log.Println("PortAudio Stream Started.")

	// Start the buffer manager goroutine
	go apu.bufferManager()

	log.Println("APU Initialization Complete.")
	return apu, nil
}

// audioCallback is called by PortAudio when it needs more audio data.
func (apu *APU) audioCallback(out []float32) {
	apu.mutex.Lock()
	
	// Check if we have a complete buffer
	if apu.bufferIndex < len(apu.backBuffer) {
		// Buffer underrun - not enough samples were generated
		if DebugAudio {
			apu.bufferStats.underruns++
			if time.Since(apu.bufferStats.lastReport) > 5*time.Second {
				log.Printf("APU Stats: %d underruns, %d overruns", 
					apu.bufferStats.underruns, apu.bufferStats.overruns)
				apu.bufferStats.lastReport = time.Now()
				apu.bufferStats.underruns = 0
				apu.bufferStats.overruns = 0
			}
		}
		
		// Fill the remaining buffer with silence to prevent audible glitches
		for i := apu.bufferIndex; i < len(apu.backBuffer); i++ {
			apu.backBuffer[i] = 0.0
		}
	}
	
	// Provide the prepared buffer
	copy(out, apu.buffer)
	apu.mutex.Unlock()

	// Signal the bufferManager that a swap is needed
	select {
	case apu.bufferSwap <- struct{}{}:
		// Signal sent successfully
	default:
		// Channel full - bufferManager hasn't processed previous signal yet
		if DebugAudio {
			log.Println("APU Warning: Audio callback faster than buffer manager")
		}
	}
}

// bufferManager runs in a separate goroutine to swap audio buffers.
func (apu *APU) bufferManager() {
	for range apu.bufferSwap { // Waits for signal from audioCallback
		apu.mutex.Lock()
		// Swap the buffers
		apu.buffer, apu.backBuffer = apu.backBuffer, apu.buffer
		// Reset the index for the (now) backBuffer
		apu.bufferIndex = 0
		// Clear the new backBuffer to ensure clean audio
		for i := range apu.backBuffer {
			apu.backBuffer[i] = 0.0
		}
		apu.mutex.Unlock()
	}
	log.Println("APU Buffer Manager stopped.")
}

// Clock advances the APU state by one CPU cycle.
func (apu *APU) Clock() {
	// --- Triangle Channel Timer Clock ---
	// The triangle timer clocks every CPU cycle, others clock every other CPU cycle.
	apu.triangle.ClockTimer()

	// --- Clock other channels every *other* CPU cycle ---
	// This is a simplification; the exact timing is complex. This matches many emulators.
	apu.cpuCycleCounter++
	if apu.cpuCycleCounter%2 == 0 {
		apu.pulse1.ClockTimer()
		apu.pulse2.ClockTimer()
		apu.noise.ClockTimer()
	}

	// --- Frame Counter Clocking ---
	apu.frameCounterCycles += 1.0
	if apu.frameCounterCycles >= CpuCyclesPerFrameStep {
		apu.frameCounterCycles -= CpuCyclesPerFrameStep
		apu.clockFrameSequencer()
	}

	// --- Audio Sample Generation ---
	apu.sampleGenCycles += 1.0
	if apu.sampleGenCycles >= CpuCyclesPerAudioSample {
		apu.sampleGenCycles -= CpuCyclesPerAudioSample
		apu.generateSample()
	}
}

// clockFrameSequencer advances the frame counter state (4 or 5 steps).
// This clocks envelopes, length counters, and sweeps at specific intervals.
func (apu *APU) clockFrameSequencer() {
	apu.mutex.Lock() // Lock needed as this modifies IRQ state read by CPU

	step := apu.frameSequenceStep

	if apu.sequenceMode5Step { // 5-Step Sequence (Mode 1)
		// Step 1 (0): Clock envelopes & linear counter
		// Step 2 (1): Clock envelopes, linear counter, length counters, sweep units
		// Step 3 (2): Clock envelopes & linear counter
		// Step 4 (3): (none)
		// Step 5 (4): Clock envelopes, linear counter, length counters, sweep units
		// IRQ is NOT generated in 5-step mode.

		clockEnvelopes := (step == 0 || step == 1 || step == 2 || step == 4)
		clockLengthsSweeps := (step == 1 || step == 4)

		if clockEnvelopes {
			apu.clockEnvelopesAndLinear()
		}
		if clockLengthsSweeps {
			apu.clockLengthAndSweep()
		}

		apu.frameSequenceStep = (apu.frameSequenceStep + 1) % 5

	} else { // 4-Step Sequence (Mode 0)
		// Step 1 (0): Clock envelopes & linear counter
		// Step 2 (1): Clock envelopes, linear counter, length counters, sweep units
		// Step 3 (2): Clock envelopes & linear counter
		// Step 4 (3): Clock envelopes, linear counter, length counters, sweep units, generate IRQ (if enabled)

		clockEnvelopes := (step == 0 || step == 1 || step == 2 || step == 3)
		clockLengthsSweeps := (step == 1 || step == 3)

		if clockEnvelopes {
			apu.clockEnvelopesAndLinear()
		}
		if clockLengthsSweeps {
			apu.clockLengthAndSweep()
		}

		// Trigger IRQ on the last step (step 3) if not inhibited
		if step == 3 && !apu.inhibitIRQ {
			apu.irqPending = true
			if DebugAudio {
				log.Printf("APU: Frame IRQ triggered (Step %d, Inhibit: %v)", step, apu.inhibitIRQ)
			}
		}

		apu.frameSequenceStep = (apu.frameSequenceStep + 1) % 4
	}

	apu.mutex.Unlock()
}

// clockEnvelopesAndLinear clocks envelope units and the triangle's linear counter.
func (apu *APU) clockEnvelopesAndLinear() {
	apu.pulse1.ClockEnvelope()
	apu.pulse2.ClockEnvelope()
	apu.triangle.ClockLinearCounter()
	apu.noise.ClockEnvelope()
}

// clockLengthAndSweep clocks length counters and sweep units.
func (apu *APU) clockLengthAndSweep() {
	apu.pulse1.ClockLengthCounter()
	apu.pulse1.ClockSweep() // Sweep clocked along with length
	apu.pulse2.ClockLengthCounter()
	apu.pulse2.ClockSweep() // Sweep clocked along with length
	apu.triangle.ClockLengthCounter()
	apu.noise.ClockLengthCounter()
	// Triangle and Noise don't have sweep units
}

// generateSample creates one audio sample by mixing channel outputs.
func (apu *APU) generateSample() {
	// Mix the channel outputs
	newSample := apu.mixer.MixChannels(
		apu.pulse1.Output(),
		apu.pulse2.Output(),
		apu.triangle.Output(),
		apu.noise.Output(),
		0, // DMC channel placeholder
	)
	
	// Apply a simple low-pass filter to reduce noise (simple weighted average)
	// 80% new sample, 20% previous sample
	filteredSample := (newSample * 0.8) + (apu.previousSample * 0.2)
	apu.previousSample = filteredSample

	apu.mutex.Lock()
	if apu.bufferIndex < len(apu.backBuffer) {
		apu.backBuffer[apu.bufferIndex] = filteredSample
		apu.bufferIndex++
	} else {
		// Buffer overflow - audio generation is faster than consumption
		if DebugAudio {
			apu.bufferStats.overruns++
		}
		// We'll handle this on the next buffer swap
	}
	apu.mutex.Unlock()
}

// WriteRegister handles CPU writes to APU registers ($4000 - $4017).
func (apu *APU) WriteRegister(addr uint16, value byte) {
	apu.mutex.Lock()
	defer apu.mutex.Unlock()

	switch {
	case addr >= 0x4000 && addr <= 0x4003:
		apu.pulse1.WriteRegister(addr, value)
	case addr >= 0x4004 && addr <= 0x4007:
		apu.pulse2.WriteRegister(addr, value)
	case addr >= 0x4008 && addr <= 0x400B:
		apu.triangle.WriteRegister(addr, value)
	case addr >= 0x400C && addr <= 0x400F:
		apu.noise.WriteRegister(addr, value)
	case addr >= 0x4010 && addr <= 0x4013:
		// DMC registers - Not implemented yet
	case addr == 0x4015: // Channel Enable / Status ($4015)
		apu.pulse1.SetEnabled((value & 0x01) != 0)
		apu.pulse2.SetEnabled((value & 0x02) != 0)
		apu.triangle.SetEnabled((value & 0x04) != 0)
		apu.noise.SetEnabled((value & 0x08) != 0)
		// DMC enable (value & 0x10) - Not implemented
		// Writing to $4015 clears the frame IRQ flag
		apu.irqPending = false
		if DebugAudio {
			log.Printf("APU Write $4015 = %02X, IRQ Cleared", value)
		}

	case addr == 0x4017: // Frame Counter Control ($4017)
		apu.sequenceMode5Step = (value & 0x80) != 0
		apu.inhibitIRQ = (value & 0x40) != 0
		if DebugAudio {
			log.Printf("APU Write $4017 = %02X (Mode5: %v, InhibitIRQ: %v)", 
				value, apu.sequenceMode5Step, apu.inhibitIRQ)
		}

		// If IRQ inhibit flag is set, clear any pending IRQ
		if apu.inhibitIRQ {
			apu.irqPending = false
			if DebugAudio {
				log.Println("APU: IRQ Inhibited, pending flag cleared.")
			}
		}

		// Frame counter reset behavior
		apu.frameCounterCycles = 0 // Reset cycle counter towards next step
		apu.frameSequenceStep = 0  // Reset to step 0

		// If mode 1 (5-step) is set, clock Length/Sweep and Envelopes/Linear immediately.
		if apu.sequenceMode5Step {
			apu.clockEnvelopesAndLinear()
			apu.clockLengthAndSweep()
			if DebugAudio {
				log.Println("APU: Mode 5 set, immediate clocking triggered.")
			}
		}

	default:
		// Ignore writes to unused registers in the APU range
	}
}

// ReadStatus reads the APU status register ($4015).
func (apu *APU) ReadStatus() byte {
	apu.mutex.Lock()
	defer apu.mutex.Unlock()

	var status byte
	if apu.pulse1.IsLengthCounterActive() {
		status |= 0x01
	}
	if apu.pulse2.IsLengthCounterActive() {
		status |= 0x02
	}
	if apu.triangle.IsLengthCounterActive() {
		status |= 0x04
	}
	if apu.noise.IsLengthCounterActive() {
		status |= 0x08
	}
	// DMC status (0x10) - Not implemented

	if apu.irqPending {
		status |= 0x40
		if DebugAudio {
			log.Println("APU Read $4015: IRQ was pending.")
		}
	}

	// NESDev Wiki implies it *always* clears the IRQ flag
	apu.irqPending = false
	
	if DebugAudio {
		log.Printf("APU Read $4015 = %02X, IRQ Cleared", status)
	}

	return status
}

// IRQ asserts the CPU IRQ line if the frame counter generated an interrupt.
func (apu *APU) IRQ() bool {
	apu.mutex.Lock()
	defer apu.mutex.Unlock()
	return apu.irqPending
}

// ClearIRQ allows the CPU to acknowledge and clear the IRQ flag.
func (apu *APU) ClearIRQ() {
	apu.mutex.Lock()
	apu.irqPending = false
	apu.mutex.Unlock()
	if DebugAudio {
		log.Println("APU: CPU Cleared IRQ.")
	}
}

// Shutdown stops the audio stream and cleans up resources.
func (apu *APU) Shutdown() {
	log.Println("Shutting down APU...")
	apu.mutex.Lock() // Acquire lock before closing resources

	// Close the buffer swap channel to stop the manager goroutine
	close(apu.bufferSwap)

	// Stop and close the audio stream
	if apu.stream != nil {
		if err := apu.stream.Stop(); err != nil {
			log.Printf("PortAudio Stop Stream Error: %v", err)
		}
		if err := apu.stream.Close(); err != nil {
			log.Printf("PortAudio Close Stream Error: %v", err)
		}
		apu.stream = nil
	}

	// Terminate PortAudio
	if err := portaudio.Terminate(); err != nil {
		log.Printf("PortAudio Termination Error: %v", err)
	}

	apu.mutex.Unlock() // Release lock after cleanup
	log.Println("APU Shutdown complete.")
}