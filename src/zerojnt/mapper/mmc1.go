package mapper

import (
	"fmt"
)

// MMC1 specific constants
const (
	// Control register bits
	MMC1_CHR_MODE_8K          = 0x10
	MMC1_CHR_MODE_4K          = 0x00
	MMC1_PRG_MODE_32K         = 0x00
	MMC1_PRG_MODE_FIRST_FIXED = 0x08
	MMC1_PRG_MODE_LAST_FIXED  = 0x0C
	
	// Mirroring modes
	MMC1_MIRROR_SINGLE_LOWER  = 0x00
	MMC1_MIRROR_SINGLE_UPPER  = 0x01
	MMC1_MIRROR_VERTICAL      = 0x02
	MMC1_MIRROR_HORIZONTAL    = 0x03
	MMC1_MIRROR_FOUR_SCREEN   = 0x04
	
	// Internal constants
	MMC1_SHIFT_RESET          = 0x10
	MMC1_WRITE_COUNT_MAX      = 5
	MMC1_MIN_WRITE_CYCLES     = 3
)

// MMC1State holds the internal state of the MMC1 mapper
type MMC1State struct {
	ShiftRegister     byte   // Used for serial-to-parallel conversion of writes
	WriteCount        byte   // Number of writes to the shift register
	Control           byte   // Control register (mirroring, switching modes)
	CHRBank0          byte   // CHR ROM bank 0 selection
	CHRBank1          byte   // CHR ROM bank 1 selection (4KB mode) or lower 4 bits for (SOROM, SUROM, or SXROM)
	PRGBank           byte   // PRG ROM bank selection
	PRGRAMEnable      bool   // PRG RAM enable/disable flag
	LastWriteCycle    uint64 // Cycle count of the last write (for write timing)
	CurrentCycleCount uint64 // Counter for the current cycle
	MMC1Revision      int    // Revision of the MMC1 chip (0: MMC1A, 1: MMC1B, 2: MMC1C)
	MirroringMode     byte   // Current mirroring mode
}

// MMC1 represents the MMC1 mapper (Mapper 1).
type MMC1 struct {
	state MMC1State
	cart  MapperAccessor // Interface for accessing cartridge data
}

// Initialize initializes the MMC1 mapper.
func (m *MMC1) Initialize(cart MapperAccessor) {
	m.cart = cart
	m.reset()
}

// reset resets the MMC1 mapper to its default state.
func (m *MMC1) reset() {
	m.state = MMC1State{
		ShiftRegister:     MMC1_SHIFT_RESET,
		WriteCount:        0,
		Control:           MMC1_PRG_MODE_LAST_FIXED,
		CHRBank0:          0,
		CHRBank1:          0,
		PRGBank:           0,
		PRGRAMEnable:      false,
		LastWriteCycle:    0,
		CurrentCycleCount: 0,
		MMC1Revision:      1, // Default to MMC1B behavior
		MirroringMode:     0xFF, // Invalid mirroring mode initially
	}
	
	m.updateControlRegister(0x0C) // Set default mirroring and PRG mode
	m.UpdateBankMapping()         // Update banks after reset
}

// MapCPU maps a CPU address to a PRG ROM/RAM address.
func (m *MMC1) MapCPU(addr uint16) (bool, uint16) {
	m.state.CurrentCycleCount++

	// PRG RAM ($6000-$7FFF)
	if addr >= 0x6000 && addr <= 0x7FFF {
		if m.state.PRGRAMEnable && m.cart.HasSRAM() {
			// Extended PRG RAM handling for special board variants
			variant := m.cart.GetHeader().MMC1Variant
			if variant == "SOROM" || variant == "SUROM" || variant == "SXROM" {
				// These variants use CHRBank0 bits to select PRG RAM banks
				prgRAMBank := uint16(m.state.CHRBank0>>2) & 0x03
				return false, prgRAMBank*8192 + uint16(addr-0x6000)
			}
			
			// Regular 8KB PRG RAM
			return false, addr - 0x6000
		}
		
		return false, 0
	}
	
	// PRG ROM ($8000-$FFFF)
	if addr >= 0x8000 {
		// Return true to indicate ROM access
		// Actual address translation happens in UpdateBankMapping
		return true, addr - 0x8000
	}
	
	// All other addresses are unmapped
	return false, addr
}

// MapPPU maps a PPU address to a CHR ROM/RAM address.
func (m *MMC1) MapPPU(addr uint16) uint16 {
	// Palette RAM ($3F00-$3FFF)
	if addr >= 0x3F00 && addr <= 0x3FFF {
		// Palette mirroring: $3F00-$3F1F mirrors to $3F20-$3FFF
		addr = 0x3F00 + (addr % 0x20)
		
		// Addresses $3F10, $3F14, $3F18, $3F1C mirror to $3F00, $3F04, $3F08, $3F0C
		if addr == 0x3F10 || addr == 0x3F14 || addr == 0x3F18 || addr == 0x3F1C {
			addr -= 0x10
		}
		
		return addr
	}
	
	// CHR ROM/RAM ($0000-$1FFF)
	if addr < 0x2000 {
		// Address mapping is handled by updateCHRBanks
		return addr
	}
	
	// Ensure address is within 0-$3FFF range
	addr = addr & 0x3FFF
	
	// $3000-$3EFF mirrors $2000-$2EFF
	if addr >= 0x3000 && addr < 0x3F00 {
		addr -= 0x1000
	}
	
	// Nametable mirroring logic ($2000-$2FFF)
	if addr >= 0x2000 && addr < 0x3000 {
		addr = addr - 0x2000
		table := addr / 0x400  // Which nametable (0-3)
		offset := addr % 0x400 // Offset within table
		
		mirroringMode := m.cart.GetMirroringMode()
		
		switch mirroringMode {
		case MMC1_MIRROR_HORIZONTAL:
			// In horizontal mirroring, tables 0,1 and 2,3 are mirrors
			if table >= 2 {
				addr = 0x2400 + offset // Table 1
			} else {
				addr = 0x2000 + offset // Table 0
			}
			
		case MMC1_MIRROR_VERTICAL:
			// In vertical mirroring, tables 0,2 and 1,3 are mirrors
			if table == 1 || table == 3 {
				addr = 0x2400 + offset // Table 1
			} else {
				addr = 0x2000 + offset // Table 0
			}
			
		case MMC1_MIRROR_SINGLE_LOWER, MMC1_MIRROR_SINGLE_UPPER:
			// Single-screen mirroring uses either the lower or upper nametable
			screenOffset := uint16(mirroringMode & 0x01) * 0x400
			addr = 0x2000 + screenOffset + offset
			
		case MMC1_MIRROR_FOUR_SCREEN:
			// Four-screen mirroring - each nametable is distinct
			addr = 0x2000 + uint16(table)*0x400 + offset
			
		default:
			addr = 0x2000 + offset
		}
	}
	
	return addr
}

// Write handles writes to the MMC1 mapper's registers.
func (m *MMC1) Write(addr uint16, value byte) {
	// Check if the address is in the register range ($8000-$FFFF)
	if addr < 0x8000 {
		return
	}
	
	// Detect consecutive writes (MMC1 ignores writes that are too close together)
	if m.state.CurrentCycleCount - m.state.LastWriteCycle < MMC1_MIN_WRITE_CYCLES {
		m.state.LastWriteCycle = m.state.CurrentCycleCount
		return
	}
	m.state.LastWriteCycle = m.state.CurrentCycleCount
	
	// Reset signal: bit 7 set
	if (value & 0x80) != 0 {
		m.state.ShiftRegister = MMC1_SHIFT_RESET
		m.state.WriteCount = 0
		m.state.Control |= MMC1_PRG_MODE_LAST_FIXED
		m.updateControlRegister(m.state.Control)
		return
	}
	
	// Shift the new bit into the shift register
	m.state.ShiftRegister = ((m.state.ShiftRegister >> 1) | ((value & 0x01) << 4))
	m.state.WriteCount++
	
	// Check if we've received 5 bits
	if m.state.WriteCount == MMC1_WRITE_COUNT_MAX {
		registerData := m.state.ShiftRegister
		
		// Write to the appropriate register based on the address
		switch {
		case addr <= 0x9FFF: // $8000-$9FFF: Control register
			m.updateControlRegister(registerData)
			
		case addr <= 0xBFFF: // $A000-$BFFF: CHR bank 0
			m.state.CHRBank0 = registerData
			if m.cart.GetMapperNumber() == 105 {
				// NES-EVENT board PRG RAM handling
				m.state.PRGRAMEnable = (registerData & 0x10) == 0
			}
			
		case addr <= 0xDFFF: // $C000-$DFFF: CHR bank 1
			m.state.CHRBank1 = registerData
			
		default: // $E000-$FFFF: PRG bank
			m.state.PRGBank = registerData
			// PRG RAM enable/disable (except on MMC1A)
			if m.state.MMC1Revision != 0 {
				m.state.PRGRAMEnable = (registerData & 0x10) == 0
			}
		}
		
		// Reset shift register for the next write
		m.state.ShiftRegister = MMC1_SHIFT_RESET
		m.state.WriteCount = 0
		
		// Update bank mapping after writing to registers
		m.UpdateBankMapping()
	}
}

// updateControlRegister updates the MMC1 control register and mirroring settings.
func (m *MMC1) updateControlRegister(value byte) {
	m.state.Control = value
	
	// Update mirroring mode
	newMirroringMode := value & 0x03
	if m.state.MirroringMode != newMirroringMode {
		m.state.MirroringMode = newMirroringMode
	}
	
	// Update cartridge mirroring mode
	m.cart.SetMirroringMode(
		(value & MMC1_MIRROR_VERTICAL) == MMC1_MIRROR_VERTICAL,   // Vertical
		(value & MMC1_MIRROR_HORIZONTAL) == MMC1_MIRROR_HORIZONTAL, // Horizontal
		false, // Not four-screen
		(value & 0x03), // Raw mirroring value
	)
}

// UpdateBankMapping updates the PRG and CHR ROM bank mapping based on the current MMC1 state.
func (m *MMC1) UpdateBankMapping() error {
	numPRGBanks := m.cart.GetPRGSize() / PRG_BANK_SIZE
	
	// Check for invalid PRG ROM size
	if m.cart.GetPRGSize() == 0 || numPRGBanks == 0 {
		return fmt.Errorf("invalid PRG ROM size")
	}
	
	// Update PRG banks
	if err := m.updatePRGBanks(numPRGBanks); err != nil {
		return err
	}
	
	// Update CHR banks
	if err := m.updateCHRBanks(); err != nil {
		return err
	}
	
	return nil
}

// updatePRGBanks updates the PRG ROM bank mapping based on the current MMC1 state.
func (m *MMC1) updatePRGBanks(numPRGBanks uint32) error {
	prgMode := m.state.Control & 0x0C // Extract PRG ROM bank mode (bits 2-3)
	
	switch prgMode {
	case MMC1_PRG_MODE_32K:
		// 32KB switching mode (uses 32KB banks, ignoring low bit of bank number)
		bankNum := uint32(m.state.PRGBank & 0x0E) * PRG_BANK_SIZE
		
		// Handle bank number wrapping
		if bankNum >= m.cart.GetPRGSize() {
			bankNum = (bankNum % (numPRGBanks * PRG_BANK_SIZE))
		}
		
		// Safety check
		if bankNum + 2*PRG_BANK_SIZE > m.cart.GetPRGSize() {
			return fmt.Errorf("PRG bank out of bounds in 32KB mode: bank=%d, size=%d", 
				bankNum, m.cart.GetPRGSize())
		}
		
		// Copy two consecutive 16KB banks
		m.cart.CopyPRGData(0, bankNum, PRG_BANK_SIZE)
		m.cart.CopyPRGData(PRG_BANK_SIZE, bankNum + PRG_BANK_SIZE, PRG_BANK_SIZE)
		
	case MMC1_PRG_MODE_FIRST_FIXED:
		// First bank fixed, second bank switchable
		
		// First bank ($8000-$BFFF) is fixed to bank 0
		m.cart.CopyPRGData(0, 0, PRG_BANK_SIZE)
		
		// Second bank ($C000-$FFFF) is switchable
		bankNum := uint32(m.state.PRGBank & 0x0F) * PRG_BANK_SIZE
		
		// Handle bank number wrapping
		if bankNum >= m.cart.GetPRGSize() {
			bankNum = (bankNum % (numPRGBanks * PRG_BANK_SIZE))
		}
		
		// Safety check
		if bankNum + PRG_BANK_SIZE > m.cart.GetPRGSize() {
			return fmt.Errorf("PRG bank out of bounds in first-fixed mode: bank=%d, size=%d", 
				bankNum, m.cart.GetPRGSize())
		}
		
		m.cart.CopyPRGData(PRG_BANK_SIZE, bankNum, PRG_BANK_SIZE)
		
	case MMC1_PRG_MODE_LAST_FIXED:
		// First bank switchable, last bank fixed
		
		// Special handling for MMC1A revision (different bit 3 behavior)
		if m.state.MMC1Revision == 0 && (m.state.PRGBank & 0x08) != 0 {
			// MMC1A with bit 3 set - this bit controls PRG A17 line
			bankNum := uint32(m.state.PRGBank & 0x07) * PRG_BANK_SIZE
			
			// Handle wrapping
			if bankNum >= m.cart.GetPRGSize() {
				bankNum = (bankNum % (numPRGBanks * PRG_BANK_SIZE))
			}
			
			// Safety check
			if bankNum + PRG_BANK_SIZE > m.cart.GetPRGSize() {
				return fmt.Errorf("PRG bank out of bounds in MMC1A mode: bank=%d, size=%d", 
					bankNum, m.cart.GetPRGSize())
			}
			
			// Set first bank ($8000-$BFFF)
			m.cart.CopyPRGData(0, bankNum, PRG_BANK_SIZE)
			
			// Calculate second bank based on bit 3
			lastBankNum := uint32(m.state.PRGBank & 0x08) * PRG_BANK_SIZE
			if lastBankNum + PRG_BANK_SIZE > m.cart.GetPRGSize() {
				lastBankNum = 0
			}
			
			// Set second bank ($C000-$FFFF)
			m.cart.CopyPRGData(PRG_BANK_SIZE, lastBankNum, PRG_BANK_SIZE)
			
		} else {
			// MMC1B/C behavior or MMC1A with bit 3 clear
			
			// First bank ($8000-$BFFF) is switchable
			bankNum := uint32(m.state.PRGBank & 0x0F) * PRG_BANK_SIZE
			
			// Handle wrapping
			if bankNum >= m.cart.GetPRGSize() {
				bankNum = (bankNum % (numPRGBanks * PRG_BANK_SIZE))
			}
			
			// Safety check
			if bankNum + PRG_BANK_SIZE > m.cart.GetPRGSize() {
				return fmt.Errorf("PRG bank out of bounds in last-fixed mode: bank=%d, size=%d", 
					bankNum, m.cart.GetPRGSize())
			}
			
			// Set first bank
			m.cart.CopyPRGData(0, bankNum, PRG_BANK_SIZE)
			
			// Last bank ($C000-$FFFF) is fixed to the last 16KB bank
			lastBankAddr := (numPRGBanks - 1) * PRG_BANK_SIZE
			m.cart.CopyPRGData(PRG_BANK_SIZE, lastBankAddr, PRG_BANK_SIZE)
		}
		
	default:
		return fmt.Errorf("invalid PRG bank mode: %02X", prgMode)
	}
	
	return nil
}

// updateCHRBanks updates the CHR ROM bank mapping based on the current MMC1 state.
func (m *MMC1) updateCHRBanks() error {
	// If no CHR ROM, we're using CHR RAM
	if m.cart.GetCHRSize() == 0 {
		// Ensure 8KB CHR RAM is available
		if m.cart.GetCHRRAMSize() != 8192 {
			m.cart.SetCHRRAMSize(8192)
		}
		return nil
	}
	
	// Determine CHR bank mode (4KB or 8KB)
	chrMode := m.state.Control & MMC1_CHR_MODE_8K
	numCHRBanks := m.cart.GetCHRSize() / CHR_BANK_SIZE
	
	// Safety check for ROM size
	if numCHRBanks == 0 {
		return fmt.Errorf("invalid CHR ROM size")
	}
	
	if chrMode == MMC1_CHR_MODE_8K {
		// 8KB CHR ROM mode - use a single 8KB bank
		// Note: In 8KB mode, we ignore the low bit of CHRBank0
		bankBase := uint32(m.state.CHRBank0 & 0xFE) * 4096
		
		// Handle bank number wrapping
		if bankBase >= m.cart.GetCHRSize() {
			bankBase = (bankBase % m.cart.GetCHRSize())
		}
		
		// Safety check
		if bankBase + 8192 > m.cart.GetCHRSize() {
			return fmt.Errorf("CHR bank out of bounds in 8KB mode: bank=%d, size=%d", 
				bankBase, m.cart.GetCHRSize())
		}
		
		// Copy the entire 8KB bank
		m.cart.CopyCHRData(0, bankBase, 8192)
		
	} else {
		// 4KB CHR ROM mode - use two separate 4KB banks
		bank0 := uint32(m.state.CHRBank0) * 4096
		bank1 := uint32(m.state.CHRBank1) * 4096
		
		// Handle bank number wrapping
		if bank0 >= m.cart.GetCHRSize() {
			bank0 = (bank0 % m.cart.GetCHRSize())
		}
		if bank1 >= m.cart.GetCHRSize() {
			bank1 = (bank1 % m.cart.GetCHRSize())
		}
		
		// Safety checks
		if bank0 + 4096 > m.cart.GetCHRSize() {
			return fmt.Errorf("CHR bank 0 out of bounds in 4KB mode: bank=%d, size=%d", 
				bank0, m.cart.GetCHRSize())
		}
		if bank1 + 4096 > m.cart.GetCHRSize() {
			return fmt.Errorf("CHR bank 1 out of bounds in 4KB mode: bank=%d, size=%d", 
				bank1, m.cart.GetCHRSize())
		}
		
		// Copy the two 4KB banks
		m.cart.CopyCHRData(0, bank0, 4096)
		m.cart.CopyCHRData(4096, bank1, 4096)
	}
	
	return nil
}