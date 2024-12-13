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
)

type CPU struct {
    Name        string
    A           byte          // Accumulator
    X           byte          // X Index
    Y           byte          // Y Index
    P           byte          // Status
    PC          uint16        // Program Counter 16bits
    lastPC      uint16        // Previous PC value (for debugging)
    SP          byte          // Stack Pointer
    CYC         uint16        // Current cycle count for the instruction
    CYCSpecial  uint16        // Additional cycles for specific cases
    PageCrossed byte          // Flag for page crossing during addressing
    Running     bool          // Indicates if the CPU is running
    Start       int           // Start address for execution (debugging)
    End         int           // End address for execution (debugging)
    SwitchTimes int          // Counter for debugging
    D           debug.Debug   // Debug information
    IO          ioports.IOPorts // IO Ports
    APU         *apu.APU      // APU instance
    cycleCount  uint64        // Global cycle counter
}

// StartCPU initializes the CPU, APU, and sets default values
func StartCPU() CPU {
	var cpu CPU
	cpu.Name = "Ricoh 2A03"
	cpu.Start = 0
	cpu.End = 0xFFFF
	cpu.SwitchTimes = -1

	// Initialize CPU with default values, but don't set PC yet
	ResetCPU(&cpu)

	// Initialize APU after resetting the CPU
	var err error
	cpu.APU, err = apu.NewAPU()
	if err != nil {
		log.Fatalf("Failed to initialize APU: %v", err)
	}

	fmt.Println(cpu.Name)
	return cpu
}

// ResetCPU resets the CPU to its initial state (except for PC)
func ResetCPU(cpu *CPU) {
	cpu.A = 0
	cpu.X = 0
	cpu.Y = 0
	cpu.P = 0x24 // bit 5 is always set, and interrupt disable flag is set
	// Do not set PC here; it should be set by SetResetVector after ROM loading
	cpu.SP = 0xFD // Stack pointer starts at 0xFD
	cpu.CYCSpecial = 0
	cpu.Running = true
	cpu.SwitchTimes = -1
}

// SetResetVector sets the Program Counter (PC) to the reset vector from the ROM
func SetResetVector(cpu *CPU, cart *cartridge.Cartridge) {
	cpu.PC = LE(RM(cpu, cart, 0xFFFC), RM(cpu, cart, 0xFFFD))
}

// Process executes a single CPU cycle
func Process(cpu *CPU, cart *cartridge.Cartridge) {
    if cpu.Running {
        emulate(cpu, cart)
        // Pass the global cycle count from the emulator to the APU
        if cpu.APU != nil {
            cpu.APU.Clock(cpu.cycleCount) // Pass the cycle count
        }
    }
}

// WriteMemory handles memory writes, including APU registers
func WriteMemory(cpu *CPU, cart *cartridge.Cartridge, addr uint16, value byte) {
	if addr >= 0x4000 && addr <= 0x4017 {
		cpu.APU.WriteRegister(addr, value)
		return
	}

	WM(cpu, cart, addr, value)
}

// ReadMemory handles memory reads, including APU registers
func ReadMemory(cpu *CPU, cart *cartridge.Cartridge, addr uint16) byte {
	if addr == 0x4015 {
		return cpu.APU.ReadStatus() // Read APU status register
	}

	return RM(cpu, cart, addr)
}

// ZeroFlag sets or clears the Zero flag based on the value
func ZeroFlag(cpu *CPU, value uint16) {
	SetZ(cpu, BoolToByte(value&0xFF == 0))
}

// NegativeFlag sets or clears the Negative flag based on the value

func NegativeFlag(cpu *CPU, value uint16) {
    SetN(cpu, byte(value>>7))
}


// CarryFlag sets or clears the Carry flag based on the value
func CarryFlag(cpu *CPU, value uint16) {
	SetC(cpu, BoolToByte(value > 0xFF))
}

// Cleanup shuts down the APU
func Cleanup(cpu *CPU) {
	if cpu.APU != nil {
		cpu.APU.Shutdown()
	}
}