/*
Copyright 2014, 2015 Jonathan da Silva SAntos

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
	"log" // <<<--- IMPORT LOG
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

	// CPU cycles per frame = CPU frequency / frames per second (approximately)
	cpuCyclesPerFrame = cpuFrequency / framesPerSecond

	// Duration of one frame in nanoseconds
	frameTime = time.Second / framesPerSecond

	// PPU runs at 3x the CPU clock rate
	ppuCyclesPerCpuCycle = 3
)

type Emulator struct {
	Running        bool
	Paused         bool
	frameStartTime time.Time
	cycleCount     uint64 // Global cycle counter
}

var (
	Cart     cartridge.Cartridge
	Nescpu   cpu.CPU
	Nesppu   *ppu.PPU // <<<--- CHANGED TO POINTER TYPE *ppu.PPU
	Nesio    ioports.IOPorts
	Debug    debug.Debug
	PPUDebug debug.PPUDebug
	Alphanes Emulator
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: alphanes <rom-file> [debug-file]")
		os.Exit(1)
	}

	fmt.Println("Loading " + os.Args[1])
	Cart = cartridge.LoadRom(os.Args[1])

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

	// Initialize CPU first
	Nescpu = cpu.StartCPU()
	// Initialize IOPorts using the loaded Cartridge
	Nesio = ioports.StartIOPorts(&Cart) // <<<--- INITIALIZE Nesio HERE
	Nescpu.IO = Nesio                   // Assign initialized IO to CPU
	Nescpu.D = Debug
	Nescpu.D.Verbose = true

	// Set Reset Vector *after* loading the ROM and initializing the CPU
	cpu.SetResetVector(&Nescpu, &Cart)

	// DEBUG: Print PC after SetResetVector
	fmt.Printf("PC after SetResetVector: %04X\n", Nescpu.PC)

	// Initialize PPU
	var errPPU error
	Nesppu, errPPU = ppu.StartPPU(&Nescpu.IO, &Cart) // <<<--- FIXED: Assign to Nesppu pointer, pass &Nescpu.IO and &Cart, capture error
	if errPPU != nil {                              // <<<--- FIXED: Check error
		log.Fatalf("Failed to initialize PPU: %v", errPPU)
	}
	// Nesppu.D = &PPUDebug // <<<--- REMOVED/COMMENTED: PPU struct has no 'D' field

	// Link PPU to CPU (if CPU needs a direct reference, currently PPU has IO ref)
	Nescpu.SetPPU(Nesppu) // <<<--- ADDED: Assumes a SetPPU method exists in cpu.go

	// DEBUG: Print contents of cart.PRG around the reset vector
	fmt.Printf("PRG[0x3FFC]: %02X\n", Cart.PRG[0x3FFC])
	fmt.Printf("PRG[0x3FFD]: %02X\n", Cart.PRG[0x3FFD])

	// Start emulation
	Alphanes = Emulator{
		Running:        true,
		Paused:         false,
		frameStartTime: time.Now(),
		cycleCount:     0,
	}

	emulate()

	// Cleanup
	if Nescpu.APU != nil {
		Nescpu.APU.Shutdown()
	}
	if Nesppu != nil { // <<<--- ADDED: Cleanup PPU SDL resources
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

	for Alphanes.Running && Nescpu.Running {
		if !Alphanes.Paused {
			tick()
		} else {
			// Handle keyboard input even when paused (e.g., to unpause)
			if Nesppu != nil {
				Nesppu.CheckKeyboard() // Assuming CheckKeyboard exists in PPU for quit events
			}
			time.Sleep(16 * time.Millisecond) // Prevent busy-waiting when paused
		}
	}
}

func tick() {
	// Execute one CPU cycle
	cpu.Process(&Nescpu, &Cart) // Process handles internal cycle counting

	// Execute 3 PPU cycles for each CPU cycle
	for i := 0; i < ppuCyclesPerCpuCycle; i++ {
		ppu.Process(Nesppu) // <<<--- FIXED: Pass pointer Nesppu, remove Cart argument
		if Nesppu != nil {
			// Pass the global cycle count for potential PPU timing? Not typically needed directly.
		}
	}

	// Pass the global cycle count to the APU
	if Nescpu.APU != nil {
		Nescpu.APU.Clock(Alphanes.cycleCount)
	}

	// Increment global cycle count
	Alphanes.cycleCount++

	// Check if we've completed a frame (timing based on PPU frames is more accurate)
	// If Nesppu indicates frame is complete, handle timing. PPU runs 262 scanlines * 341 cycles.
	// Let's stick to the CPU-based timing for now, but PPU frame completion is better.
	if Alphanes.cycleCount%cpuCyclesPerFrame == 0 { // Approximate frame boundary
		// Calculate how long this frame took
		elapsed := time.Since(Alphanes.frameStartTime)

		// If we completed the frame too quickly, sleep for the remaining time
		if elapsed < frameTime {
			time.Sleep(frameTime - elapsed)
		}

		// Start timing the next frame
		Alphanes.frameStartTime = time.Now()

		// Check keyboard input once per frame (or more often if needed)
		if Nesppu != nil {
			// Moved CheckKeyboard to VBlank in PPU.Process for better timing
			// Nesppu.CheckKeyboard()
		}
	}
}