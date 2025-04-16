// File: ./mapper/nrom.go
package mapper

import "log"

// NROM represents the NROM mapper (Mapper 0).
type NROM struct {
	cart         MapperAccessor // Interface to access cartridge data
	prgBanks     byte           // Number of 16KB PRG banks (1 or 2)
	isChrRAM     bool           // Does the cartridge use CHR RAM?
}

// Initialize sets up the NROM mapper state.
func (m *NROM) Initialize(cart MapperAccessor) {
	m.cart = cart
	header := cart.GetHeader()
	m.prgBanks = header.ROM_SIZE // Size in 16KB units
	m.isChrRAM = (header.VROM_SIZE == 0)

	// NROM doesn't control mirroring, it's fixed by the header/cartridge wiring.
	// Set the initial mirroring state in the cartridge based on the header.
	cart.SetMirroringMode(header.VerticalMirroring, header.HorizontalMirroring, header.FourScreenVRAM, header.SingleScreenBank)
}

// Reset handles mapper reset. For NROM, this copies the initial PRG/CHR banks.
func (m *NROM) Reset() {
	// Copy initial PRG bank(s)
	if m.prgBanks == 1 {
		// 16KB PRG: Mirror first bank to $8000-$BFFF and $C000-$FFFF
		m.cart.CopyPRGData(0, 0, PRG_BANK_SIZE)             // Copy to $8000-$BFFF
		m.cart.CopyPRGData(PRG_BANK_SIZE, 0, PRG_BANK_SIZE) // Mirror to $C000-$FFFF
		log.Println("NROM Reset: Mirrored 16KB PRG ROM.")
	} else {
		// 32KB PRG: Copy first bank to $8000-$BFFF, second to $C000-$FFFF
		m.cart.CopyPRGData(0, 0, PRG_BANK_SIZE)                       // Copy bank 0 to $8000-$BFFF
		m.cart.CopyPRGData(PRG_BANK_SIZE, PRG_BANK_SIZE, PRG_BANK_SIZE) // Copy bank 1 to $C000-$FFFF
		log.Println("NROM Reset: Loaded 32KB PRG ROM.")
	}

	// Copy initial CHR data (if using CHR ROM)
	if !m.isChrRAM && m.cart.GetCHRSize() > 0 {
        // NROM always maps the first 8KB of CHR ROM/RAM
        // Ensure CHR ROM isn't larger than the 8KB window can hold (shouldn't happen for NROM)
        copySize := uint32(CHR_BANK_SIZE)
        if m.cart.GetCHRSize() < copySize {
            copySize = m.cart.GetCHRSize()
        }
		m.cart.CopyCHRData(0, 0, copySize)
		log.Println("NROM Reset: Loaded CHR ROM.")
	} else if m.isChrRAM {
        log.Println("NROM Reset: CHR RAM active.")
		// CHR RAM is just used directly, no initial copy needed (unless initializing pattern)
    }
}

// MapCPU maps a CPU address ($6000-$FFFF) to a PRG ROM/RAM offset.
// NROM only maps PRG ROM at $8000-$FFFF. $6000-$7FFF is usually SRAM or open bus.
func (m *NROM) MapCPU(addr uint16) (isROM bool, mappedAddr uint16) {
	if addr >= 0x8000 {
		// Address is relative to the 32KB window starting at $8000.
		// The correct bank is already copied by Reset().
		return true, addr & 0x7FFF // Offset within the 32KB mapped PRG window
	}

	if addr >= 0x6000 && addr < 0x8000 {
		// Check if SRAM is present
		if m.cart.HasSRAM() {
			// Map to SRAM offset (basic, no banking for NROM)
			sramSize := uint16(m.cart.GetPRGRAMSize())
			offset := addr - 0x6000
			if offset < sramSize {
				return false, offset // Valid SRAM address
			}
		}
		// Otherwise, it's open bus or unmapped
		return false, 0xFFFF // Indicate unmapped access
	}

	// Addresses below $6000 are handled by CPU memory map (WRAM, PPU regs etc)
	return false, 0xFFFF // Indicate unmapped access by mapper
}

// MapPPU maps a PPU address ($0000-$1FFF) to a CHR ROM/RAM offset.
// NROM simply maps the address range directly to the first 8KB of CHR.
func (m *NROM) MapPPU(addr uint16) uint16 {
	if addr < 0x2000 {
		// NROM uses a fixed 8KB CHR bank (already loaded in Reset or accessed directly if RAM).
		// Just return the offset within this 8KB window.
		return addr & 0x1FFF
	}

	// Addresses $2000+ (nametables, palettes) are handled by PPU memory map, not the mapper directly.
	log.Printf("Warning: NROM MapPPU called with non-CHR address %04X", addr)
	return 0xFFFF // Indicate invalid address for CHR mapping
}

// Write handles CPU writes. NROM has no registers, but could potentially write to SRAM.
func (m *NROM) Write(addr uint16, value byte) {
	// Check for write to SRAM range $6000-$7FFF
	if addr >= 0x6000 && addr < 0x8000 {
		if m.cart.HasSRAM() {
			sramSize := uint16(m.cart.GetPRGRAMSize())
			offset := addr - 0x6000
			if offset < sramSize {
				m.cart.WriteSRAM(offset, value) // Use accessor to write
			}
			// Ignore write if out of SRAM bounds
		}
		// Ignore write if no SRAM
		return
	}

	// Writes to $8000-$FFFF are ignored by NROM hardware (writes to ROM).
	// log.Printf("NROM Write Ignored: Addr=%04X Val=%02X", addr, value)
}