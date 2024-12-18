package cpu

import (
	"log"
	"zerojnt/cartridge"
	"zerojnt/ioports"
)

// RM (Read Memory) reads a byte from memory, using the mapper to handle address mapping.
func RM(cpu *CPU, cart *cartridge.Cartridge, addr uint16) byte {
	if addr >= 0x2000 && addr <= 0x3FFF {
		return ioports.RMPPU(&cpu.IO, cart, addr)
	}

	prgrom, newaddr := cart.Mapper.MapCPU(addr)
	if prgrom {
		return cart.PRG[newaddr]
	} else {
		return cpu.IO.CPU_RAM[newaddr]
	}
}

// WM (Write Memory) writes a byte to memory, handling mapper writes and PPU register writes.
func WM(cpu *CPU, cart *cartridge.Cartridge, addr uint16, value byte) {
	if addr >= 0x4000 && addr <= 0x4017 {
		cpu.APU.WriteRegister(addr, value)
		return
	}

	// Handle PPU registers first
	ppuHandle := (addr >= 0x2000 && addr <= 0x3FFF) || (addr == 0x4014)

	if ppuHandle {
		ioports.WMPPU(&cpu.IO, cart, addr, value)
		return
	}

	// Handle mapper writes (typically 0x8000 and above)
	if addr >= 0x8000 {
		cart.Mapper.Write(addr, value)
		return
	}

	// Prevent direct writes to PRG-ROM for non-mapper writes
	prgrom, newaddr := cart.Mapper.MapCPU(addr)
	if prgrom {
		log.Printf("Error: Attempting to write directly to PRG-ROM at address %04X", addr)
		return
	}

	cpu.IO.CPU_RAM[newaddr] = value
}

// PushMemory pushes a byte onto the stack.
func PushMemory(cpu *CPU, v byte) {
	cpu.IO.CPU_RAM[0x0100+int(cpu.SP)] = v
	cpu.SP--
}

// PopMemory pops a byte from the stack.
func PopMemory(cpu *CPU) byte {
	cpu.SP++
	var result byte = cpu.IO.CPU_RAM[0x0100+uint(cpu.SP)]
	return result
}

// PushWord pushes a 16-bit word onto the stack (high byte first).
func PushWord(cpu *CPU, v uint16) {
	PushMemory(cpu, byte(v>>8))   // Push high byte first
	PushMemory(cpu, byte(v&0xFF)) // Push low byte next
}

// PopWord pops a 16-bit word from the stack (low byte first).
func PopWord(cpu *CPU) uint16 {
	var lo, hi byte
	lo = PopMemory(cpu)             // Pop low byte first
	hi = PopMemory(cpu)             // Pop high byte next
	return uint16(hi)<<8 | uint16(lo) // Combine bytes correctly
}