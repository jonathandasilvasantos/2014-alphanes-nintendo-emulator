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
	// Audio configuration
	SampleRate        = 44100
	BufferSizeSamples = 8192          // ← fewer underruns
	RingBufferSize    = BufferSizeSamples * 4
	batchSamples      = 8             // number of samples mixed per call

	// Debug flags
	DebugAudio        = false
	LogBufferStats    = false
	LogRegisterWrites = false
	LogIRQ            = false

	// NES timing constants (NTSC)
	CpuClockSpeed    = 1789773.0
	FrameCounterRate = 240.0
	CpuClockSpeedInt = 1789773
	
	// --- Cadência exata NTSC (7457,5 ciclos de CPU) ---
	frameStepCyclesLong  int64 = 7458 // primeiro passo do sequenciador
	frameStepCyclesShort int64 = 7457 // todos os demais
)

var (
	// Calculate cycles per step using integer math for precision
	CpuCyclesPerFrameStepInt = int64(CpuClockSpeedInt / int(FrameCounterRate))
	CpuCyclesPerAudioSampleInt = int64(CpuClockSpeedInt / SampleRate)
)

// APU represents the NES Audio Processing Unit.
type APU struct {
	pulse1   *channels.PulseChannel
	pulse2   *channels.PulseChannel
	triangle *channels.TriangleChannel
	noise    *channels.NoiseChannel
	dmc      *channels.DMCChannel
	mixer    *Mixer

	// Audio output buffering
	ring   *ringBuf
	stream *portaudio.Stream
	regMu  sync.RWMutex

	bufferStats struct {
		underruns  uint64
		overruns   uint64
		lastReport time.Time
	}

	// Frame counter state
	frameSequenceStep int
	sequenceMode5Step bool
	inhibitIRQ        bool
	irqPending        bool

	// Timing counters
	frameCounterCycleCounter int64
	sampleGenCycleCounter    int64
	cpuCycleCounter          uint64
	currentStepCycles int64 // duração do passo atual do frame-sequencer
}

// NewAPU creates and initializes a new APU instance.
func NewAPU() (*APU, error) {
	log.Println("Initializing APU...")

	apu := &APU{
		pulse1:   channels.NewPulseChannel(1, CpuClockSpeed, float64(SampleRate)),
		pulse2:   channels.NewPulseChannel(2, CpuClockSpeed, float64(SampleRate)),
		triangle: channels.NewTriangleChannel(CpuClockSpeed),
		noise:    channels.NewNoiseChannel(),
		dmc:      channels.NewDMCChannel(),
		mixer:    NewMixer(),
		ring:     newRing(RingBufferSize),
		bufferStats: struct {
			underruns  uint64
			overruns   uint64
			lastReport time.Time
		}{
			lastReport: time.Now(),
		},
		frameCounterCycleCounter: 0,
		sampleGenCycleCounter:    0,
		currentStepCycles: frameStepCyclesLong,
	}

	// Initialize APU registers
	apu.regMu.Lock()
	apu.writeRegisterInternal(0x4017, 0x00)
	apu.writeRegisterInternal(0x4015, 0x00)
	apu.regMu.Unlock()

	// Reset channels
	apu.pulse1.Reset()
	apu.pulse2.Reset()
	apu.triangle.Reset()
	apu.noise.Reset()
	apu.dmc.Reset()

	// Initialize PortAudio
	if err := portaudio.Initialize(); err != nil {
		log.Printf("PortAudio Initialization Error: %v", err)
		return nil, err
	}

	// Open audio stream
	stream, err := portaudio.OpenDefaultStream(
		0,
		1,
		float64(SampleRate),
		BufferSizeSamples,
		apu.audioCallback,
	)
	if err != nil {
		log.Printf("PortAudio Open Stream Error: %v", err)
		portaudio.Terminate()
		return nil, err
	}
	apu.stream = stream

	// Start stream
	if err := stream.Start(); err != nil {
		log.Printf("PortAudio Start Stream Error: %v", err)
		stream.Close()
		portaudio.Terminate()
		return nil, err
	}
	log.Printf("PortAudio Stream Started (SampleRate: %d, BufferSize: %d)", SampleRate, BufferSizeSamples)

	log.Println("APU Initialization Complete.")
	return apu, nil
}

// audioCallback is called by PortAudio when it needs more audio data.
func (apu *APU) audioCallback(out []float32) {
	rb := apu.ring
	samplesRequested := len(out)
	samplesGenerated := 0

	for i := range out {
		currentReadIdx := atomic.LoadUint32(&rb.readIdx)
		currentWriteIdx := atomic.LoadUint32(&rb.writeIdx)

		if currentReadIdx == currentWriteIdx {
			// Buffer empty - underrun
			atomic.AddUint64(&apu.bufferStats.underruns, 1)
			out[i] = 0.0
			continue
		}

		// Read sample from buffer
		out[i] = rb.data[currentReadIdx]
		samplesGenerated++

		// Advance read index
		atomic.StoreUint32(&rb.readIdx, (currentReadIdx+1)&rb.mask)
	}

	// Debug logging
	if LogBufferStats {
		underrunOccurred := samplesGenerated < samplesRequested
		now := time.Now()
		if underrunOccurred || now.Sub(apu.bufferStats.lastReport) > 5*time.Second {
			underruns := atomic.LoadUint64(&apu.bufferStats.underruns)
			overruns := atomic.LoadUint64(&apu.bufferStats.overruns)

			finalReadIdx := atomic.LoadUint32(&rb.readIdx)
			finalWriteIdx := atomic.LoadUint32(&rb.writeIdx)

			if underruns > 0 || overruns > 0 || now.Sub(apu.bufferStats.lastReport) > 5*time.Second {
				fillLevel := (finalWriteIdx - finalReadIdx + uint32(len(rb.data))) & rb.mask
				log.Printf("APU Stats (Callback): Underruns=%d (Occurred: %v), Overruns=%d, Samples Req=%d, Gen=%d, Fill=%d/%d",
					underruns, underrunOccurred, overruns, samplesRequested, samplesGenerated, fillLevel, len(rb.data))
				apu.bufferStats.lastReport = now
			}
		}
	}
}

// Clock advances the APU state by one CPU cycle.
func (apu *APU) Clock() {
	// Clock triangle channel timer (runs at CPU speed)
	apu.triangle.ClockTimer()

	// Clock pulse, noise, and DMC timers (run at half CPU speed)
	apu.cpuCycleCounter++
	runHalfSpeed := apu.cpuCycleCounter%2 == 0
	if runHalfSpeed {
		apu.pulse1.ClockTimer()
		apu.pulse2.ClockTimer()
		apu.noise.ClockTimer()
	}

	// Frame counter clocking
	apu.frameCounterCycleCounter++
	if apu.frameCounterCycleCounter >= apu.currentStepCycles {
		apu.frameCounterCycleCounter -= apu.currentStepCycles

		// Alterna 7458 / 7457 para manter 7457,5 de média
		if apu.currentStepCycles == frameStepCyclesLong {
			apu.currentStepCycles = frameStepCyclesShort
		} else {
			apu.currentStepCycles = frameStepCyclesLong
		}
		apu.clockFrameSequencer()
	}

	// Audio sample generation
	apu.sampleGenCycleCounter++
	if apu.sampleGenCycleCounter >= CpuCyclesPerAudioSampleInt {
		// Mix several samples in one go
		needed := int(apu.sampleGenCycleCounter / CpuCyclesPerAudioSampleInt)
		if needed > batchSamples {
			needed = batchSamples
		}
		apu.sampleGenCycleCounter -= int64(needed) * CpuCyclesPerAudioSampleInt
		apu.generateSamples(needed)
	}
}

// clockFrameSequencer advances the frame counter state.
func (apu *APU) clockFrameSequencer() {
	apu.regMu.Lock()
	defer apu.regMu.Unlock()

	step := apu.frameSequenceStep

	// Determine which units to clock
	clockQuarterFrame := false
	clockHalfFrame := false

	if apu.sequenceMode5Step {
		// 5-Step Sequence (Mode 1)
		clockQuarterFrame = (step == 0 || step == 1 || step == 2 || step == 4)
		clockHalfFrame = (step == 1 || step == 4)
	} else {
		// 4-Step Sequence (Mode 0)
		clockQuarterFrame = true
		clockHalfFrame = (step == 1 || step == 3)
		
		// Trigger IRQ on step 3 if not inhibited
		if step == 3 && !apu.inhibitIRQ {
			apu.irqPending = true
			if LogIRQ {
				log.Printf("APU: Frame IRQ Triggered (Step 3, Mode 0)")
			}
		}
	}

	// Clock the units
	if clockQuarterFrame {
		apu.clockEnvelopesAndLinear()
	}
	if clockHalfFrame {
		apu.clockLengthAndSweep()
	}

	// Advance step counter
	maxSteps := 4
	if apu.sequenceMode5Step {
		maxSteps = 5
	}
	apu.frameSequenceStep = (apu.frameSequenceStep + 1) % maxSteps
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

// generateSamples generates multiple audio samples at once
func (apu *APU) generateSamples(n int) {
	for i := 0; i < n; i++ {
		apu.generateSample()
	}
}

// generateSample creates one audio sample and pushes it to the ring buffer.
func (apu *APU) generateSample() {
	// Snapshot register-mutable state quickly
	apu.regMu.RLock()
	p1 := apu.pulse1
	p2 := apu.pulse2
	tr := apu.triangle
	nz := apu.noise
	//d := apu.dmc
	apu.regMu.RUnlock()

	// Compute outputs **after** the lock
	p1Out := p1.Output()
	p2Out := p2.Output()
	triOut := tr.Output()
	noiOut := nz.Output()
	dmcOut := float32(0.0) // DMC not yet implemented

	// Mix channels
	mixedSample := apu.mixer.MixChannels(p1Out, p2Out, triOut, noiOut, dmcOut)

	// Store into the ring buffer
	rb := apu.ring
	currentWriteIdx := atomic.LoadUint32(&rb.writeIdx)
	nextWriteIdx := (currentWriteIdx + 1) & rb.mask

	// Check if buffer is full
	if nextWriteIdx == atomic.LoadUint32(&rb.readIdx) {
		// Buffer full - overrun
		atomic.AddUint64(&apu.bufferStats.overruns, 1)
		if LogBufferStats && time.Since(apu.bufferStats.lastReport) > 1*time.Second {
			overruns := atomic.LoadUint64(&apu.bufferStats.overruns)
			log.Printf("APU Warning: Ring buffer overrun! Count: %d", overruns)
			apu.bufferStats.lastReport = time.Now()
		}
		return
	}

	// Write sample to buffer
	rb.data[currentWriteIdx] = mixedSample

	// Advance write index
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
	if LogRegisterWrites {
		log.Printf("APU Write: Addr=$%04X, Value=$%02X", addr, value)
	}

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
		apu.dmc.WriteRegister(addr, value)
	case addr == 0x4015: // Channel Enable / Status ($4015)
		apu.pulse1.SetEnabled((value & 0x01) != 0)
		apu.pulse2.SetEnabled((value & 0x02) != 0)
		apu.triangle.SetEnabled((value & 0x04) != 0)
		apu.noise.SetEnabled((value & 0x08) != 0)
		apu.dmc.SetEnabled((value & 0x10) != 0)

		// Clear frame IRQ flag
		if apu.irqPending && LogIRQ {
			log.Printf("APU: Frame IRQ Cleared by write to $4015")
		}
		apu.irqPending = false

		// Se o DMC vier a implementar IRQ, limpe aqui também
		if apu.dmc != nil {
			apu.dmc.ClearIRQ()
		}

	case addr == 0x4017: // Frame Counter Control ($4017)
		newMode5Step := (value & 0x80) != 0
		newInhibitIRQ := (value & 0x40) != 0
		apu.irqPending = false        // <— ALWAYS clear on any $4017 write


		apu.sequenceMode5Step = newMode5Step
		apu.inhibitIRQ = newInhibitIRQ

		// Clear IRQ if inhibit flag is set
		if apu.inhibitIRQ && apu.irqPending {
			if LogIRQ {
				log.Printf("APU: Frame IRQ Cleared by inhibit flag in $4017 write")
			}
			apu.irqPending = false
		}

		// Reset counters com atraso obrigatório (3 ou 4 ciclos de CPU)
		delay := int64(3)              // Modo 0 (4-step)
		if newMode5Step {              // Modo 1 (5-step)
			delay = 4
		}
		apu.frameCounterCycleCounter = -delay
		apu.currentStepCycles        = frameStepCyclesLong
		apu.frameSequenceStep        = 0

		// Mode 1 (5-step) immediate clocking
		if apu.sequenceMode5Step {
			apu.clockEnvelopesAndLinear()
			apu.clockLengthAndSweep()
			if DebugAudio {
				log.Println("APU: Mode 5 set ($4017), immediate clocking triggered.")
			}
		}
	}
}

// ReadStatus reads the APU status register ($4015).
func (apu *APU) ReadStatus() byte {
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
	if apu.dmc.IsSamplePlaybackActive() {
		status |= 0x10
	}

	// Read and clear IRQ
	apu.regMu.Lock()
	frameIRQ := apu.irqPending
	apu.irqPending = false
	apu.regMu.Unlock()

	if frameIRQ {
		status |= 0x40
		if LogIRQ {
			log.Printf("APU Read $4015: Status=$%02X (IRQ was Pending, now cleared)", status)
		}
	} else if DebugAudio {
		log.Printf("APU Read $4015: Status=$%02X (IRQ not pending)", status)
	}

	return status
}

// IRQ returns true if the frame counter or DMC generated an interrupt.
func (apu *APU) IRQ() bool {
	apu.regMu.Lock()
	defer apu.regMu.Unlock()
	return apu.irqPending
}

// ClearIRQ allows the CPU to acknowledge and clear the APU IRQ flag.
func (apu *APU) ClearIRQ() {
	apu.regMu.Lock()
	apu.irqPending = false
	if apu.dmc != nil { apu.dmc.ClearIRQ() }
	apu.regMu.Unlock()
}

// Shutdown stops the audio stream and cleans up resources.
func (apu *APU) Shutdown() {
	log.Println("Shutting down APU...")

	apu.regMu.Lock()
	streamToClose := apu.stream
	apu.stream = nil
	apu.regMu.Unlock()

	if streamToClose != nil {
		log.Println("Closing PortAudio stream...")
		if err := streamToClose.Close(); err != nil {
			log.Printf("PortAudio Close Stream Error: %v", err)
		} else {
			log.Println("PortAudio stream closed.")
		}
	} else {
		log.Println("PortAudio stream was already nil.")
	}

	log.Println("Terminating PortAudio...")
	if err := portaudio.Terminate(); err != nil {
		log.Printf("PortAudio Termination Error: %v", err)
	}

	log.Println("APU Shutdown complete.")
}