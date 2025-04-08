// File: cpu/memory.go
package cpu

import (
	"zerojnt/cartridge"
)

// RM (Read Memory) reads a byte from the CPU's address space.
// It handles CPU RAM, PPU registers, APU/IO registers, and Cartridge space (PRG ROM/RAM via mapper).
func RM(cpu *CPU, cart *cartridge.Cartridge, addr uint16) byte {
	switch {
	case addr < 0x2000: // CPU RAM ($0000-$07FF) mirrored up to $1FFF
		return cpu.IO.CPU_RAM[addr&0x07FF] // Read from RAM mirror

	case addr >= 0x2000 && addr <= 0x3FFF: // PPU Registers ($2000-$2007) mirrored every 8 bytes
		if cpu.ppu == nil {
			return 0
		}
		return cpu.ppu.ReadRegister(addr & 0x2007) // Use mask for mirroring

	case addr >= 0x4000 && addr <= 0x401F: // APU and I/O Registers ($4000-$401F)
		switch addr {
		case 0x4015: // APU Status Register
			if cpu.APU == nil {
				return 0
			}
			return cpu.APU.ReadStatus()

		case 0x4016: // Controller 1 Read (Joystick)
			return 0

		case 0x4017: // Controller 2 Read (Joystick)
			return 0

		default:
			return 0
		}

	case addr >= 0x6000 && addr <= 0x7FFF: // Cartridge SRAM (Battery-backed potentially)
		if cart.HasSRAM() {
			isROM, mappedAddr := cart.Mapper.MapCPU(addr)
			if !isROM {
				sramSize := uint16(cart.GetPRGRAMSize())
				if mappedAddr < sramSize {
					return cart.SRAM[mappedAddr]
				}
			}
		}
		return 0

	case addr >= 0x8000: // Cartridge PRG ROM ($8000-$FFFF)
		isROM, mappedAddr := cart.Mapper.MapCPU(addr)
		if isROM {
			prgData := cart.PRG
			if mappedAddr < uint16(len(prgData)) {
				return prgData[mappedAddr]
			}
		}
		return 0

	default: // Unhandled ranges (e.g., $4020-$5FFF Expansion ROM)
		return 0
	}
}

// WM (Write Memory) writes a byte to the CPU's address space.
// Handles CPU RAM, PPU registers, APU/IO registers, OAM DMA, and Cartridge space (Mapper/SRAM).
// Returns the number of CPU cycles to stall (specifically for OAM DMA).
func WM(cpu *CPU, cart *cartridge.Cartridge, addr uint16, value byte) (stallCycles int) {
	stallCycles = 0 // Default stall cycles to 0 (named return value)

	switch {
	case addr < 0x2000: // CPU RAM ($0000-$07FF) mirrored up to $1FFF
		cpu.IO.CPU_RAM[addr&0x07FF] = value // Write to RAM mirror
		return

	case addr >= 0x2000 && addr <= 0x3FFF: // PPU Registers ($2000-$2007) mirrored every 8 bytes
		if cpu.ppu == nil {
			return
		}
		cpu.ppu.WriteRegister(addr&0x2007, value) // Use mask for mirroring
		return

	case addr == 0x4014: // OAM DMA Register Write ($4014)
		if cpu.ppu == nil {
			return
		}

		dmaSourceAddrBase := uint16(value) << 8
		currentOAMAddr := cpu.IO.OAMADDR

		for i := 0; i < 256; i++ {
			sourceAddr := dmaSourceAddrBase + uint16(i)
			data := RM(cpu, cart, sourceAddr)

			writeAddr := currentOAMAddr + byte(i)

			if int(writeAddr) < len(cpu.IO.OAM) {
				cpu.IO.OAM[writeAddr] = data
			}
		}

		cpu.IO.StartOAMDMA(value)
		stallCycles = int(cpu.IO.CPU_CYC_INCREASE)

		return

	case addr >= 0x4000 && addr <= 0x4013 || addr == 0x4015 || addr == 0x4017: // APU Registers ($4000-$4013, $4015, $4017)
		if cpu.APU == nil {
			return
		}
		cpu.APU.WriteRegister(addr, value)
		return

	case addr == 0x4016: // Controller Strobe/Write (Joystick)
		return

	case addr >= 0x6000 && addr <= 0x7FFF: // Cartridge SRAM / PRG RAM
		if cart.HasSRAM() {
			cart.Mapper.Write(addr, value)
		} else {
			cart.Mapper.Write(addr, value)
		}
		return

	case addr >= 0x8000: // Cartridge PRG ROM space ($8000-$FFFF) -> Mapper control
		cart.Mapper.Write(addr, value)
		return

	default: // Unhandled ranges (e.g., $4020-$5FFF Expansion ROM)
		return
	}
}

// --- Stack Operations ---
// Stack resides in CPU RAM page 1 ($0100 - $01FF)

// PushMemory pushes a byte onto the stack.
// SP points to the next available free location *before* the push.
// Stack grows downwards ($FF -> $00 within page 1).
func PushMemory(cpu *CPU, v byte) {
	addr := 0x0100 | uint16(cpu.SP)
	cpu.IO.CPU_RAM[addr&0x07FF] = v // Write to stack location (handle RAM mirror just in case)
	cpu.SP--                        // Decrement stack pointer *after* write
}

// PopMemory pops a byte from the stack.
// SP points to the *last written* item. Increment SP *first* to point to the item to be read.
func PopMemory(cpu *CPU) byte {
	cpu.SP++ // Increment stack pointer *before* read
	addr := 0x0100 | uint16(cpu.SP)
	return cpu.IO.CPU_RAM[addr&0x07FF] // Read from stack location (handle RAM mirror just in case)
}

// PushWord pushes a 16-bit word onto the stack (high byte first, then low byte).
func PushWord(cpu *CPU, v uint16) {
	PushMemory(cpu, byte(v>>8))   // Push high byte (SP decrements)
	PushMemory(cpu, byte(v&0xFF)) // Push low byte (SP decrements again)
}

// PopWord pops a 16-bit word from the stack (low byte first, then high byte).
func PopWord(cpu *CPU) uint16 {
	var lo, hi byte
	lo = PopMemory(cpu) // Pop low byte (SP increments)
	hi = PopMemory(cpu) // Pop high byte (SP increments again)
	return uint16(hi)<<8 | uint16(lo)
}