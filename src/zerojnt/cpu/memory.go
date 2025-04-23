// File: cpu/memory.go
package cpu

import (
	"log" // Added for potential logging in default cases
	"zerojnt/cartridge"
	// Assumes cartridge.Cartridge has fields like:
	// PRG []byte // Mapped PRG Window (e.g., 32KB)
	// SRAM []byte // Save RAM buffer
	// IO ioports.IOPorts // Embedded or pointer
	// Mapper mapper.Mapper // Pointer to the current mapper
	// ... and PPU/APU are accessible via cpu.ppu and cpu.APU
)

// RM (Read Memory) reads a byte from the CPU's address space.
// It handles CPU RAM, PPU registers, APU/IO registers, and Cartridge space (PRG ROM/RAM via mapper).
func RM(cpu *CPU, cart *cartridge.Cartridge, addr uint16) byte {
	switch {
	// $0000-$1FFF CPU internal RAM (mirrored every 2 KB)
	case addr < 0x2000:
		return cpu.IO.CPU_RAM[addr&0x07FF] // Use mask 0x7FF for 2KB mirroring

	// $2000-$3FFF PPU registers (mirrored every 8 bytes)
	case addr >= 0x2000 && addr < 0x4000:
		if cpu.ppu == nil {
			// log.Printf("Warning: RM PPU register %04X when PPU is nil", addr)
			return 0 // Or potentially cpu.IO.LastRegWrite for open bus? Defaulting to 0.
		}
		return cpu.ppu.ReadRegister(addr) // PPU ReadRegister should handle mirroring (addr & 0x2007)

	// $4000-$401F APU & I/O Registers
	case addr >= 0x4000 && addr <= 0x401F:
		switch addr {
		case 0x4015: // APU Status Register
			if cpu.APU == nil {
				// log.Printf("Warning: RM APU register %04X when APU is nil", addr)
				return 0
			}
			return cpu.APU.ReadStatus() // Assuming APU has a ReadStatus method

		case 0x4016: // Controller 1 Data Register
			if len(cpu.IO.Controllers) < 1 {
				return 0
			}
			pad := &cpu.IO.Controllers[0]
			var data byte
			if pad.Strobe {
				data = pad.CurrentButtons & 0x01
			} else {
				if pad.ShiftCounter < 8 {
					data = (pad.LatchedButtons >> pad.ShiftCounter) & 0x01
					pad.ShiftCounter++
				} else {
					data = 1
				}
			}
			// Returning only bit 0 for simplicity. Real hardware might return open bus bits.
			return data

		case 0x4017: // Controller 2 Data Register
			if len(cpu.IO.Controllers) < 2 {
				return 0
			}
			pad := &cpu.IO.Controllers[1]
			var data byte
			if pad.Strobe {
				data = pad.CurrentButtons & 0x01
			} else {
				if pad.ShiftCounter < 8 {
					data = (pad.LatchedButtons >> pad.ShiftCounter) & 0x01
					pad.ShiftCounter++
				} else {
					data = 1
				}
			}
			// Returning only bit 0.
			return data

		default:
			// log.Printf("Info: Read from write-only or unhandled APU/IO register %04X", addr)
			// Open bus behavior is complex; returning 0 is a common simplification.
			// You might want to return cpu.IO.LastRegWrite or similar for better accuracy later.
			return 0
		}

	// $4020-$5FFF Expansion ROM (Usually unused)
	case addr >= 0x4020 && addr < 0x6000:
		// log.Printf("Info: Read from Expansion ROM area %04X", addr)
		return 0 // Typically returns 0 or open bus

	// $6000-$7FFF Cartridge SRAM / PRG-RAM (if available)
	case addr >= 0x6000 && addr < 0x8000:
		if cart != nil && cart.Mapper != nil {
			// Ask the mapper how to interpret this address
			isROM, mappedAddr := cart.Mapper.MapCPU(addr) // <<< FIXED: Use MapCPU

			if mappedAddr == 0xFFFF { // Check if mapper considers this address unmapped
				// log.Printf("Warning: Read from unmapped SRAM/PRG-RAM space %04X", addr)
				return 0 // Or open bus behavior?
			}

			if isROM {
				// This is unusual for $6000-$7FFF, but handle if mapper says ROM.
				log.Printf("Warning: MapCPU returned isROM=true for address %04X in SRAM range", addr)
				if cart.PRG == nil {
					log.Printf("Error: Read from mapped PRG address %04X but cart.PRG is nil", mappedAddr)
					return 0
				}
				if int(mappedAddr) < len(cart.PRG) {
					return cart.PRG[mappedAddr] // <<< FIXED: Read from cart.PRG buffer
				}
				log.Printf("Warning: Read from mapped PRG ROM address %04X out of bounds (%d) for CPU address %04X", mappedAddr, len(cart.PRG), addr)
				return 0
			} else {
				// Mapper indicates RAM (SRAM)
				if cart.SRAM == nil {
					// log.Printf("Warning: Read from mapped SRAM address %04X but cart.SRAM is nil for CPU address %04X", mappedAddr, addr)
					return 0 // No SRAM buffer available
				}
				if int(mappedAddr) < len(cart.SRAM) {
					return cart.SRAM[mappedAddr] // <<< FIXED: Read from cart.SRAM buffer
				}
				log.Printf("Warning: Read from mapped SRAM address %04X out of bounds (%d) for CPU address %04X", mappedAddr, len(cart.SRAM), addr)
				return 0
			}
		}
		// log.Printf("Warning: Read from SRAM/PRG-RAM %04X with nil cart/mapper", addr)
		return 0

	// $8000-$FFFF Cartridge PRG-ROM
	case addr >= 0x8000:
		if cart != nil && cart.Mapper != nil {
			// Ask the mapper how to interpret this address
			isROM, mappedAddr := cart.Mapper.MapCPU(addr) // <<< FIXED: Use MapCPU

			if mappedAddr == 0xFFFF { // Check if mapper considers this address unmapped
				// log.Printf("Warning: Read from unmapped PRG ROM space %04X", addr)
				return 0 // Or open bus behavior?
			}

			if isROM {
				// Mapper indicates ROM (PRG)
				if cart.PRG == nil {
					log.Printf("Error: Read from mapped PRG address %04X but cart.PRG is nil", mappedAddr)
					return 0
				}
				if int(mappedAddr) < len(cart.PRG) {
					return cart.PRG[mappedAddr] // <<< FIXED: Read from cart.PRG buffer
				}
				log.Printf("Warning: Read from mapped PRG ROM address %04X out of bounds (%d) for CPU address %04X", mappedAddr, len(cart.PRG), addr)
				return 0
			} else {
				// This is unusual for $8000+, but handle if mapper says RAM (e.g., mapper uses this range for RAM).
				log.Printf("Warning: MapCPU returned isROM=false for address %04X in PRG ROM range, mapping to SRAM", addr)
				if cart.SRAM == nil {
					// log.Printf("Warning: Read from mapped SRAM address %04X but cart.SRAM is nil for CPU address %04X", mappedAddr, addr)
					return 0
				}
				if int(mappedAddr) < len(cart.SRAM) {
					return cart.SRAM[mappedAddr] // <<< FIXED: Read from cart.SRAM buffer
				}
				log.Printf("Warning: Read from mapped SRAM address %04X out of bounds (%d) for CPU address %04X", mappedAddr, len(cart.SRAM), addr)
				return 0
			}
		}
		// log.Printf("Warning: Read from PRG-ROM %04X with nil cart/mapper", addr)
		return 0

	// Should not be reached if all ranges are covered
	default:
		log.Printf("Error: Unhandled CPU read from address %04X", addr)
		return 0
	}
}

// WM (Write Memory) writes a byte to the CPU's address space.
// Handles CPU RAM, PPU registers, APU/IO registers, OAM DMA, and Cartridge space (Mapper/SRAM).
func WM(cpu *CPU, cart *cartridge.Cartridge, addr uint16, value byte) {
	switch {
	case addr < 0x2000: // CPU RAM ($0000-$07FF) mirrored up to $1FFF
		cpu.IO.CPU_RAM[addr&0x07FF] = value // Write to RAM mirror
		return

	case addr >= 0x2000 && addr < 0x4000: // PPU Registers ($2000-$2007) mirrored every 8 bytes
		if cpu.ppu == nil {
			// log.Printf("Warning: WM PPU register %04X when PPU is nil", addr)
			return
		}
		cpu.ppu.WriteRegister(addr, value) // PPU WriteRegister handles mirroring and side effects
		return

	case addr == 0x4014: // OAM DMA Register Write ($4014)
		if cpu.ppu == nil {
			// log.Printf("Warning: WM OAMDMA register %04X when PPU is nil", addr)
			return
		}

		// --- OAM DMA Transfer ---
		dmaSourceAddrBase := uint16(value) << 8

		// DMA always writes to OAM starting from OAM[00].
		for i := 0; i < 256; i++ {
			sourceAddr := dmaSourceAddrBase + uint16(i)
			dataByte := RM(cpu, cart, sourceAddr) // Read from CPU space

			// Write directly to OAM index `i`.
			writeAddr := byte(i) // DMA destination is 0x00-0xFF

			// Ensure the index is within OAM bounds (should always be true for i<256)
			if int(writeAddr) < len(cpu.IO.OAM) {
				cpu.IO.OAM[writeAddr] = dataByte
			}
		}

		// Set DMA state and cycle penalty in IO struct
		cpu.IO.StartOAMDMA(value) // This should set CPU_CYC_INCREASE to 513/514
		return

	case addr == 0x4016: // Controller 1 Strobe Register ($4016)
		strobeBit := value & 1 // Extract strobe bit (bit 0)
		for i := 0; i < len(cpu.IO.Controllers); i++ { // Apply to all controllers managed by this register (usually just controller 1)
			controller := &cpu.IO.Controllers[i]
			if strobeBit == 1 {
				controller.Strobe = true
				controller.LatchedButtons = controller.CurrentButtons // Latch current state
			} else {
				if controller.Strobe { // Detect falling edge
					controller.LatchedButtons = controller.CurrentButtons // Final latch on falling edge
				}
				controller.Strobe = false
				controller.ShiftCounter = 0 // Reset counter when strobe goes low
			}
		}
		return

	// APU Registers ($4000-$4013, $4015, $4017)
	case (addr >= 0x4000 && addr <= 0x4013) || addr == 0x4015 || addr == 0x4017:
		if cpu.APU == nil {
			// log.Printf("Warning: WM APU register %04X when APU is nil", addr)
			return
		}
		cpu.APU.WriteRegister(addr, value) // Delegate write to APU
		return

	// $4020-$5FFF Expansion ROM (Typically ignore writes)
	case addr >= 0x4020 && addr < 0x6000:
		// log.Printf("Info: Write to Expansion ROM area %04X value %02X", addr, value)
		return

	// $6000-$7FFF Cartridge SRAM / PRG-RAM
	case addr >= 0x6000 && addr < 0x8000:
		if cart != nil && cart.Mapper != nil {
			// Mapper's Write method should handle checks for SRAM presence and write protection
			cart.Mapper.Write(addr, value)
		} else {
			// log.Printf("Warning: Write to SRAM/PRG-RAM %04X with nil cart/mapper", addr)
		}
		return

	// $8000-$FFFF Cartridge PRG-ROM (Used for Mapper Registers)
	case addr >= 0x8000:
		if cart != nil && cart.Mapper != nil {
			cart.Mapper.Write(addr, value) // Delegate write to mapper
		} else {
			// log.Printf("Warning: Write to PRG-ROM/Mapper %04X with nil cart/mapper", addr)
		}
		return

	default: // Unhandled address ranges
		log.Printf("Warning: Unhandled CPU write to address %04X value %02X", addr, value)
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
	cpu.IO.CPU_RAM[addr&0x07FF] = v // Direct write to RAM mirror
	cpu.SP--                        // Decrement stack pointer *after* write
}

// PopMemory pops a byte from the stack.
// SP points to the *last written* item. Increment SP *first* to point to the item to be read.
func PopMemory(cpu *CPU) byte {
	cpu.SP++ // Increment stack pointer *before* read
	addr := 0x0100 | uint16(cpu.SP)
	return cpu.IO.CPU_RAM[addr&0x07FF] // Direct read from RAM mirror
}

// PushWord pushes a 16-bit word onto the stack (high byte first, then low byte).
func PushWord(cpu *CPU, v uint16) {
	PushMemory(cpu, byte(v>>8))   // Push high byte (SP decrements)
	PushMemory(cpu, byte(v&0xFF)) // Push low byte (SP decrements again)
}

// PopWord pops a 16-bit word from the stack (low byte first, then high byte).
func PopWord(cpu *CPU) uint16 {
	lo := PopMemory(cpu) // Pop low byte (SP increments)
	hi := PopMemory(cpu) // Pop high byte (SP increments again)
	return (uint16(hi) << 8) | uint16(lo)
}