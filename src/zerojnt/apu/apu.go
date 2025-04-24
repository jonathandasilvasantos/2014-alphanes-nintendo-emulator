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
	SampleRate        = 16000
	BufferSizeSamples = 3072
	RingBufferSize    = 4096

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
func NewAPU() (*APU, error) {
	log.Println("Initializing APU with Lock-Free Ring Buffer...")

	apu := &APU{
		// Pass CpuClockSpeed and SampleRate to NewPulseChannel
		pulse1:     channels.NewPulseChannel(1, CpuClockSpeed, float64(SampleRate)),
		pulse2:     channels.NewPulseChannel(2, CpuClockSpeed, float64(SampleRate)),
		triangle:   channels.NewTriangleChannel(), // Triangle channel doesn't need clock/sample rate passed
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
			out[i] = 0.0 // Output silence on underrun
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
	// Triangle channel's phase is advanced during its Output() call.
	// It does not have a timer clocked directly by the CPU cycle like pulse/noise.
	// ---> The line apu.triangle.ClockTimer() has been removed here <---

	// Clock Pulse and Noise channels every other CPU cycle (as they run at half CPU speed)
	apu.cpuCycleCounter++
	if apu.cpuCycleCounter%2 == 0 {
		apu.pulse1.ClockTimer() // Pulse 1/2 timer clocking (if needed by implementation)
		apu.pulse2.ClockTimer() // Pulse 1/2 timer clocking (if needed by implementation)
		apu.noise.ClockTimer()  // Noise timer clocking
	}

	// Frame Counter Clocking (runs at CPU Speed / CpuCyclesPerFrameStep)
	apu.frameCounterCycles += 1.0
	if apu.frameCounterCycles >= CpuCyclesPerFrameStep {
		apu.frameCounterCycles -= CpuCyclesPerFrameStep
		apu.clockFrameSequencer() // Clocks envelopes, lengths, sweeps
	}

	// Audio Sample Generation (runs at CPU Speed / CpuCyclesPerAudioSample)
	apu.sampleGenCycles += 1.0
	if apu.sampleGenCycles >= CpuCyclesPerAudioSample {
		apu.sampleGenCycles -= CpuCyclesPerAudioSample
		apu.generateSample() // Generate one audio sample and put it in the buffer
	}
}

// clockFrameSequencer advances the frame counter state (4 or 5 steps).
func (apu *APU) clockFrameSequencer() {
	// No lock needed here, as it's called within Clock(), which should be synchronized externally if needed,
	// OR this function is called from within the lock in writeRegisterInternal for $4017 side effects.
	// Taking the lock here could lead to deadlocks if called from writeRegisterInternal.
	// EDIT: Added lock/unlock here as it's called from Clock() which doesn't hold the lock.
	// This ensures consistency when reading/modifying sequence state vs register writes.
	apu.regMu.Lock()
	defer apu.regMu.Unlock()


	step := apu.frameSequenceStep

	// Determine which units to clock based on mode and step
	var clockEnvelopes, clockLengthsSweeps bool
	if apu.sequenceMode5Step { // 5-Step Sequence (Mode 1)
		clockEnvelopes = (step == 0 || step == 1 || step == 2 || step == 4)
		clockLengthsSweeps = (step == 1 || step == 4)
	} else { // 4-Step Sequence (Mode 0)
		clockEnvelopes = true                     // Clock envelopes on steps 0, 1, 2, 3
		clockLengthsSweeps = (step == 1 || step == 3) // Clock lengths/sweeps on steps 1, 3
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
// Called by clockFrameSequencer, assumes lock is held if necessary.
func (apu *APU) clockEnvelopesAndLinear() {
	apu.pulse1.ClockEnvelope()
	apu.pulse2.ClockEnvelope()
	apu.triangle.ClockLinearCounter() // Triangle linear counter is clocked here
	apu.noise.ClockEnvelope()
}

// clockLengthAndSweep clocks length counters and sweep units.
// Called by clockFrameSequencer, assumes lock is held if necessary.
func (apu *APU) clockLengthAndSweep() {
	apu.pulse1.ClockLengthCounter()
	apu.pulse1.ClockSweep()
	apu.pulse2.ClockLengthCounter()
	apu.pulse2.ClockSweep()
	apu.triangle.ClockLengthCounter() // Triangle length counter is clocked here
	apu.noise.ClockLengthCounter()
}

// generateSample creates one audio sample and pushes it to the lock-free ring buffer.
func (apu *APU) generateSample() {
	// Get channel outputs (no lock needed for reading output values)
	p1Out := apu.pulse1.Output()
	p2Out := apu.pulse2.Output()
	triOut := apu.triangle.Output()
	noiOut := apu.noise.Output()
	dmcOut := float32(0.0) // Placeholder for DMC output

	// Mix channels using the canonical mixer
	newSample := apu.mixer.MixChannels(p1Out, p2Out, triOut, noiOut, dmcOut)

	// Apply simple low-pass filter (optional, adjust alpha for effect)
	// Alpha = 0.8 new, 0.2 previous
	filtered := (newSample * 0.8) + (apu.previousSample * 0.2)
	apu.previousSample = filtered

	// --- Store into the lock-free ring buffer ---
	rb := apu.ring
	// 1. Read current write index (atomically)
	currentWriteIdx := atomic.LoadUint32(&rb.writeIdx)
	// 2. Calculate next write index
	nextWriteIdx := (currentWriteIdx + 1) & rb.mask

	// 3. Check if buffer is full (next write index would collide with read index)
	if nextWriteIdx == atomic.LoadUint32(&rb.readIdx) {
		// Ring buffer full - Overrun! Increment counter and drop sample.
		atomic.AddUint64(&apu.bufferStats.overruns, 1)
		if DebugAudio {
			// Log overrun moderately, not on every occurrence
			if time.Since(apu.bufferStats.lastReport) > 1*time.Second {
				overruns := atomic.LoadUint64(&apu.bufferStats.overruns)
				log.Printf("APU Warning: Ring buffer overrun! Count: %d", overruns)
				apu.bufferStats.lastReport = time.Now() // Reset report time on log
			}
		}
		return // Sample is dropped
	}

	// 4. Write sample to buffer at the current write index
	rb.data[currentWriteIdx] = filtered

	// 5. Advance write index atomically (make the written sample visible to consumer)
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
		// DMC registers - TODO: Implement DMC channel and writes
		// apu.dmc.WriteRegister(addr, value)
	case addr == 0x4015: // Channel Enable / Status ($4015)
		apu.pulse1.SetEnabled((value & 0x01) != 0)
		apu.pulse2.SetEnabled((value & 0x02) != 0)
		apu.triangle.SetEnabled((value & 0x04) != 0)
		apu.noise.SetEnabled((value & 0x08) != 0)
		// TODO: apu.dmc.SetEnabled((value & 0x10) != 0)

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

		// If IRQ inhibit flag is set, clear any pending IRQ immediately
		if apu.inhibitIRQ {
			apu.irqPending = false
		}

		// Frame counter reset behavior (Reset occurs ~3-4 CPU cycles after write)
		// We'll simplify and reset immediately here.
		apu.frameCounterCycles = 0 // Reset cycle counter for next frame step
		apu.frameSequenceStep = 0  // Reset sequence step

		// Side effect: If mode 1 (5-step) is set, clock Length/Sweep and Envelopes/Linear immediately.
		if apu.sequenceMode5Step {
			// Call these directly - they don't modify state protected by regMu in a conflicting way here.
			apu.clockEnvelopesAndLinear()
			apu.clockLengthAndSweep()
			if DebugAudio {
				log.Println("APU: Mode 5 set, immediate clocking triggered.")
			}
		}

	default:
		// Ignore writes to unused registers in the APU range (e.g., $4014 is OAMDMA handled by CPU/PPU)
	}
}

// ReadStatus reads the APU status register ($4015).
func (apu *APU) ReadStatus() byte {
	apu.regMu.Lock()
	// Capture IRQ status before potentially clearing it
	frameIRQ := apu.irqPending
	apu.regMu.Unlock() // Unlock earlier, status calls below don't need the main lock

	var status byte
	// Use channel-specific methods to check length counter status
	if apu.pulse1.IsLengthCounterActive() {
		status |= 0x01
	}
	if apu.pulse2.IsLengthCounterActive() {
		status |= 0x02
	}
	// ---> Use the method added to TriangleChannel <---
	if apu.triangle.IsLengthCounterActive() {
		status |= 0x04
	}
	if apu.noise.IsLengthCounterActive() {
		status |= 0x08
	}
	// TODO: Add DMC status bit if apu.dmc.IsSamplePlaybackActive() { status |= 0x10 }

	// Frame IRQ Status
	if frameIRQ {
		status |= 0x40
		if DebugAudio {
			// Less spammy logging can go here if needed
		}
	}

	// --- Critical Section for Clearing IRQ ---
	apu.regMu.Lock()
	// Reading $4015 clears the frame interrupt flag *after* its status is returned.
	apu.irqPending = false
	apu.regMu.Unlock()
	// --- End Critical Section ---


	if DebugAudio {
		// Log the status read AFTER clearing the flag
		log.Printf("APU Read $4015 = %02X (Frame IRQ Pending Before Read: %v -> Now: false)", status, frameIRQ)
	}

	return status
}

// IRQ returns true if the frame counter generated an interrupt.
// Needs lock as irqPending is modified by multiple goroutines (CPU via Write/Read, APU via Clock).
func (apu *APU) IRQ() bool {
	apu.regMu.Lock()
	defer apu.regMu.Unlock()
	return apu.irqPending
}

// ClearIRQ allows the CPU to acknowledge and clear the frame IRQ flag explicitly.
// This might be useful if the IRQ line is connected directly, rather than polled via $4015.
// Needs lock for safe modification of irqPending.
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

	// Stop and close the audio stream - needs lock to safely access apu.stream
	apu.regMu.Lock()
	streamToClose := apu.stream // Copy to local var before setting to nil
	apu.stream = nil             // Prevent further use
	apu.regMu.Unlock()

	if streamToClose != nil {
		log.Println("Stopping PortAudio stream...")
		// Stop might block, do it outside the lock if possible, but closing needs care
		if err := streamToClose.Stop(); err != nil {
			// Log non-fatal error, might happen if already stopped
			log.Printf("PortAudio Stop Stream Error: %v", err)
		} else {
			log.Println("PortAudio stream stopped.")
		}

		log.Println("Closing PortAudio stream...")
		if err := streamToClose.Close(); err != nil {
			log.Printf("PortAudio Close Stream Error: %v", err)
		} else {
			log.Println("PortAudio stream closed.")
		}
	} else {
		log.Println("PortAudio stream was already nil.")
	}


	// Terminate PortAudio (usually safe to call multiple times, but best once)
	log.Println("Terminating PortAudio...")
	if err := portaudio.Terminate(); err != nil {
		log.Printf("PortAudio Termination Error: %v", err)
	}

	log.Println("APU Shutdown complete.")
}