// File: ./mapper/unrom.go
package mapper

import (
	"log"
	// No explicit locking needed for this simple mapper if accesses are sequential
)

// UNROM represents the UNROM mapper (Mapper 2).
// Features 16KB switchable PRG ROM at $8000 and 16KB fixed PRG ROM (last bank) at $C000.
// CHR is typically 8KB RAM or fixed ROM. Mirroring is fixed by the board.
type UNROM struct {
	cart MapperAccessor // Interface to access cartridge data

	// PRG Banking
	prgBankCount16k     uint32 // Total number of 16KB PRG banks in the ROM
	prgBankMask         uint32 // Mask to apply to the written value to select a bank
	selectedPrgBankOffset uint32 // Current offset for the switchable bank ($8000-$BFFF)
	lastPrgBankOffset   uint32 // Offset of the fixed last bank ($C000-$FFFF)

	// CHR Info
	isChrRAM bool // Does the cartridge use CHR RAM?

	// Cartridge Info Cache
	prgSize uint32 // Total size of PRG ROM in bytes
	chrSize uint32 // Total size of CHR ROM in bytes (0 if RAM)
	hasSRAM bool   // Does the cart have SRAM? (Uncommon for UNROM)
}

// Compile-time check to ensure UNROM implements the Mapper interface
var _ Mapper = (*UNROM)(nil)

// Initialize sets up the UNROM mapper state.
func (m *UNROM) Initialize(cart MapperAccessor) {
	m.cart = cart
	header := cart.GetHeader()

	m.prgSize = cart.GetPRGSize()
	m.chrSize = cart.GetCHRSize()
	m.hasSRAM = cart.HasSRAM()
	m.isChrRAM = (m.chrSize == 0)

	// Calculate PRG banking info
	if m.prgSize == 0 || m.prgSize%PRG_BANK_SIZE != 0 {
		log.Printf("UNROM Warning: Invalid PRG ROM size %d bytes. Must be a non-zero multiple of %dKB.", m.prgSize, PRG_BANK_SIZE/1024)
		// Attempt to proceed, might lead to issues if size is truly incompatible
		m.prgBankCount16k = m.prgSize / PRG_BANK_SIZE
		if m.prgBankCount16k == 0 {
			m.prgBankCount16k = 1 // Avoid division by zero / mask issues, assume at least 1 block conceptually
		}
	} else {
		m.prgBankCount16k = m.prgSize / PRG_BANK_SIZE
	}

	// Calculate the mask based on the number of banks.
	if m.prgBankCount16k > 0 {
		m.prgBankMask = m.prgBankCount16k - 1
		// Use the isPowerOfTwo function defined in mapper.go
		if !isPowerOfTwo(m.prgBankCount16k) {
			log.Printf("UNROM Warning: PRG bank count (%d) is not a power of two. Bank masking will wrap.", m.prgBankCount16k)
		}
	} else {
		m.prgBankMask = 0 // No banks, mask is 0
	}

	// Calculate offset for the fixed last bank
	if m.prgBankCount16k > 0 {
		m.lastPrgBankOffset = (m.prgBankCount16k - 1) * PRG_BANK_SIZE
	} else {
		m.lastPrgBankOffset = 0 // Should not happen with valid ROM
	}

	// Set initial mirroring based on header (UNROM doesn't change it)
	cart.SetMirroringMode(header.VerticalMirroring, header.HorizontalMirroring, header.FourScreenVRAM, 0) // Single screen bank irrelevant here

	log.Printf("UNROM Initializing: PRG: %dKB (%d banks, mask %X), CHR: %dKB (RAM: %v), SRAM: %v, Mirroring: %s",
		m.prgSize/1024, m.prgBankCount16k, m.prgBankMask,
		m.chrSize/1024, m.isChrRAM, m.hasSRAM,
		getMirroringModeString(header.VerticalMirroring, header.HorizontalMirroring, header.FourScreenVRAM)) // Use local helper
}

// Reset handles mapper reset. Copies initial PRG banks (0 and last) and CHR if ROM.
func (m *UNROM) Reset() {
	// --- PRG Reset ---
	// Set selected bank to the first bank (bank 0)
	m.selectedPrgBankOffset = 0 * PRG_BANK_SIZE

	// Copy bank 0 to the lower PRG window ($8000-$BFFF)
	if m.prgSize >= PRG_BANK_SIZE {
		m.cart.CopyPRGData(0, m.selectedPrgBankOffset, PRG_BANK_SIZE)
	} else {
		log.Println("UNROM Reset Warning: PRG ROM too small for even one bank.")
		// Fill with 0xFF? Or leave uninitialized?
	}

	// Copy the last bank to the upper PRG window ($C000-$FFFF)
	if m.prgSize >= PRG_BANK_SIZE { // Need at least one bank
		if m.lastPrgBankOffset < m.prgSize {
			m.cart.CopyPRGData(PRG_BANK_SIZE, m.lastPrgBankOffset, PRG_BANK_SIZE)
		} else {
			log.Printf("UNROM Reset Error: Calculated last bank offset %X is out of bounds for PRG size %X. Mapping bank 0 instead.", m.lastPrgBankOffset, m.prgSize)
			m.cart.CopyPRGData(PRG_BANK_SIZE, 0, PRG_BANK_SIZE) // Map bank 0 as fallback
		}
	} else {
		// If PRG is too small, the previous warning covers it. No copy needed here.
	}

	// --- CHR Reset ---
	// If using CHR ROM, copy the first (and only) 8KB bank.
	if !m.isChrRAM && m.chrSize > 0 {
		copySize := uint32(CHR_BANK_SIZE)
		if m.chrSize < copySize {
			log.Printf("UNROM Reset Warning: CHR ROM size %d is less than 8KB.", m.chrSize)
			copySize = m.chrSize
		}
		m.cart.CopyCHRData(0, 0, copySize)
		log.Printf("UNROM Reset: Loaded %dKB CHR ROM.", copySize/1024)
	} else if m.isChrRAM {
		log.Println("UNROM Reset: CHR RAM active.")
	} else {
		log.Println("UNROM Reset: No CHR ROM or RAM detected.")
	}

	log.Printf("UNROM Reset: Mapped PRG Bank 0 to $8000, Last PRG Bank (%d) to $C000.", m.lastPrgBankOffset/PRG_BANK_SIZE)
}

// MapCPU maps a CPU address ($6000-$FFFF) to a PRG ROM/RAM offset.
// Returns (isROM bool, mappedAddr uint16).
// mappedAddr is the offset within the corresponding memory space (PRG window or SRAM).
// Returns (false, 0xFFFF) for invalid/unmapped access.
func (m *UNROM) MapCPU(addr uint16) (isROM bool, mappedAddr uint16) {
	switch {
	case addr >= 0xC000:
		// Fixed upper bank ($C000-$FFFF -> offset $4000-$7FFF in 32KB mapped PRG window)
		if m.prgSize == 0 {
			return false, 0xFFFF // Invalid if no PRG ROM
		}
		mappedAddr = (addr & 0x3FFF) | 0x4000 // Offset within the 32KB PRG window
		isROM = true
		return

	case addr >= 0x8000:
		// Switched lower bank ($8000-$BFFF -> offset $0000-$3FFF in 32KB mapped PRG window)
		if m.prgSize == 0 {
			return false, 0xFFFF // Invalid if no PRG ROM
		}
		mappedAddr = addr & 0x3FFF // Offset within the 32KB PRG window
		isROM = true
		return

	case addr >= 0x6000:
		// Optional SRAM ($6000-$7FFF)
		if m.hasSRAM {
			sramSize := uint16(m.cart.GetPRGRAMSize())
			offset := addr - 0x6000
			if offset < sramSize {
				mappedAddr = offset // Offset within the SRAM buffer itself
				isROM = false
				return
			}
		}
		// Unmapped SRAM or address out of bounds
		return false, 0xFFFF

	default: // addr < 0x6000
		// Handled elsewhere (CPU RAM, PPU regs, etc.)
		return false, 0xFFFF
	}
}

// MapPPU maps a PPU address ($0000-$1FFF) to a CHR ROM/RAM offset.
// Returns the offset within the 8KB mapped CHR window.
// Returns 0xFFFF for invalid/unmapped access.
func (m *UNROM) MapPPU(addr uint16) uint16 {
	if addr < 0x2000 {
		// Fixed 8KB CHR window (either ROM or RAM)
		if m.isChrRAM || m.chrSize > 0 {
			return addr & 0x1FFF // Offset within the 8KB window
		} else {
			// No CHR available on this cartridge
			// log.Printf("Warning: UNROM MapPPU request for %04X, but no CHR detected.", addr)
			return 0xFFFF // Indicate no valid CHR mapping
		}
	}

	// Addresses $2000+ (nametables, palettes) are handled by PPU memory map, not the mapper.
	// log.Printf("Warning: UNROM MapPPU called with non-CHR address %04X", addr)
	return 0xFFFF // Indicate invalid address for mapper's CHR mapping
}

// Write handles CPU writes, primarily for PRG bank switching in the $8000-$FFFF range.
func (m *UNROM) Write(addr uint16, value byte) {
	// Check for write to SRAM range $6000-$7FFF
	if addr >= 0x6000 && addr < 0x8000 {
		if m.hasSRAM {
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

	// Handle PRG Bank Switching writes ($8000-$FFFF)
	if addr >= 0x8000 {
		// Apply mask to the value to get the desired bank number (relative to 16KB banks)
		// The mask handles wrapping for power-of-two sizes.
		selectedBank := uint32(value) & m.prgBankMask
		newOffset := selectedBank * PRG_BANK_SIZE

		// Ensure the calculated offset is valid within the actual PRG ROM size
		if newOffset < m.prgSize {
			// Only update and copy if the bank actually changed
			if newOffset != m.selectedPrgBankOffset {
				m.selectedPrgBankOffset = newOffset
				m.cart.CopyPRGData(0, m.selectedPrgBankOffset, PRG_BANK_SIZE) // Copy to lower 16KB window ($8000-$BFFF)
				// log.Printf("UNROM Write: Addr=%04X Val=%02X -> Switched PRG $8000-$BFFF to Bank %d (Offset %X)", addr, value, selectedBank, newOffset)
			}
		} else {
			log.Printf("UNROM Write Warning: Addr=%04X Val=%02X -> Selected bank %d (masked value) results in invalid offset %X for PRG size %X. Write ignored.", addr, value, selectedBank, newOffset, m.prgSize)
		}
	}
	// Writes below $6000 are ignored by the mapper.
}

// IRQState returns false as UNROM does not generate IRQs.
func (m *UNROM) IRQState() bool {
	return false
}

// ClockIRQCounter does nothing for UNROM.
func (m *UNROM) ClockIRQCounter() {
	// UNROM has no IRQ counter
}

// --- Local Helper Functions ---

// getMirroringModeString is a local helper for logging mirroring state.
func getMirroringModeString(v, h, four bool) string {
	if four {
		return "Four Screen"
	}
	if v {
		return "Vertical"
	}
	if h {
		return "Horizontal"
	}
	// UNROM typically relies on fixed wiring specified by header.
	return "Single Screen (Fixed Wiring)"
}

// NOTE: isPowerOfTwo function is removed from here.
// It should exist only once in mapper/mapper.go