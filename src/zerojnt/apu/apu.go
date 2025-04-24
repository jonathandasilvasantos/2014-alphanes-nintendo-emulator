package apu

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gordonklaus/portaudio"
	"zerojnt/apu/channels"
)

const (
	// Audio System Configuration
	SampleRate        = 44100
	BufferSizeSamples = 1024
	RingBufferSize    = 16384

	// Debug flags
	DebugAudio = false

	// NES Timing Constants (NTSC)
	CpuClockSpeed           = 1789773.0
	FrameCounterRate        = 240.0
	CpuCyclesPerAudioSample = CpuClockSpeed / SampleRate
	CpuCyclesPerFrameStep   = CpuClockSpeed / FrameCounterRate
)

// APU represents the NES Audio Processing Unit.
type APU struct {
	pulse1   *channels.PulseChannel
	pulse2   *channels.PulseChannel
	triangle *channels.TriangleChannel
	noise    *channels.NoiseChannel
	mixer    *Mixer

	// Audio Output Buffering
	ring   *ringBuf
	stream *portaudio.Stream
	regMu  sync.Mutex

	bufferStats struct {
		underruns uint64
		overruns  uint64
		lastReport time.Time
	}

	// Frame Counter / Sequencer State
	frameCounterCycles float64
	frameSequenceStep  int
	sequenceMode5Step  bool
	inhibitIRQ         bool
	irqPending         bool

	// Sample Generation Timing
	sampleGenCycles float64

	// Low-pass filter state for noise reduction
	previousSample float32

	// CPU Synchronization
	cpuCycleCounter uint64
}

// NewAPU creates and initializes a new APU instance with the lock-free buffer.
// NewAPU creates and initializes a new APU instance with the lock-free buffer.
func NewAPU() (*APU, error) {
	log.Println("Initializing APU with Lock-Free Ring Buffer...")

	apu := &APU{
		// Pass CpuClockSpeed and SampleRate to NewPulseChannel
		pulse1:     channels.NewPulseChannel(1, CpuClockSpeed, float64(SampleRate)),
		pulse2:     channels.NewPulseChannel(2, CpuClockSpeed, float64(SampleRate)),
		triangle:   channels.NewTriangleChannel(),
		noise:      channels.NewNoiseChannel(),
		mixer:      NewMixer(),
		ring:       newRing(RingBufferSize),
		bufferStats: struct {
			underruns  uint64
			overruns   uint64
			lastReport time.Time
		}{
			lastReport: time.Now(),
		},
		// Initialize timing counters
		sampleGenCycles:    0.0,
		frameCounterCycles: 0.0,
		previousSample:     0.0,
	}

	// Initialize APU registers to documented power-on values
	apu.regMu.Lock()
	apu.writeRegisterInternal(0x4017, 0x00) // Write to frame counter control
	apu.regMu.Unlock()

	// Ensure channels start in their reset state
	apu.pulse1.Reset()
	apu.pulse2.Reset()
	apu.triangle.Reset()
	apu.noise.Reset()

	// Initialize PortAudio
	if err := portaudio.Initialize(); err != nil {
		log.Printf("PortAudio Initialization Error: %v", err)
		return nil, err
	}

	// Open Default Audio Stream
	stream, err := portaudio.OpenDefaultStream(
		0, // no input channels
		1, // mono output
		float64(SampleRate),
		BufferSizeSamples,
		apu.audioCallback,
	)
	if err != nil {
		log.Printf("PortAudio Open Stream Error: %v", err)
		portaudio.Terminate() // Clean up portaudio
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

	log.Println("APU Initialization Complete.")
	return apu, nil
}
// audioCallback is called by PortAudio when it needs more audio data.
func (apu *APU) audioCallback(out []float32) {
	rb := apu.ring

	samplesGenerated := 0
	samplesRequested := len(out)

	for i := range out {
		currentReadIdx := atomic.LoadUint32(&rb.readIdx)
		currentWriteIdx := atomic.LoadUint32(&rb.writeIdx)

		if currentReadIdx == currentWriteIdx {
			// Ring buffer is empty - Underrun!
			atomic.AddUint64(&apu.bufferStats.underruns, 1)
			out[i] = 0.0
			continue
		}

		// Read sample from buffer
		out[i] = rb.data[currentReadIdx]
		samplesGenerated++

		// Advance read index atomically
		atomic.StoreUint32(&rb.readIdx, (currentReadIdx+1)&rb.mask)
	}

	// Optional Debug Logging
	if DebugAudio {
		underrunOccurred := samplesGenerated < samplesRequested
		if underrunOccurred || time.Since(apu.bufferStats.lastReport) > 5*time.Second {
			// Load stats atomically for reporting
			underruns := atomic.LoadUint64(&apu.bufferStats.underruns)
			overruns := atomic.LoadUint64(&apu.bufferStats.overruns)
			if underruns > 0 || overruns > 0 || time.Since(apu.bufferStats.lastReport) > 5*time.Second {
				log.Printf("APU Stats (Callback): Underruns=%d (this call: %v), Overruns=%d, Samples Req=%d, Gen=%d",
					underruns, underrunOccurred, overruns, samplesRequested, samplesGenerated)
				apu.bufferStats.lastReport = time.Now()
			}
		}
	}
}

// Clock advances the APU state by one CPU cycle.
func (apu *APU) Clock() {
	// Triangle Channel Timer Clock
	apu.triangle.ClockTimer()

	// Clock other channels every other CPU cycle
	apu.cpuCycleCounter++
	if apu.cpuCycleCounter%2 == 0 {
		apu.pulse1.ClockTimer()
		apu.pulse2.ClockTimer()
		apu.noise.ClockTimer()
	}

	// Frame Counter Clocking
	apu.frameCounterCycles += 1.0
	if apu.frameCounterCycles >= CpuCyclesPerFrameStep {
		apu.frameCounterCycles -= CpuCyclesPerFrameStep
		apu.clockFrameSequencer()
	}

	// Audio Sample Generation
	apu.sampleGenCycles += 1.0
	if apu.sampleGenCycles >= CpuCyclesPerAudioSample {
		apu.sampleGenCycles -= CpuCyclesPerAudioSample
		apu.generateSample()
	}
}

// clockFrameSequencer advances the frame counter state (4 or 5 steps).
func (apu *APU) clockFrameSequencer() {
	apu.regMu.Lock()
	defer apu.regMu.Unlock()

	step := apu.frameSequenceStep

	// Determine which units to clock based on mode and step
	var clockEnvelopes, clockLengthsSweeps bool
	if apu.sequenceMode5Step { // 5-Step Sequence (Mode 1)
		clockEnvelopes = (step == 0 || step == 1 || step == 2 || step == 4)
		clockLengthsSweeps = (step == 1 || step == 4)
	} else { // 4-Step Sequence (Mode 0)
		clockEnvelopes = true
		clockLengthsSweeps = (step == 1 || step == 3)
		// Trigger IRQ on the last step (step 3) if not inhibited
		if step == 3 && !apu.inhibitIRQ {
			apu.irqPending = true
			if DebugAudio {
				log.Printf("APU: Frame IRQ triggered (Step %d, Inhibit: %v)", step, apu.inhibitIRQ)
			}
		}
	}

	// Clock the units
	if clockEnvelopes {
		apu.clockEnvelopesAndLinear()
	}
	if clockLengthsSweeps {
		apu.clockLengthAndSweep()
	}

	// Advance step counter
	if apu.sequenceMode5Step {
		apu.frameSequenceStep = (apu.frameSequenceStep + 1) % 5
	} else {
		apu.frameSequenceStep = (apu.frameSequenceStep + 1) % 4
	}
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
	apu.pulse1.ClockSweep()
	apu.pulse2.ClockLengthCounter()
	apu.pulse2.ClockSweep()
	apu.triangle.ClockLengthCounter()
	apu.noise.ClockLengthCounter()
}

// generateSample creates one audio sample and pushes it to the lock-free ring buffer.
func (apu *APU) generateSample() {
	// Get channel outputs
	p1Out := apu.pulse1.Output()
	p2Out := apu.pulse2.Output()
	triOut := apu.triangle.Output()
	noiOut := apu.noise.Output()
	dmcOut := float32(0.0)

	// Mix channels using the canonical mixer
	newSample := apu.mixer.MixChannels(p1Out, p2Out, triOut, noiOut, dmcOut)

	// Apply simple low-pass filter
	filtered := (newSample * 0.8) + (apu.previousSample * 0.2)
	apu.previousSample = filtered

	// Store into the lock-free ring buffer
	rb := apu.ring
	currentWriteIdx := atomic.LoadUint32(&rb.writeIdx)
	nextWriteIdx := (currentWriteIdx + 1) & rb.mask

	// Check if buffer is full
	if nextWriteIdx == atomic.LoadUint32(&rb.readIdx) {
		// Ring buffer full - Overrun!
		atomic.AddUint64(&apu.bufferStats.overruns, 1)
		if DebugAudio {
			// Log overrun moderately, not on every occurrence
			if time.Since(apu.bufferStats.lastReport) > 1*time.Second {
				overruns := atomic.LoadUint64(&apu.bufferStats.overruns)
				log.Printf("APU Warning: Ring buffer overrun! Count: %d", overruns)
				apu.bufferStats.lastReport = time.Now()
			}
		}
		return
	}

	// Write sample to buffer
	rb.data[currentWriteIdx] = filtered

	// Advance write index atomically
	atomic.StoreUint32(&rb.writeIdx, nextWriteIdx)
}

// WriteRegister handles CPU writes to APU registers ($4000 - $4017).
func (apu *APU) WriteRegister(addr uint16, value byte) {
	apu.regMu.Lock()
	defer apu.regMu.Unlock()
	apu.writeRegisterInternal(addr, value)
}

// writeRegisterInternal contains the logic, called by WriteRegister while holding the lock.
func (apu *APU) writeRegisterInternal(addr uint16, value byte) {
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
		}

		// Frame counter reset behavior
		apu.frameCounterCycles = 0
		apu.frameSequenceStep = 0

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
	apu.regMu.Lock()
	defer apu.regMu.Unlock()

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

	frameIRQ := apu.irqPending
	if frameIRQ {
		status |= 0x40
		if DebugAudio {
			// Less spammy logging
		}
	}
	// Reading $4015 clears the frame interrupt flag
	apu.irqPending = false

	// Log the status read AFTER clearing the flag
	if DebugAudio {
		log.Printf("APU Read $4015 = %02X (Frame IRQ Pending: %v -> false)", status, frameIRQ)
	}

	return status
}

// IRQ returns true if the frame counter generated an interrupt.
func (apu *APU) IRQ() bool {
	apu.regMu.Lock()
	defer apu.regMu.Unlock()
	return apu.irqPending
}

// ClearIRQ allows the CPU to acknowledge and clear the frame IRQ flag.
func (apu *APU) ClearIRQ() {
	apu.regMu.Lock()
	apu.irqPending = false
	apu.regMu.Unlock()
	if DebugAudio {
		log.Println("APU: CPU Cleared IRQ flag via ClearIRQ().")
	}
}

// Shutdown stops the audio stream and cleans up resources.
func (apu *APU) Shutdown() {
	log.Println("Shutting down APU...")

	// Stop and close the audio stream
	apu.regMu.Lock()
	if apu.stream != nil {
		log.Println("Stopping PortAudio stream...")
		if err := apu.stream.Stop(); err != nil {
			log.Printf("PortAudio Stop Stream Error: %v", err)
		}
		log.Println("Closing PortAudio stream...")
		if err := apu.stream.Close(); err != nil {
			log.Printf("PortAudio Close Stream Error: %v", err)
		}
		apu.stream = nil
		log.Println("PortAudio stream stopped and closed.")
	}
	apu.regMu.Unlock()

	// Terminate PortAudio
	log.Println("Terminating PortAudio...")
	if err := portaudio.Terminate(); err != nil {
		log.Printf("PortAudio Termination Error: %v", err)
	}

	log.Println("APU Shutdown complete.")
}