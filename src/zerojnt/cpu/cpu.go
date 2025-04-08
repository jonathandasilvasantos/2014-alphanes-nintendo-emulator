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
package cpu

import (
	"fmt"
	"log"
	"zerojnt/apu"
	"zerojnt/cartridge"
	"zerojnt/debug"
	"zerojnt/ioports"
	"zerojnt/ppu" // <<<--- IMPORT PPU PACKAGE
)

type CPU struct {
	Name        string
	A           byte            // Accumulator
	X           byte            // X Index
	Y           byte            // Y Index
	P           byte            // Status
	PC          uint16          // Program Counter 16bits
	lastPC      uint16          // Previous PC value (for debugging)
	SP          byte            // Stack Pointer
	CYC         uint16          // Current cycle count for the instruction
	CYCSpecial  uint16          // Additional cycles for specific cases
	PageCrossed byte            // Flag for page crossing during addressing
	Running     bool            // Indicates if the CPU is running
	Start       int             // Start address for execution (debugging)
	End         int             // End address for execution (debugging)
	SwitchTimes int             // Counter for debugging
	D           debug.Debug     // Debug information
	IO          ioports.IOPorts // IO Ports (value type, not pointer)
	APU         *apu.APU        // APU instance (pointer)
	ppu         *ppu.PPU        // PPU instance (pointer) <<<--- ADDED PPU FIELD (Pointer)
	cycleCount  uint64          // Global cycle counter (passed down from emulator)
}

// StartCPU initializes the CPU, APU, and sets default values
func StartCPU() CPU {
	var cpu CPU
	cpu.Name = "Ricoh 2A03"
	cpu.Start = 0
	cpu.End = 0xFFFF
	cpu.SwitchTimes = -1 // Initialize SwitchTimes, typically starts at 0 or -1 depending on log comparison logic

	// Initialize CPU registers and flags with default values, but don't set PC yet
	ResetCPU(&cpu)

	// Initialize APU after resetting the CPU
	var err error
	cpu.APU, err = apu.NewAPU()
	if err != nil {
		log.Fatalf("Failed to initialize APU: %v", err)
	}
	// PPU is initialized and linked externally in main via SetPPU

	fmt.Println("CPU Initialized:", cpu.Name)
	return cpu
}

// ResetCPU resets the CPU to its initial power-up state (except for PC)
func ResetCPU(cpu *CPU) {
	cpu.A = 0
	cpu.X = 0
	cpu.Y = 0
	cpu.P = 0x24 // IRQ disabled (I=1), Bit 5 is always 1
	// Do not set PC here; it should be set by SetResetVector after ROM loading
	cpu.SP = 0xFD // Stack pointer starts at $01FD and grows downwards
	cpu.CYC = 0
	cpu.CYCSpecial = 0
	cpu.PageCrossed = 0
	cpu.Running = true
	// cpu.SwitchTimes = -1 // Reset counter if needed on reset
}

// SetResetVector sets the Program Counter (PC) to the reset vector address ($FFFC/$FFFD) from the cartridge.
// This should be called AFTER the ROM/Cartridge is loaded and the mapper is initialized.
func SetResetVector(cpu *CPU, cart *cartridge.Cartridge) {
	if cart == nil || cart.Mapper == nil {
		log.Fatal("SetResetVector called with nil cartridge or mapper")
		return
	}
	// Use the RM function which handles memory mapping correctly
	lowByte := RM(cpu, cart, 0xFFFC)
	highByte := RM(cpu, cart, 0xFFFD)
	cpu.PC = LE(lowByte, highByte)
	log.Printf("Reset Vector read: $%02X%02X -> PC set to $%04X\n", highByte, lowByte, cpu.PC)
	// Reset takes 7 cycles according to some sources (Nesdev wiki seems to imply 8 for reset sequence?)
	cpu.CYC = 7 // Initialize cycle count for reset sequence
}

// SetPPU links the PPU instance to the CPU instance. <<<--- ADDED METHOD
func (c *CPU) SetPPU(p *ppu.PPU) {
	c.ppu = p
	log.Println("PPU linked to CPU.") // Optional log message
}

// Process executes a single CPU step (handle cycles, execute instruction).
// It now expects the global cycle count to be updated externally.
func Process(cpu *CPU, cart *cartridge.Cartridge) {
	if cpu.Running {
		// Handle OAM DMA stall cycles potentially initiated by WM
		// The stall needs to be handled *before* executing the next instruction
		// if cpu.CYC == 0 and a DMA was just triggered.
		// Correct handling involves the main loop managing the DMA transfer bytes
		// while the CPU is stalled.
		// Simple stall handling:
		if cpu.IO.CPU_CYC_INCREASE > 0 {
			cpu.CYC += cpu.IO.CPU_CYC_INCREASE // Add stall cycles
			log.Printf("OAM DMA stall: Adding %d cycles to CPU.CYC. Current CYC=%d", cpu.IO.CPU_CYC_INCREASE, cpu.CYC)
			// TODO: Main loop should drive the actual DMA byte transfers during these cycles.
			// For now, just clear the flag after adding the cycles.
			cpu.IO.CPU_CYC_INCREASE = 0 // Clear the flag
		}

		// If CYC > 0, just decrement it (CPU is busy with previous instruction or DMA stall)
		if cpu.CYC > 0 {
			cpu.CYC--
			// If DMA was active, the main loop should be transferring bytes here.
			return // Still processing previous instruction or stalled
		}

		// If CYC is 0, execute the next instruction
		emulate(cpu, cart) // emulate will set cpu.CYC for the executed instruction

		// Note: APU clocking is now handled in the main emulator loop after CPU/PPU processing
	}
}

// WriteMemory is a convenience function, but direct WM/RM calls are often used internally.
// This function delegates writes, especially to APU/PPU registers handled differently.
func WriteMemory(cpu *CPU, cart *cartridge.Cartridge, addr uint16, value byte) {
	// WM handles the logic, including APU/PPU register ranges and OAM DMA stalls.
	// We don't need the explicit APU check here anymore as WM covers it.
	WM(cpu, cart, addr, value)
}

// ReadMemory is a convenience function. Direct RM calls are common internally.
// This function delegates reads, especially for APU/PPU registers.
func ReadMemory(cpu *CPU, cart *cartridge.Cartridge, addr uint16) byte {
	// RM handles the logic, including APU/PPU registers.
	// The explicit APU status check is handled within RM now.
	return RM(cpu, cart, addr)
}

// ZeroFlag sets or clears the Zero flag (Z) in the status register (P)
// based on whether the lower 8 bits of the value are zero.
func ZeroFlag(cpu *CPU, value uint16) {
	SetZ(cpu, BoolToByte((value&0xFF) == 0))
}

// NegativeFlag sets or clears the Negative flag (N) in the status register (P)
// based on the 7th bit (most significant bit) of the value.
func NegativeFlag(cpu *CPU, value uint16) {
	SetN(cpu, byte((value&0x80)>>7)) // Extract bit 7
}

// CarryFlag sets or clears the Carry flag (C) in the status register (P)
// based on whether the value requires more than 8 bits (e.g., > 0xFF for additions).
func CarryFlag(cpu *CPU, value uint16) {
	SetC(cpu, BoolToByte(value > 0xFF))
}

// Cleanup shuts down associated components like the APU.
func Cleanup(cpu *CPU) {
	if cpu.APU != nil {
		cpu.APU.Shutdown()
		fmt.Println("APU Shutdown called.")
	} else {
		fmt.Println("Cleanup: APU was already nil.")
	}
	// Note: PPU cleanup (SDL) should be handled in the main file where PPU is managed.
}

// BoolToByte is a utility function used by flag setting logic.
func BoolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// LE combines low and high bytes into a 16-bit word (Little-Endian).
// Already defined in bitaccess.go, but useful to have here for context if needed.
// func LE(low, high byte) uint16 {
//     return uint16(high)<<8 | uint16(low)
// }