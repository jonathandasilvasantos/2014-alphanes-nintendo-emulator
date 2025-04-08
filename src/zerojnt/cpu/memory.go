// File: cpu/memory.go
package cpu

import (
	"log"
	"zerojnt/cartridge"
	// "zerojnt/ioports" // Included via cpu.IO
	// "zerojnt/ppu"     // Included via cpu.ppu
	// "zerojnt/apu"     // Included via cpu.APU
)

// RM (Read Memory) reads a byte from the CPU's address space.
// It handles CPU RAM, PPU registers, APU/IO registers, and Cartridge space (PRG ROM/RAM via mapper).
func RM(cpu *CPU, cart *cartridge.Cartridge, addr uint16) byte {
	switch {
	case addr < 0x2000: // CPU RAM ($0000-$07FF) mirrored up to $1FFF
		return cpu.IO.CPU_RAM[addr&0x07FF] // Read from RAM mirror

	case addr >= 0x2000 && addr <= 0x3FFF: // PPU Registers ($2000-$2007) mirrored every 8 bytes
		if cpu.ppu == nil {
			log.Printf("Error: CPU trying to read PPU register %04X but CPU's PPU reference is nil", addr)
			return 0 // Consider returning cpu.IO.LastRegWrite for open bus?
		}
		return cpu.ppu.ReadRegister(addr & 0x2007) // Use mask for mirroring

	case addr >= 0x4000 && addr <= 0x401F: // APU and I/O Registers ($4000-$401F)
		switch addr {
		case 0x4015: // APU Status Register
			if cpu.APU == nil {
				log.Printf("Error: CPU trying to read APU status register %04X but APU is nil", addr)
				return 0
			}
			return cpu.APU.ReadStatus()

		case 0x4016: // Controller 1 Read (Joystick)
			// TODO: Implement Controller 1 Read Logic
			// log.Printf("TODO: CPU Read from Controller 1 ($4016)")
			// Often returns open bus (last byte written to CPU bus) or joystick data bits.
			// return cpu.IO.LastRegWrite // Placeholder for open bus?
			return 0 // Simpler placeholder for now

		case 0x4017: // Controller 2 Read (Joystick)
			// TODO: Implement Controller 2 Read Logic
			// log.Printf("TODO: CPU Read from Controller 2 ($4017)")
			// return cpu.IO.LastRegWrite // Placeholder for open bus?
			return 0 // Simpler placeholder for now

		default:
			// Other addresses in this range ($4000-$4014, $4018-$401F) are typically write-only or open bus on read.
			// log.Printf("Warning: CPU Read from potentially unreadable/write-only I/O address %04X", addr)
			// Return PPU's LastRegWrite? Can be inaccurate. Return 0 is simpler.
			// return cpu.IO.LastRegWrite // Placeholder for open bus?
			return 0
		}

	case addr >= 0x6000 && addr <= 0x7FFF: // Cartridge SRAM (Battery-backed potentially)
		if cart.HasSRAM() {
			isROM, mappedAddr := cart.Mapper.MapCPU(addr)
			if !isROM {
				// Ensure mappedAddr is within the actual SRAM bounds
				sramSize := uint16(cart.GetPRGRAMSize()) // Get actual SRAM size
				if mappedAddr < sramSize {
					return cart.SRAM[mappedAddr]
				} else {
					log.Printf("Error: Mapper mapped read address %04X to SRAM %04X which is out of bounds (SRAM size: %d)", addr, mappedAddr, sramSize)
					return 0
				}
			} else {
				log.Printf("Warning: Mapper mapped SRAM read address %04X to ROM address %04X", addr, mappedAddr)
				return 0 // Should ideally not happen for $6000-$7FFF if mapper is correct
			}
		} else {
			// log.Printf("Debug: CPU Read from disabled SRAM area %04X", addr)
			return 0 // Reads from disabled SRAM typically return open bus; 0 is a simplification
		}

	case addr >= 0x8000: // Cartridge PRG ROM ($8000-$FFFF)
		isROM, mappedAddr := cart.Mapper.MapCPU(addr)
		if isROM {
			// Use the PRG slice which might be dynamically updated by the mapper
			prgData := cart.PRG
			if mappedAddr < uint16(len(prgData)) {
				return prgData[mappedAddr]
			} else {
				originalSize := cart.GetPRGSize()
				log.Printf("Error: Mapper mapped read address %04X to PRG ROM %04X which is out of bounds (Mapped PRG size: %d, Original PRG size: %d)",
					addr, mappedAddr, len(prgData), originalSize)
				return 0
			}
		} else {
			// Mapper indicated this address ($8000+) is NOT ROM (e.g., mapper registers).
			// Read logic for high-mapped registers would go here if needed by a mapper.
			// log.Printf("Warning: CPU Read from address >= 0x8000 (%04X) mapped to non-ROM address %04X by mapper", addr, mappedAddr)
			return 0 // Placeholder for reads from high-mapped registers or open bus
		}

	default: // Unhandled ranges (e.g., $4020-$5FFF Expansion ROM)
		// log.Printf("Warning: CPU Read from unhandled/unmapped address range %04X", addr)
		return 0 // Placeholder for open bus
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
		return                              // Returns stallCycles (0)

	case addr >= 0x2000 && addr <= 0x3FFF: // PPU Registers ($2000-$2007) mirrored every 8 bytes
		if cpu.ppu == nil {
			log.Printf("Error: CPU trying to write PPU register %04X but CPU's PPU reference is nil", addr)
			return // Returns stallCycles (0)
		}
		cpu.ppu.WriteRegister(addr&0x2007, value) // Use mask for mirroring
		return                                     // Returns stallCycles (0)

	case addr == 0x4014: // OAM DMA Register Write ($4014)
		if cpu.ppu == nil {
			log.Printf("Error: CPU trying to write OAMDMA register %04X but PPU is nil", addr)
			return // Return 0 stall cycles on error
		}
		log.Printf("CPU Write to OAMDMA ($4014) with page value %02X", value)

		// --- Start Instant DMA Transfer Logic (Option 2) ---
		// The CPU still stalls, but we populate OAM immediately.

		dmaSourceAddrBase := uint16(value) << 8
		log.Printf("Performing instant OAM DMA transfer from CPU page %02X ($%04X)...", value, dmaSourceAddrBase)

		// Nesdev Wiki: DMA always starts writing to OAM at the current OAMADDR,
		// writing 256 bytes, wrapping around within OAM ($00-$FF).
		// OAMADDR is *not* reset to 0 by the $4014 write itself.
		currentOAMAddr := cpu.IO.OAMADDR // Get the starting OAM address from PPU/IO

		for i := 0; i < 256; i++ {
			// Read data byte from CPU memory space (using RM handles mirroring/mapping)
			sourceAddr := dmaSourceAddrBase + uint16(i)
			data := RM(cpu, cart, sourceAddr) // Use RM to read the source byte

			// Calculate the OAM destination address, wrapping around $00-$FF using byte addition
			writeAddr := currentOAMAddr + byte(i) // Byte addition handles wrap automatically

			// Write data byte to PPU OAM buffer in IOPorts
			// Bounds check is technically redundant due to byte wrap, but safe.
			if int(writeAddr) < len(cpu.IO.OAM) {
				cpu.IO.OAM[writeAddr] = data
			} else {
				// This should not happen with byte wrapping
				log.Printf("FATAL Error: OAM DMA write address calculation error. Index: %d, Addr: %02X", i, writeAddr)
				// Maybe panic here? This indicates a fundamental logic error.
			}
		}
		log.Printf("Instant OAM DMA transfer complete (OAM populated).")
		// --- End Instant DMA Transfer Logic ---

		// Now, initiate the *state* for the CPU stall (even though data is copied)
		// Call StartOAMDMA primarily to set the CPU_CYC_INCREASE value
		cpu.IO.StartOAMDMA(value) // Sets io.CPU_CYC_INCREASE and other flags (which might not be used now)
		stallCycles = int(cpu.IO.CPU_CYC_INCREASE) // Get the stall duration

		// Note: The actual transfer function io.DoOAMDMATransfer is NOT called
		// in the main loop when using this instant method. The CPU just waits.

		return // Return the calculated stallCycles

	case addr >= 0x4000 && addr <= 0x4013 || addr == 0x4015 || addr == 0x4017: // APU Registers ($4000-$4013, $4015, $4017)
		if cpu.APU == nil {
			log.Printf("Error: CPU trying to write APU register %04X but APU is nil", addr)
			return // Returns stallCycles (0)
		}
		cpu.APU.WriteRegister(addr, value)
		return // Returns stallCycles (0)

	case addr == 0x4016: // Controller Strobe/Write (Joystick)
		// TODO: Implement Controller Strobe/Write Logic
		// This usually involves telling the controller(s) to latch their state.
		// log.Printf("TODO: CPU Write to Controller Strobe ($4016) with value %02X", value)
		// cpu.IO.WriteControllerStrobe(value) // Example call
		return // Returns stallCycles (0)

	// case addr >= 0x4018 && addr <= 0x401F: // Typically unused APU/IO test regs, often disabled
	// 	log.Printf("Warning: CPU Write to unused/disabled I/O address %04X with value %02X", addr, value)
	// 	return // Returns stallCycles (0)

	case addr >= 0x6000 && addr <= 0x7FFF: // Cartridge SRAM / PRG RAM
		if cart.HasSRAM() {
			// Let the mapper handle potential mapping within SRAM or mapper registers
			cart.Mapper.Write(addr, value) // Mapper's write function should handle this range
		} else {
			// log.Printf("Warning: CPU Write to disabled SRAM area %04X with value %02X", addr, value)
			// Writing to disabled RAM usually has no effect. Mapper might still react.
			cart.Mapper.Write(addr, value) // Allow mapper to react even if SRAM not enabled
		}
		return // Returns stallCycles (0)

	case addr >= 0x8000: // Cartridge PRG ROM space ($8000-$FFFF) -> Mapper control
		// Writes to this range are usually directed to the mapper's registers
		// to control bank switching, mirroring, etc.
		cart.Mapper.Write(addr, value)
		return // Returns stallCycles (0)

	default: // Unhandled ranges (e.g., $4020-$5FFF Expansion ROM)
		// log.Printf("Warning: CPU Write to unhandled/unmapped address range %04X with value %02X", addr, value)
		return // Returns stallCycles (0)
	}
	// The return is handled within the switch cases implicitly via the named return value
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

