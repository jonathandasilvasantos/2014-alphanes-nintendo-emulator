// File: cpu/memory.go
package cpu

import (
	"log"
	"zerojnt/cartridge"
)

// RM reads a byte from the CPU's 16-bit address space.
func RM(cpu *CPU, cart *cartridge.Cartridge, addr uint16) byte {
	switch {
	// CPU Internal RAM (2KB mirrored)
	case addr < 0x2000:
		return cpu.IO.CPU_RAM[addr&0x07FF]

	// PPU Registers (8 registers mirrored every 8 bytes)
	case addr >= 0x2000 && addr < 0x4000:
		if cpu.ppu == nil {
			return 0
		}
		// Mirror every 8 bytes within $2000-$3FFF range (0x2008 maps to 0x2000, etc.)
		ppuAddr := uint16(0x2000 | (addr & 0x0007))
		return cpu.ppu.ReadRegister(ppuAddr)

	// APU and I/O Registers
	case addr >= 0x4000 && addr <= 0x401F:
		switch addr {
		case 0x4015: // APU Status Register
			if cpu.APU == nil {
				return 0
			}
			return cpu.APU.ReadStatus()

		case 0x4016: // Controller 1 Data Register
			if len(cpu.IO.Controllers) < 1 {
				return 0
			}

			pad := &cpu.IO.Controllers[0]
			var dataToReturn byte

			if pad.Strobe {
				dataToReturn = pad.CurrentButtons & 0x01
			} else {
				if pad.ShiftCounter < 8 {
					dataToReturn = (pad.LatchedButtons >> pad.ShiftCounter) & 0x01
					pad.ShiftCounter++
				} else {
					dataToReturn = 1
				}
			}
			return dataToReturn

		case 0x4017: // Controller 2 Data Register
			if len(cpu.IO.Controllers) < 2 {
				return 0
			}

			pad := &cpu.IO.Controllers[1]
			var dataToReturn byte

			if pad.Strobe {
				dataToReturn = 0
			} else {
				if pad.ShiftCounter < 8 {
					dataToReturn = (pad.LatchedButtons >> pad.ShiftCounter) & 0x01
					pad.ShiftCounter++
				} else {
					dataToReturn = 1
				}
			}
			return dataToReturn

		default:
			return 0
		}

	// Expansion ROM
	case addr >= 0x4020 && addr < 0x6000:
		if cart != nil && cart.Mapper != nil {
			isROM, mappedAddr := cart.Mapper.MapCPU(addr)
			if mappedAddr != 0xFFFF {
				log.Printf("Info: Mapper handled read from Expansion ROM %04X (isROM: %v, mapped: %04X)", addr, isROM, mappedAddr)
			}
		}
		return 0

	// Cartridge SRAM / PRG-RAM
	case addr >= 0x6000 && addr < 0x8000:
		if cart != nil && cart.Mapper != nil {
			isROM, mappedAddr := cart.Mapper.MapCPU(addr)

			if mappedAddr == 0xFFFF {
				return 0
			}

			if isROM {
				log.Printf("Warning: MapCPU returned isROM=true for address %04X in SRAM range", addr)
				if cart.PRG == nil || int(mappedAddr) >= len(cart.PRG) {
					log.Printf("Error: Read from mapped PRG address %04X out of bounds or PRG is nil", mappedAddr)
					return 0
				}
				return cart.PRG[mappedAddr]
			} else {
				if !cart.HasSRAM() {
					return 0
				}
				if cart.SRAM == nil || int(mappedAddr) >= len(cart.SRAM) {
					return 0
				}
				return cart.SRAM[mappedAddr]
			}
		}
		return 0

	// Cartridge PRG-ROM
	case addr >= 0x8000:
		if cart != nil && cart.Mapper != nil {
			isROM, mappedAddr := cart.Mapper.MapCPU(addr)

			if mappedAddr == 0xFFFF {
				return 0
			}

			if isROM {
				prgWindowSize := len(cart.PRG)
				if cart.PRG == nil || int(mappedAddr) >= prgWindowSize {
					log.Printf("Error: Read from mapped PRG address %04X (original %04X) out of bounds (size %d) or PRG is nil", mappedAddr, addr, prgWindowSize)
					return 0
				}
				return cart.PRG[mappedAddr]
			} else {
				log.Printf("Warning: MapCPU returned isROM=false for address %04X in PRG ROM range, mapping to SRAM", addr)
				if !cart.HasSRAM() {
					return 0
				}
				if cart.SRAM == nil || int(mappedAddr) >= len(cart.SRAM) {
					return 0
				}
				return cart.SRAM[mappedAddr]
			}
		}
		return 0

	default:
		log.Printf("Error: Unhandled CPU read from address %04X", addr)
		return 0
	}
}

// WM writes a byte to the CPU's 16-bit address space.
func WM(cpu *CPU, cart *cartridge.Cartridge, addr uint16, value byte) {
	switch {
	// CPU Internal RAM (2KB mirrored)
	case addr < 0x2000:
		cpu.IO.CPU_RAM[addr&0x07FF] = value
		return

	// PPU Registers (8 registers mirrored every 8 bytes)
	case addr >= 0x2000 && addr < 0x4000:
		if cpu.ppu == nil {
			return
		}
		// Mirror every 8 bytes within $2000-$3FFF range (0x2008 maps to 0x2000, etc.)
		ppuAddr := uint16(0x2000 | (addr & 0x0007))
		cpu.ppu.WriteRegister(ppuAddr, value)
		return

	// APU and I/O Registers
	case addr >= 0x4000 && addr <= 0x401F:
		switch addr {
		case 0x4014: // OAM DMA Register
			if cpu.ppu == nil {
				break
			}

			dmaSourceAddrBase := uint16(value) << 8
			oamDestAddrStart := uint16(cpu.IO.OAMADDR)

			for i := 0; i < 256; i++ {
				sourceAddr := dmaSourceAddrBase + uint16(i)
				dataByte := RM(cpu, cart, sourceAddr)

				destOAMIndex := byte(oamDestAddrStart + uint16(i))

				if int(destOAMIndex) < len(cpu.IO.OAM) {
					cpu.IO.OAM[destOAMIndex] = dataByte
				} else {
					log.Printf("Error: OAM DMA write destination index %d calculated unexpectedly large", destOAMIndex)
					break
				}
			}
			cpu.IO.StartOAMDMA(value)

			// cycle penalty according to current CPU cycle parity
			cyclePenalty := 513
			if cpu.cycleCount&1 == 0 { // even?
				cyclePenalty = 514
			}
			cpu.IO.CPU_CYC_INCREASE = uint16(cyclePenalty)
			break

		case 0x4016: // Controller Strobe Register
			strobeVal := value & 1
			for i := 0; i < len(cpu.IO.Controllers); i++ {
				controller := &cpu.IO.Controllers[i]
				isStrobingNow := (strobeVal == 1)
				wasStrobingBefore := controller.Strobe

				if isStrobingNow {
					controller.LatchedButtons = controller.CurrentButtons
				}

				controller.Strobe = isStrobingNow

				if wasStrobingBefore && !isStrobingNow {
					controller.ShiftCounter = 0
				}
			}
			break

		case 0x4017: // APU Frame Counter / P2 Strobe
			fallthrough
		case 0x4015: // APU Channel Control
			fallthrough
		case 0x4000, 0x4001, 0x4002, 0x4003, 0x4004, 0x4005, 0x4006, 0x4007,
			0x4008, 0x4009, 0x400A, 0x400B, 0x400C, 0x400D, 0x400E, 0x400F,
			0x4010, 0x4011, 0x4012, 0x4013:
			if cpu.APU != nil {
				cpu.APU.WriteRegister(addr, value)
			}
			break

		default:
			break
		}
		return

	// Expansion ROM
	case addr >= 0x4020 && addr < 0x6000:
		if cart != nil && cart.Mapper != nil {
			cart.Mapper.Write(addr, value)
		}
		return

	// Cartridge Space (SRAM / PRG-RAM / Mapper Registers)
	case addr >= 0x6000:
		if cart != nil && cart.Mapper != nil {
			cart.Mapper.Write(addr, value)
		}
		return

	default:
		log.Printf("Warning: Unhandled CPU write address %04X value %02X", addr, value)
		return
	}
}

// PushMemory pushes a byte onto the stack.
func PushMemory(cpu *CPU, v byte) {
	addr := 0x0100 | uint16(cpu.SP)
	cpu.IO.CPU_RAM[addr&0x07FF] = v
	cpu.SP--
}

// PopMemory pops a byte from the stack.
func PopMemory(cpu *CPU) byte {
	cpu.SP++
	addr := 0x0100 | uint16(cpu.SP)
	return cpu.IO.CPU_RAM[addr&0x07FF]
}

// PushWord pushes a 16-bit word onto the stack (high byte first, then low byte).
func PushWord(cpu *CPU, v uint16) {
	PushMemory(cpu, byte(v>>8))
	PushMemory(cpu, byte(v&0xFF))
}

// PopWord pops a 16-bit word from the stack (low byte first, then high byte).
func PopWord(cpu *CPU) uint16 {
	lo := PopMemory(cpu)
	hi := PopMemory(cpu)
	return (uint16(hi) << 8) | uint16(lo)
}