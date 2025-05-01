package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"zerojnt/cartridge"
	"zerojnt/cpu"
	"zerojnt/debug"
	"zerojnt/input"
	"zerojnt/ioports"
	"zerojnt/ppu"
	"github.com/veandco/go-sdl2/sdl"
)

const (
	cpuFrequency = 1789773
	framesPerSecond = 60
	cpuCyclesPerFrameF = float64(cpuFrequency) / float64(framesPerSecond)
	frameTime = time.Second / framesPerSecond
	ppuCyclesPerCpuCycle = 3
	ppuBatchSize = 256
)

type Emulator struct {
	Running       bool
	Paused        bool
	cycleCount    uint64
	leftover      float64
	lastFrameTime time.Time
	renderCounter int
}

var (
	Cart           *cartridge.Cartridge
	Nescpu         cpu.CPU
	Nesppu         *ppu.PPU
	Nesio          ioports.IOPorts
	frameSkipPercent *int
	NesInput       *input.InputHandler
	Debug          debug.Debug
	PPUDebug       debug.PPUDebug
	Alphanes       Emulator
)

func main() {
	defer cleanup()

	frameSkipPercent = flag.Int("skip", 0, "Percentage of frames to skip rendering (0-99)")
	flag.Parse()

	if *frameSkipPercent < 0 || *frameSkipPercent > 99 {
		log.Fatalf("Error: Frame skip percentage must be between 0 and 99.")
	}

	romFile := flag.Arg(0)
	
	if romFile == "" {
		fmt.Println("Usage: alphanes [options] <rom-file> [debug-file]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	fmt.Printf("Loading %s (Frame Skip: %d%%)\n", romFile, *frameSkipPercent)
	var err error
	Cart, err = cartridge.LoadRom(romFile)
	if err != nil {
		log.Fatalf("Failed to load ROM: %v", err)
	}

	setupDebugMode()
	initializeEmulator()
	emulate()
}

func setupDebugMode() {
	debugFile := flag.Arg(1)
	if debugFile != "" && strings.Contains(debugFile, ".debug") {
		fmt.Printf("Debug mode is on using %s\n", debugFile)
		Debug = debug.OpenDebugFile(debugFile)
	} else {
		Debug.Enable = false
	}

	if debugFile != "" && strings.Contains(debugFile, ".ppu") {
		PPUDebug = debug.OpenPPUDumpFile(debugFile)
		PPUDebug.Enable = true
	} else {
		PPUDebug.Enable = false
	}
}

func initializeEmulator() {
	Nescpu = cpu.StartCPU()

	Nesio = ioports.StartIOPorts(Cart)
	Nescpu.IO = Nesio
	Nescpu.D = Debug
	Nescpu.D.Verbose = true

	cpu.SetResetVector(&Nescpu, Cart)

	fmt.Printf("PC after SetResetVector: %04X\nPRG[0x3FFC]: %02X\nPRG[0x3FFD]: %02X\n",
		Nescpu.PC, Cart.PRG[0x3FFC], Cart.PRG[0x3FFD])

	var errPPU error
	Nesppu, errPPU = ppu.StartPPU(&Nescpu.IO, Cart)
	if errPPU != nil {
		log.Fatalf("Failed to initialize PPU: %v", errPPU)
	}

	Nescpu.SetPPU(Nesppu)

	NesInput = input.NewInputHandler(Nesppu.IO)

	Alphanes = Emulator{
		Running:       true,
		Paused:        false,
		cycleCount:    0,
		leftover:      0,
		lastFrameTime: time.Now(),
		renderCounter: 0,
	}
}

func cleanup() {
	if Nescpu.APU != nil {
		Nescpu.APU.Shutdown()
	}

	if Nesppu != nil {
		Nesppu.Cleanup()
	}
}

func emulate() {
	fmt.Printf("Entering emulate(), PC: %04X\n", Nescpu.PC)

	if strings.HasSuffix(flag.Arg(0), "nestest.nes") {
		if Nescpu.PC == 0xC004 {
			Nescpu.PC = 0xC000
			fmt.Printf("  emulate() - Manually set PC to: %04X for nestest.nes\n", Nescpu.PC)
		}
	}

	cyclesThisFrame := uint64(0)
	frameCount := uint64(0)

	lastPerformanceReport := time.Now()
	framesProcessed := uint64(0)

	for Alphanes.Running && Nescpu.Running {
		now := time.Now()
		elapsedSinceLastFrame := now.Sub(Alphanes.lastFrameTime)

		if !Alphanes.Paused {
			if elapsedSinceLastFrame >= frameTime || cyclesThisFrame == 0 {
				budget := cpuCyclesPerFrameF + Alphanes.leftover
				cyclesBudget := int(budget)
				Alphanes.leftover = budget - float64(cyclesBudget)

				for cyclesThisFrame < uint64(cyclesBudget) {
					batchSize := ppuBatchSize
					if cyclesThisFrame+uint64(batchSize) > uint64(cyclesBudget) {
						batchSize = int(uint64(cyclesBudget) - cyclesThisFrame)
					}

					for i := 0; i < batchSize; i++ {
						cpu.Process(&Nescpu, Cart)

						for j := 0; j < ppuCyclesPerCpuCycle; j++ {
							ppu.Process(Nesppu)
						}

						if Nescpu.APU != nil {
							Nescpu.APU.Clock()
						}

						Alphanes.cycleCount++
						cyclesThisFrame++

						if cyclesThisFrame >= uint64(cyclesBudget) {
							break
						}
					}
				}

				sdl.PumpEvents()
				for processed := 0; processed < 6; processed++ {
					currentEvent := sdl.PollEvent()
					if currentEvent == nil {
						break
					}

					NesInput.HandleEvent(currentEvent)

					switch e := currentEvent.(type) {
					case sdl.KeyboardEvent:
						keyName := sdl.GetKeyName(e.Keysym.Sym)
						isPressed := (e.State == sdl.PRESSED)

						if keyName == "Escape" && isPressed {
							fmt.Printf("DEBUG: Escape key pressed, quitting application\n")
							return
						}
					}
				}

				cyclesThisFrame = 0
				frameCount++

				shouldRender := true
				if *frameSkipPercent > 0 {
					renderDecisionValue := 100 - *frameSkipPercent
					if Alphanes.renderCounter >= renderDecisionValue {
						shouldRender = false
					}
					Alphanes.renderCounter++
					if Alphanes.renderCounter >= 100 {
						Alphanes.renderCounter = 0
					}
				}
				Nesppu.SetSkipRender(!shouldRender)

				framesProcessed++
				Alphanes.lastFrameTime = now

				if time.Since(lastPerformanceReport) >= 5*time.Second {
					timeElapsed := time.Since(lastPerformanceReport).Seconds()
					fps := float64(framesProcessed) / timeElapsed

					avgCyclesPerFrame := float64(cpuFrequency) / float64(framesPerSecond)
					cyclesPerSecond := float64(framesProcessed) * avgCyclesPerFrame / timeElapsed
					cpuPercentage := (cyclesPerSecond / float64(cpuFrequency)) * 100

					fmt.Printf("Performance: %.2f FPS (target: %d) - CPU utilization: %.1f%%\n",
						fps, framesPerSecond, cpuPercentage)

					lastPerformanceReport = time.Now()
					framesProcessed = 0
				}
			} else {
				sleepDuration := frameTime - elapsedSinceLastFrame
				if sleepDuration > time.Millisecond {
					time.Sleep(sleepDuration / 2)
				} else {
					time.Sleep(time.Millisecond)
				}
			}
		} else {
			time.Sleep(16 * time.Millisecond)
		}
	}
}