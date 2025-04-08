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
	"log"
	"zerojnt/apu"
	"zerojnt/cartridge"
	"zerojnt/debug"
	"zerojnt/ioports"
	"zerojnt/ppu"
)

type CPU struct {
	Name        string
	A           byte
	X           byte
	Y           byte
	P           byte
	PC          uint16
	lastPC      uint16
	SP          byte
	CYC         uint16
	CYCSpecial  uint16
	PageCrossed byte
	Running     bool
	Start       int
	End         int
	SwitchTimes int
	D           debug.Debug
	IO          ioports.IOPorts
	APU         *apu.APU
	ppu         *ppu.PPU
	cycleCount  uint64
}

// StartCPU initializes the CPU, APU, and sets default values
func StartCPU() CPU {
	var cpu CPU
	cpu.Name = "Ricoh 2A03"
	cpu.Start = 0
	cpu.End = 0xFFFF
	cpu.SwitchTimes = -1

	// Initialize CPU registers and flags with default values
	ResetCPU(&cpu)

	// Initialize APU after resetting the CPU
	var err error
	cpu.APU, err = apu.NewAPU()
	if err != nil {
		log.Fatalf("Failed to initialize APU: %v", err)
	}
	
	return cpu
}

// ResetCPU resets the CPU to its initial power-up state (except for PC)
func ResetCPU(cpu *CPU) {
	cpu.A = 0
	cpu.X = 0
	cpu.Y = 0
	cpu.P = 0x24 // IRQ disabled (I=1), Bit 5 is always 1
	cpu.SP = 0xFD // Stack pointer starts at $01FD and grows downwards
	cpu.CYC = 0
	cpu.CYCSpecial = 0
	cpu.PageCrossed = 0
	cpu.Running = true
}

// SetResetVector sets the Program Counter (PC) to the reset vector address ($FFFC/$FFFD) from the cartridge.
func SetResetVector(cpu *CPU, cart *cartridge.Cartridge) {
	if cart == nil || cart.Mapper == nil {
		log.Fatal("SetResetVector called with nil cartridge or mapper")
		return
	}
	
	lowByte := RM(cpu, cart, 0xFFFC)
	highByte := RM(cpu, cart, 0xFFFD)
	cpu.PC = LE(lowByte, highByte)
	cpu.CYC = 7 // Initialize cycle count for reset sequence
}

// SetPPU links the PPU instance to the CPU instance.
func (c *CPU) SetPPU(p *ppu.PPU) {
	c.ppu = p
}

// Process executes a single CPU step (handle cycles, execute instruction).
func Process(cpu *CPU, cart *cartridge.Cartridge) {
	if cpu.Running {
		if cpu.IO.CPU_CYC_INCREASE > 0 {
			cpu.CYC += cpu.IO.CPU_CYC_INCREASE
			cpu.IO.CPU_CYC_INCREASE = 0
		}

		if cpu.CYC > 0 {
			cpu.CYC--
			return
		}

		emulate(cpu, cart)
	}
}

// WriteMemory is a convenience function that delegates writes
func WriteMemory(cpu *CPU, cart *cartridge.Cartridge, addr uint16, value byte) {
	WM(cpu, cart, addr, value)
}

// ReadMemory is a convenience function that delegates reads
func ReadMemory(cpu *CPU, cart *cartridge.Cartridge, addr uint16) byte {
	return RM(cpu, cart, addr)
}

// ZeroFlag sets or clears the Zero flag (Z) in the status register (P)
func ZeroFlag(cpu *CPU, value uint16) {
	SetZ(cpu, BoolToByte((value&0xFF) == 0))
}

// NegativeFlag sets or clears the Negative flag (N) in the status register (P)
func NegativeFlag(cpu *CPU, value uint16) {
	SetN(cpu, byte((value&0x80)>>7))
}

// CarryFlag sets or clears the Carry flag (C) in the status register (P)
func CarryFlag(cpu *CPU, value uint16) {
	SetC(cpu, BoolToByte(value > 0xFF))
}

// Cleanup shuts down associated components like the APU.
func Cleanup(cpu *CPU) {
	if cpu.APU != nil {
		cpu.APU.Shutdown()
	}
}

// BoolToByte is a utility function used by flag setting logic.
func BoolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}