// File: ./mapper/mmc1.go
package mapper

import (
	"fmt"
	"log"
	"sync" // Using RWMutex
)

// MMC1 specific constants
const (
	// Control register bits (Reg 0: $8000-$9FFF)
	MMC1_CTRL_MIRROR_MASK   = 0x03 // Bits 0,1: Mirroring Mode
	MMC1_CTRL_MIRROR_SINGLE_L = 0x00 //   00: One-screen, lower bank
	MMC1_CTRL_MIRROR_SINGLE_H = 0x01 //   01: One-screen, upper bank
	MMC1_CTRL_MIRROR_VERT     = 0x02 //   10: Vertical
	MMC1_CTRL_MIRROR_HORZ     = 0x03 //   11: Horizontal
	MMC1_CTRL_PRG_MODE_MASK = 0x0C // Bits 2,3: PRG ROM bank mode
	MMC1_CTRL_PRG_MODE_32K  = 0x00 //   00/01: switch 32KB at $8000, ignoring low bit of bank number
	MMC1_CTRL_PRG_MODE_FIX_L  = 0x08 //   10: fix first bank ($8000) switch second ($C000)
	MMC1_CTRL_PRG_MODE_FIX_H  = 0x0C //   11: switch first bank ($8000) fix last bank ($C000)
	MMC1_CTRL_CHR_MODE_MASK = 0x10 // Bit 4: CHR ROM bank mode
	MMC1_CTRL_CHR_MODE_8K   = 0x00 //   0: switch 8KB at $0000
	MMC1_CTRL_CHR_MODE_4K   = 0x10 //   1: switch two 4KB banks at $0000 and $1000

	// CHR Bank 0 Register (Reg 1: $A000-$BFFF)
	// CHR Bank 1 Register (Reg 2: $C000-$DFFF)
	// PRG Bank Register (Reg 3: $E000-$FFFF)
	MMC1_PRG_BANK_MASK   = 0x0F // Bits 0-3: PRG ROM bank number (or RAM bank select)
	MMC1_PRG_RAM_DISABLE = 0x10 // Bit 4: PRG RAM Enable (0=Enabled, 1=Disabled) - Note: Active LOW logic

	// Internal constants
	MMC1_SHIFT_RESET     = 0x10 // Initial value for shift register (indicates empty, ready for bit 0)
	MMC1_WRITE_COUNT_MAX = 5
)

// MMC1State holds the internal state of the MMC1 mapper
type MMC1State struct {
	// Internal Shift Register state
	shiftRegister byte // 5-bit internal register, accumulates writes LSB first
	writeCount    byte // Counts bits written to shiftRegister (0-4)

	// Registers mirrored from shiftRegister writes (5 bits each)
	control  byte // $8000-$9FFF: Mirroring, PRG/CHR modes
	chrBank0 byte // $A000-$BFFF: CHR bank 0 (4KB/8KB) / PRG RAM bank select (SUROM+)
	chrBank1 byte // $C000-$DFFF: CHR bank 1 (4KB)
	prgBank  byte // $E000-$FFFF: PRG bank / PRG RAM enable

	// Derived state calculated from registers and cartridge info
	prgRAMEnabled     bool   // Is PRG RAM ($6000-$7FFF) enabled?
	prgBankOffset16k0 uint32 // Base offset in OriginalPRG for CPU $8000-$BFFF
	prgBankOffset16k1 uint32 // Base offset in OriginalPRG for CPU $C000-$FFFF
	chrBankOffset4k0  uint32 // Base offset in OriginalCHR/CHR RAM for PPU $0000-$0FFF
	chrBankOffset4k1  uint32 // Base offset in OriginalCHR/CHR RAM for PPU $1000-$1FFF

	// Cartridge Info Cache (avoids repeated interface calls)
	prgSize        uint32 // Size of OriginalPRG
	chrSize        uint32 // Size of OriginalCHR (0 if RAM)
	prgRAMSize     uint32 // Size of SRAM buffer
	numPrgBanks16k uint32 // Total number of 16KB banks in OriginalPRG
	numChrBanks4k  uint32 // Total number of 4KB banks in OriginalCHR/RAM
	hasSRAM        bool
	hasChrRAM      bool
	isSUROMFamily  bool // Flag for SUROM/SOROM/SXROM variants needing special banking
	variant        string // Stored variant name for logging/debugging
}

// MMC1 represents the MMC1 mapper (Mapper 1).
type MMC1 struct {
	state MMC1State
	cart  MapperAccessor // Interface for accessing cartridge data
	mutex sync.RWMutex   // Use RWMutex for separate read/write locking
}

// Compile-time check to ensure MMC1 implements the Mapper interface
var _ Mapper = (*MMC1)(nil)

// Initialize initializes the MMC1 mapper state based on the cartridge.
func (m *MMC1) Initialize(cart MapperAccessor) {
	m.cart = cart
	header := cart.GetHeader()

	// Cache essential info
	m.state.prgSize = cart.GetPRGSize()
	m.state.chrSize = cart.GetCHRSize() // 0 if CHR RAM
	m.state.prgRAMSize = cart.GetPRGRAMSize()
	m.state.hasSRAM = cart.HasSRAM()
	m.state.hasChrRAM = (m.state.chrSize == 0)

	// Calculate bank counts
	m.state.numPrgBanks16k = 0
	if m.state.prgSize > 0 {
		m.state.numPrgBanks16k = m.state.prgSize / uint32(PRG_BANK_SIZE)
	}

	m.state.numChrBanks4k = 0
	if m.state.hasChrRAM {
		// Treat CHR RAM as having a fixed size (8KB) for banking calculations
		// Use the size reported by the interface, assume 8KB if 0
		effectiveChrSize := cart.GetCHRRAMSize()
		if effectiveChrSize == 0 {
			log.Printf("MMC1 Warning: CHR RAM reported size is 0, assuming %d bytes.", CHR_BANK_SIZE)
			effectiveChrSize = uint32(CHR_BANK_SIZE)
		}
		m.state.chrSize = effectiveChrSize // Use effective size for internal calcs
		m.state.numChrBanks4k = m.state.chrSize / 4096
	} else if m.state.chrSize > 0 {
		m.state.numChrBanks4k = m.state.chrSize / 4096
	}

	// Determine variant specifics
	m.state.variant = header.MMC1Variant // Get detected variant
	m.state.isSUROMFamily = (m.state.variant == "SUROM" || m.state.variant == "SOROM" || m.state.variant == "SXROM")
	if m.state.isSUROMFamily && m.state.prgRAMSize < 32*1024 {
		log.Printf("MMC1 Warning: SUROM family variant '%s' detected, but PRG RAM size is %dKB (expected 32KB). Banking might be affected.", m.state.variant, m.state.prgRAMSize/1024)
	}

	log.Printf("MMC1 Initializing: PRG:%dKB(%d banks) CHR:%dKB(%d banks, RAM:%v) SRAM:%v(%dKB) Variant:%s SUROM+:%v",
		m.state.prgSize/1024, m.state.numPrgBanks16k,
		cart.GetCHRSize()/1024, // Log original CHR size
		m.state.numChrBanks4k, m.state.hasChrRAM,
		m.state.hasSRAM, m.state.prgRAMSize/1024,
		m.state.variant, m.state.isSUROMFamily)

	// Note: Reset() will be called by the Cartridge loader after Initialize()
}

// Reset resets the MMC1 mapper to its power-on/reset state.
func (m *MMC1) Reset() {
	m.mutex.Lock() // Use exclusive lock for reset
	defer m.mutex.Unlock()

	m.state.shiftRegister = MMC1_SHIFT_RESET // $10 indicates empty, ready for bit 0
	m.state.writeCount = 0

	// Power-on state:
	m.state.control = MMC1_CTRL_PRG_MODE_FIX_H // PRG mode 3 (switch $8000, fix last @ $C000)
	m.state.chrBank0 = 0
	m.state.chrBank1 = 0
	m.state.prgBank = 0
	// PRG RAM is initially disabled by default on MMC1 (bit 4 of PRG reg defaults high/set -> disabled)
	m.state.prgRAMEnabled = false // (MMC1_PRG_RAM_DISABLE bit is conceptually '1')

	// Apply initial state
	m.updateMirroring() // Set initial mirroring based on control reg default
	if err := m.updateBankOffsets(); err != nil {
		log.Printf("MMC1 Reset Error: Failed initial bank offset calculation: %v", err)
		// Set safe defaults to prevent panic?
		m.state.prgBankOffset16k0 = 0
		if m.state.numPrgBanks16k > 0 {
			m.state.prgBankOffset16k1 = (m.state.numPrgBanks16k - 1) * uint32(PRG_BANK_SIZE)
		} else {
			m.state.prgBankOffset16k1 = 0
		}
		m.state.chrBankOffset4k0 = 0
		m.state.chrBankOffset4k1 = 0
		if m.state.numChrBanks4k > 1 {
			m.state.chrBankOffset4k1 = 4096
		}
	}
	m.copyBanks() // Perform the initial copy of PRG/CHR banks
	log.Println("MMC1 Reset complete. Initial PRG Mode: Fix Last Bank. PRG RAM: Disabled.")
}

// MapCPU maps a CPU address ($6000-$FFFF) to a PRG ROM/RAM offset.
func (m *MMC1) MapCPU(addr uint16) (isROM bool, mappedAddr uint16) {
	m.mutex.RLock() // Read lock sufficient for mapping check
	defer m.mutex.RUnlock()

	if addr >= 0x6000 && addr <= 0x7FFF {
		if m.state.hasSRAM && m.state.prgRAMEnabled {
			offset := addr - 0x6000 // Offset within the 8KB $6000-$7FFF window

			var ramAddr uint16
			if m.state.isSUROMFamily {
				bankSelect := uint16((m.state.chrBank0 >> 2) & 0x03) // Selects 8KB bank
				ramAddr = bankSelect*uint16(SRAM_BANK_SIZE) + offset
			} else {
				ramAddr = offset // Standard 8KB SRAM mapping (no banking)
			}

			if uint32(ramAddr) < m.state.prgRAMSize {
				return false, ramAddr // Return offset within physical SRAM
			} else {
				return false, 0xFFFF // Indicate unmapped RAM access (outside allocated size)
			}
		}
		return false, 0xFFFF // Indicate unmapped access (SRAM disabled or not present)
	}

	if addr >= 0x8000 {
		// PRG ROM access. Return the offset within the 32KB mapped PRG window ($8000-$FFFF).
		return true, addr & 0x7FFF
	}

	log.Printf("Warning: MMC1 MapCPU called with unexpected address %04X", addr)
	return false, 0xFFFF // Indicate error/unmapped
}

// MapPPU maps a PPU address ($0000-$1FFF) to a CHR ROM/RAM offset.
func (m *MMC1) MapPPU(addr uint16) uint16 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if addr < 0x2000 {
		// CHR ROM/RAM access. Return the offset within the 8KB mapped CHR window ($0000-$1FFF).
		return addr & 0x1FFF
	}

	log.Printf("Warning: MMC1 MapPPU called with non-CHR address %04X", addr)
	return 0xFFFF // Indicate invalid address for CHR mapping
}

// Write handles CPU writes to mapper registers ($8000-$FFFF) or PRG RAM ($6000-$7FFF).
func (m *MMC1) Write(addr uint16, value byte) {
	// --- Handle PRG RAM Writes ($6000-$7FFF) ---
	if addr >= 0x6000 && addr <= 0x7FFF {
		m.mutex.RLock() // Check enable state with RLock first
		isEnabled := m.state.hasSRAM && m.state.prgRAMEnabled
		isSUROM := m.state.isSUROMFamily
		chrBank0 := m.state.chrBank0
		prgRAMSize := m.state.prgRAMSize
		m.mutex.RUnlock()

		if isEnabled {
			offset := addr - 0x6000
			var ramAddr uint16
			if isSUROM {
				bankSelect := uint16((chrBank0 >> 2) & 0x03)
				ramAddr = bankSelect*uint16(SRAM_BANK_SIZE) + offset
			} else {
				ramAddr = offset
			}

			if uint32(ramAddr) < prgRAMSize {
				m.cart.WriteSRAM(ramAddr, value)
			} // else: Write out of bounds is ignored
		} // else: Write to disabled RAM is ignored
		return // Done with $6000-$7FFF range
	}

	// --- Handle Mapper Register Writes ($8000-$FFFF) ---
	if addr < 0x8000 {
		return // Ignore writes below mapper range
	}

	// Mapper register writes require exclusive lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check for reset bit (bit 7)
	if (value & 0x80) != 0 {
		m.state.shiftRegister = MMC1_SHIFT_RESET
		m.state.writeCount = 0
		// Reset also sets PRG mode to 3 (fix last bank) in the control register
		// *** FIX: Apply NOT to byte type to avoid overflow ***
		m.state.control = (m.state.control & ^byte(MMC1_CTRL_PRG_MODE_MASK)) | MMC1_CTRL_PRG_MODE_FIX_H
		log.Printf("MMC1 Write: Reset bit detected. CtrlReg PRG mode forced to Fix Last.")

		m.updateMirroring()
		if err := m.updateBankOffsets(); err != nil {
			log.Printf("MMC1 Write Error: Failed bank update after reset bit: %v", err)
		}
		m.copyBanks()
		return
	}

	// Load bit 0 of the value into the shift register (LSB first)
	writeBit := (value & 0x01) << 4 // Shift bit 0 to bit 4 position
	m.state.shiftRegister = (m.state.shiftRegister >> 1) | writeBit
	m.state.writeCount++

	// If this is the 5th write, commit the value to the target register
	if m.state.writeCount == MMC1_WRITE_COUNT_MAX {
		targetData := m.state.shiftRegister & 0x1F // Use lower 5 bits
		registerAddr := addr & 0xE000            // Determine register block

		needsBankUpdate := false
		needsMirrorUpdate := false

		switch registerAddr {
		case 0x8000: // Control Register ($8000-$9FFF)
			if m.state.control != targetData {
				m.state.control = targetData
				needsMirrorUpdate = true
				needsBankUpdate = true
			}
		case 0xA000: // CHR Bank 0 / PRG RAM Bank ($A000-$BFFF)
			if m.state.chrBank0 != targetData {
				m.state.chrBank0 = targetData
				needsBankUpdate = true
			}
		case 0xC000: // CHR Bank 1 ($C000-$DFFF)
			if m.state.chrBank1 != targetData {
				m.state.chrBank1 = targetData
				if (m.state.control & MMC1_CTRL_CHR_MODE_MASK) == MMC1_CTRL_CHR_MODE_4K {
					needsBankUpdate = true // Only relevant in 4KB CHR mode
				}
			}
		case 0xE000: // PRG Bank / PRG RAM Enable ($E000-$FFFF)
			// Check if PRG Bank value changed OR PRG RAM enable bit changed
			newEnableState := (targetData & MMC1_PRG_RAM_DISABLE) == 0
			if m.state.prgBank != targetData || m.state.prgRAMEnabled != newEnableState {
				m.state.prgBank = targetData
				m.state.prgRAMEnabled = newEnableState
				// log.Printf("  Target: PRG Bank = %02X, PRG RAM Enabled: %v", targetData, newEnableState)
				needsBankUpdate = true // PRG bank number potentially changed
			}
		}

		// Reset shift register for the next write sequence
		m.state.shiftRegister = MMC1_SHIFT_RESET
		m.state.writeCount = 0

		// Apply updates if needed
		if needsMirrorUpdate {
			m.updateMirroring()
		}
		if needsBankUpdate {
			if err := m.updateBankOffsets(); err != nil {
				log.Printf("MMC1 Write Error: Failed bank update after register write: %v", err)
			} else {
				m.copyBanks()
			}
		}
	}
}

// updateMirroring sets the mirroring mode in the cartridge. Called with lock held.
func (m *MMC1) updateMirroring() {
	if m.cart.HasFourScreenVRAM() {
		m.cart.SetMirroringMode(false, false, true, 0)
		return
	}

	mode := m.state.control & MMC1_CTRL_MIRROR_MASK
	var v, h, four bool
	var singleBank byte = 0

	switch mode {
	case MMC1_CTRL_MIRROR_SINGLE_L: singleBank = 0
	case MMC1_CTRL_MIRROR_SINGLE_H: singleBank = 1
	case MMC1_CTRL_MIRROR_VERT: v = true
	case MMC1_CTRL_MIRROR_HORZ: h = true
	}
	four = false

	m.cart.SetMirroringMode(v, h, four, singleBank)
}

// updateBankOffsets calculates source offsets. Called with lock held. Returns error on invalid offset.
func (m *MMC1) updateBankOffsets() error {
	// --- PRG Bank Calculation ---
	prgMode := m.state.control & MMC1_CTRL_PRG_MODE_MASK
	prgBankSelect5Bit := uint32(m.state.prgBank & 0x1F)

	if m.state.isSUROMFamily {
		prgHighBit := uint32(m.state.chrBank0 & 0x10) // Bit 4 of CHR Bank 0
		prgBankSelect5Bit = (prgBankSelect5Bit & 0x0F) | (prgHighBit << 0) // Overwrite bit 4
	}

	prgBankMask := uint32(0) // Mask for available 16k banks
	if m.state.numPrgBanks16k > 0 {
		prgBankMask = m.state.numPrgBanks16k - 1
	}

	switch prgMode {
	case MMC1_CTRL_PRG_MODE_32K: // 00 or 01: Switch 32KB banks
		bank32kNum := (prgBankSelect5Bit >> 1) & (prgBankMask >> 1)
		baseOffset := bank32kNum * (2 * uint32(PRG_BANK_SIZE))
		m.state.prgBankOffset16k0 = baseOffset
		m.state.prgBankOffset16k1 = baseOffset + uint32(PRG_BANK_SIZE)
	case MMC1_CTRL_PRG_MODE_FIX_L: // 10: Fix first bank ($8000), switch second ($C000)
		bank16kNum := prgBankSelect5Bit & prgBankMask
		m.state.prgBankOffset16k0 = 0
		m.state.prgBankOffset16k1 = bank16kNum * uint32(PRG_BANK_SIZE)
	case MMC1_CTRL_PRG_MODE_FIX_H: // 11: Switch first bank ($8000), fix last ($C000)
		bank16kNum := prgBankSelect5Bit & prgBankMask
		lastBankIndex := uint32(0)
		if m.state.numPrgBanks16k > 0 { lastBankIndex = m.state.numPrgBanks16k - 1 }
		m.state.prgBankOffset16k0 = bank16kNum * uint32(PRG_BANK_SIZE)
		m.state.prgBankOffset16k1 = lastBankIndex * uint32(PRG_BANK_SIZE)
	}

	// --- CHR Bank Calculation ---
	if m.state.hasChrRAM {
		m.state.chrBankOffset4k0 = 0 * 4096
		m.state.chrBankOffset4k1 = 1 * 4096 // Offsets within the 8KB CHR RAM buffer
	} else {
		chrMode4k := (m.state.control & MMC1_CTRL_CHR_MODE_MASK) == MMC1_CTRL_CHR_MODE_4K
		chrBank0Select := uint32(m.state.chrBank0 & 0x1F)
		chrBank1Select := uint32(m.state.chrBank1 & 0x1F)

		chrBankMask := uint32(0) // Mask for available 4k banks
		if m.state.numChrBanks4k > 0 { chrBankMask = m.state.numChrBanks4k - 1 }

		if chrMode4k { // 4KB CHR Mode
			bank4k0Num := chrBank0Select & chrBankMask
			bank4k1Num := chrBank1Select & chrBankMask
			m.state.chrBankOffset4k0 = bank4k0Num * 4096
			m.state.chrBankOffset4k1 = bank4k1Num * 4096
		} else { // 8KB CHR Mode
			bank8kNum := (chrBank0Select >> 1) & (chrBankMask >> 1)
			baseOffset := bank8kNum * uint32(CHR_BANK_SIZE)
			m.state.chrBankOffset4k0 = baseOffset
			m.state.chrBankOffset4k1 = baseOffset + 4096
		}
	}

	// --- Validation ---
	prgSize := m.cart.GetPRGSize() // Get actual size for validation
	if prgSize > 0 {
		if m.state.prgBankOffset16k0 >= prgSize || (m.state.prgBankOffset16k0+uint32(PRG_BANK_SIZE)) > prgSize {
			return fmt.Errorf("calculated PRG bank 0 offset invalid (Offset: %X, Size: %X)", m.state.prgBankOffset16k0, prgSize)
		}
		if m.state.prgBankOffset16k1 >= prgSize || (m.state.prgBankOffset16k1+uint32(PRG_BANK_SIZE)) > prgSize {
			return fmt.Errorf("calculated PRG bank 1 offset invalid (Offset: %X, Size: %X)", m.state.prgBankOffset16k1, prgSize)
		}
	} else if m.state.prgBankOffset16k0 != 0 || m.state.prgBankOffset16k1 != 0 {
        return fmt.Errorf("calculated PRG bank offset non-zero but PRG size is 0")
    }


	if !m.state.hasChrRAM {
		chrSize := m.cart.GetCHRSize() // Use original CHR size for validation
        if chrSize > 0 {
		    if m.state.chrBankOffset4k0 >= chrSize || (m.state.chrBankOffset4k0+4096) > chrSize {
			    return fmt.Errorf("calculated CHR bank 0 offset invalid (Offset: %X, Size: %X)", m.state.chrBankOffset4k0, chrSize)
		    }
		    if m.state.chrBankOffset4k1 >= chrSize || (m.state.chrBankOffset4k1+4096) > chrSize {
			    return fmt.Errorf("calculated CHR bank 1 offset invalid (Offset: %X, Size: %X)", m.state.chrBankOffset4k1, chrSize)
		    }
        } else if m.state.chrBankOffset4k0 != 0 || m.state.chrBankOffset4k1 != 0 {
             return fmt.Errorf("calculated CHR bank offset non-zero but CHR size is 0")
        }
	}

	return nil
}

// copyBanks performs the actual copy to mapped windows. Called with lock held.
func (m *MMC1) copyBanks() {
	// Copy PRG Banks
	if m.cart.GetPRGSize() > 0 { // Avoid copy if no PRG ROM
		m.cart.CopyPRGData(0, m.state.prgBankOffset16k0, uint32(PRG_BANK_SIZE))
		m.cart.CopyPRGData(uint32(PRG_BANK_SIZE), m.state.prgBankOffset16k1, uint32(PRG_BANK_SIZE))
	}

	// Copy CHR Banks (only if using CHR ROM)
	if !m.state.hasChrRAM && m.cart.GetCHRSize() > 0 { // Avoid copy if no CHR ROM
		m.cart.CopyCHRData(0, m.state.chrBankOffset4k0, 4096)
		m.cart.CopyCHRData(4096, m.state.chrBankOffset4k1, 4096)
	}
}

// IRQState returns false as MMC1 does not generate IRQs.
func (m *MMC1) IRQState() bool {
	return false
}

// ClockIRQCounter does nothing for MMC1.
func (m *MMC1) ClockIRQCounter() {
	// MMC1 has no IRQ counter
}