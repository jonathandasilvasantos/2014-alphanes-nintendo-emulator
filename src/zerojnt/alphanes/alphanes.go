/*
Copyright 2014, 2015 Jonathan da Silva Santos

This file is part of Alphanes.

    Alphanes is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    Alphanes is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with Alphanes. If not, see <http://www.gnu.org/licenses/>.
*/
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"zerojnt/cartridge"
	"zerojnt/cpu"
	"zerojnt/debug"
	"zerojnt/ioports"
	"zerojnt/ppu"
)

const (
	// NES CPU runs at 1.789773 MHz (NTSC)
	cpuFrequency = 1789773

	// Target frame rate is 60 FPS (NTSC)
	framesPerSecond = 60

	// CPU cycles per frame = CPU frequency / frames per second
	cpuCyclesPerFrame = cpuFrequency / framesPerSecond

	// Duration of one frame in nanoseconds
	frameTime = time.Second / framesPerSecond

	// PPU runs at 3x the CPU clock rate
	ppuCyclesPerCpuCycle = 3
	
	// Batch size for PPU processing - increased for better performance without threading
	ppuBatchSize = 32
)

type Emulator struct {
	Running        bool
	Paused         bool
	cycleCount     uint64 // Global cycle counter
	frameTimer     *time.Ticker
}

var (
	Cart     *cartridge.Cartridge
	Nescpu   cpu.CPU
	Nesppu   *ppu.PPU
	Nesio    ioports.IOPorts
	Debug    debug.Debug
	PPUDebug debug.PPUDebug
	Alphanes Emulator
)

func main() {
	// Defer cleanup operations to ensure they run at program exit
	defer cleanup()

	if len(os.Args) < 2 {
		fmt.Println("Usage: alphanes <rom-file> [debug-file]")
		os.Exit(1)
	}

	fmt.Println("Loading " + os.Args[1])
	var err error
	Cart, err = cartridge.LoadRom(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to load ROM: %v", err)
	}

	// Setup debug if needed
	setupDebugMode()

	// Initialize components
	initializeEmulator()

	// Start emulation
	emulate()
}

func setupDebugMode() {
	// Handle debug mode
	if len(os.Args) >= 3 && strings.Contains(string(os.Args[2]), ".debug") {
		fmt.Printf("Debug mode is on\n")
		Debug = debug.OpenDebugFile(os.Args[2])
	} else {
		Debug.Enable = false
		fmt.Printf("Debug mode is off\n")
	}

	// Handle PPU debug mode
	if len(os.Args) >= 3 && strings.Contains(os.Args[2], ".ppu") {
		PPUDebug = debug.OpenPPUDumpFile(os.Args[2])
		PPUDebug.Enable = true
	}
}

func initializeEmulator() {
	// Initialize CPU first
	Nescpu = cpu.StartCPU()
	
	// Initialize IOPorts using the loaded Cartridge
	Nesio = ioports.StartIOPorts(Cart)
	Nescpu.IO = Nesio
	Nescpu.D = Debug
	Nescpu.D.Verbose = true

	// Set Reset Vector after loading the ROM and initializing the CPU
	cpu.SetResetVector(&Nescpu, Cart)

	fmt.Printf("PC after SetResetVector: %04X\nPRG[0x3FFC]: %02X\nPRG[0x3FFD]: %02X\n", 
		Nescpu.PC, Cart.PRG[0x3FFC], Cart.PRG[0x3FFD])

	// Initialize PPU
	var errPPU error
	Nesppu, errPPU = ppu.StartPPU(&Nescpu.IO, Cart)
	if errPPU != nil {
		log.Fatalf("Failed to initialize PPU: %v", errPPU)
	}

	// Link PPU to CPU
	Nescpu.SetPPU(Nesppu)

	// Initialize emulator state
	Alphanes = Emulator{
		Running:    true,
		Paused:     false,
		cycleCount: 0,
	}
}

func cleanup() {
	// Stop frame timer if it exists
	if Alphanes.frameTimer != nil {
		Alphanes.frameTimer.Stop()
	}

	// Cleanup CPU APU
	if Nescpu.APU != nil {
		Nescpu.APU.Shutdown()
	}
	
	// Cleanup PPU SDL resources
	if Nesppu != nil {
		Nesppu.Cleanup()
	}
}

func emulate() {
	fmt.Printf("Entering emulate(), PC: %04X\n", Nescpu.PC)

	// Special case for nestest.nes
	if strings.HasSuffix(os.Args[1], "nestest.nes") {
		if Nescpu.PC == 0xC004 {
			Nescpu.PC = 0xC000
			fmt.Printf("  emulate() - Manually set PC to: %04X for nestest.nes\n", Nescpu.PC)
		}
	}

	// Use a ticker for precise frame timing
	frameTicker := time.NewTicker(frameTime)
	defer frameTicker.Stop()
	
	cyclesThisFrame := uint64(0)
	frameCount := uint64(0)
	
	// Performance monitoring variables
	lastPerformanceReport := time.Now()
	framesProcessed := uint64(0)
	
	// Main emulation loop
	for Alphanes.Running && Nescpu.Running {
		if !Alphanes.Paused {
			// Process a batch of CPU cycles
			batchSize := ppuBatchSize
			if cyclesThisFrame + uint64(batchSize) > cpuCyclesPerFrame {
				// Don't exceed cycles per frame
				batchSize = int(cpuCyclesPerFrame - cyclesThisFrame)
			}
			
			if batchSize > 0 {
				// Process CPU and PPU cycles in a single optimized batch
				for i := 0; i < batchSize; i++ {
					// Execute one CPU cycle
					cpu.Process(&Nescpu, Cart)
					
					// Execute PPU cycles (3 per CPU cycle)
					for j := 0; j < ppuCyclesPerCpuCycle; j++ {
						ppu.Process(Nesppu)
					}
					
					// Update APU - Optional: could be moved outside the inner loop
					// for better performance if timing precision is less critical
					if Nescpu.APU != nil {
						Nescpu.APU.Clock()
					}
					
					// Increment global cycle count
					Alphanes.cycleCount++
				}
				
				cyclesThisFrame += uint64(batchSize)
			}
			
			// Check if we've completed a frame
			if cyclesThisFrame >= cpuCyclesPerFrame {
				// Wait for the next frame tick for consistent timing
				<-frameTicker.C
				
				// Process input once per frame for better performance
				if Nesppu != nil {
					Nesppu.CheckKeyboard()
				}
				
				cyclesThisFrame = 0
				frameCount++
				framesProcessed++
				
				// Performance reporting every 5 seconds
				if time.Since(lastPerformanceReport) >= 5*time.Second {
					timeElapsed := time.Since(lastPerformanceReport).Seconds()
					fps := float64(framesProcessed) / timeElapsed
					
					// Calculate CPU cycles per second for performance metrics
					cyclesPerSecond := float64(framesProcessed * cpuCyclesPerFrame) / timeElapsed
					cpuPercentage := (cyclesPerSecond / float64(cpuFrequency)) * 100
					
					fmt.Printf("Performance: %.2f FPS (target: %d) - CPU utilization: %.1f%%\n", 
						fps, framesPerSecond, cpuPercentage)
					
					lastPerformanceReport = time.Now()
					framesProcessed = 0
				}
			}
		} else {
			// When paused, just check for input occasionally and sleep to avoid CPU usage
			if Nesppu != nil {
				Nesppu.CheckKeyboard()
			}
			
			// Consume any frame ticks while paused
			select {
			case <-frameTicker.C:
				// Tick consumed
			default:
				// No pending tick, sleep a bit
				time.Sleep(16 * time.Millisecond)
			}
		}
	}
}