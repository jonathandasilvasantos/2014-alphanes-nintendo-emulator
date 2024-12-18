package mapper

import (
	"log"
)

// MMC1 specific constants
const (
	MMC1_CHR_MODE_8K          = 0x10
	MMC1_CHR_MODE_4K          = 0x00
	MMC1_PRG_MODE_32K         = 0x00
	MMC1_PRG_MODE_FIRST_FIXED = 0x08
	MMC1_PRG_MODE_LAST_FIXED  = 0x0C
	MMC1_MIRROR_SINGLE_LOWER  = 0x00
	MMC1_MIRROR_SINGLE_UPPER  = 0x01
	MMC1_MIRROR_VERTICAL      = 0x02
	MMC1_MIRROR_HORIZONTAL    = 0x03
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
	CurrentCycleCount uint64 // Introduce a counter for the current cycle
	MMC1Revision      int    // Revision of the MMC1 chip (0: MMC1A, 1: MMC1B, 2: MMC1C)
	MirroringMode     byte   // Current mirroring mode
}

// MMC1 represents the MMC1 mapper (Mapper 1).
type MMC1 struct {
	state MMC1State
	cart  MapperAccessor // Use the MapperAccessor interface
}

// Initialize initializes the MMC1 mapper.
func (m *MMC1) Initialize(cart MapperAccessor) {
	m.cart = cart
	m.reset()
	log.Printf("MMC1 Mapper Initialized\n")
}

// reset resets the MMC1 mapper to its default state.
func (m *MMC1) reset() {
	m.state.ShiftRegister = 0x10
	m.state.WriteCount = 0
	m.state.Control = MMC1_PRG_MODE_LAST_FIXED
	m.state.CHRBank0 = 0
	m.state.CHRBank1 = 0
	m.state.PRGBank = 0
	m.state.PRGRAMEnable = false
	m.state.LastWriteCycle = 0
	m.state.CurrentCycleCount = 0
	m.state.MMC1Revision = 1          // Default to MMC1B behavior
	m.state.MirroringMode = 0xFF      // Indicate invalid mirroring mode initially
	m.updateControlRegister(0x0C)     // Set default mirroring and PRG mode
	m.UpdateBankMapping()             // Update banks after reset
	log.Printf("MMC1 Mapper Reset\n") // Log reset action
}

// MapCPU maps a CPU address to a PRG ROM/RAM address.
func (m *MMC1) MapCPU(addr uint16) (bool, uint16) {
	m.state.CurrentCycleCount++

	switch {
	case addr >= 0x6000 && addr <= 0x7FFF: // PRG RAM
		if m.state.PRGRAMEnable && m.cart.HasSRAM() {
			// Extended PRG RAM handling for SOROM, SUROM, SXROM
			if m.cart.GetHeader().MMC1Variant == "SOROM" || m.cart.GetHeader().MMC1Variant == "SUROM" || m.cart.GetHeader().MMC1Variant == "SXROM" {
				prgRAMBank := uint16(m.state.CHRBank0>>2) & 0x03
				return false, prgRAMBank*8192 + uint16(addr-0x6000)
			}
			return false, addr - 0x6000 // Regular 8KB PRG RAM
		}
		log.Printf("Attempted to access disabled or non-existent PRG RAM at %04X", addr)
		return false, 0 // PRG RAM disabled or not present

	case addr >= 0x8000: // PRG ROM
		return true, addr - 0x8000 // Mapping will be done by UpdateBankMapping

	default:
		return false, addr // Unmapped addresses
	}
}

// MapPPU maps a PPU address to a CHR ROM/RAM address.
func (m *MMC1) MapPPU(addr uint16) uint16 {
	if addr >= 0x3F00 && addr <= 0x3FFF {
		// Handle mirroring and special cases for palette RAM
		addr = 0x3F00 + (addr % 0x20)
		if addr == 0x3F10 || addr == 0x3F14 || addr == 0x3F18 || addr == 0x3F1C {
			addr -= 0x10
		}
		return addr
	}

	if addr < 0x2000 { // CHR ROM/RAM, handled by updateCHRBanks
		return addr
	}

	// Nametable mirroring
	addr = addr & 0x3FFF
	if addr >= 0x3000 && addr < 0x3F00 {
		addr -= 0x1000
	}

	if addr >= 0x2000 && addr < 0x3000 {
		addr = addr - 0x2000
		table := addr / 0x400
		offset := addr % 0x400

		mirroringMode := m.cart.GetMirroringMode()
		switch mirroringMode {
		case MMC1_MIRROR_HORIZONTAL:
			if table >= 2 {
				addr = 0x2400 + offset
			} else {
				addr = 0x2000 + offset
			}
		case MMC1_MIRROR_VERTICAL:
			if table == 1 || table == 3 {
				addr = 0x2400 + offset
			} else {
				addr = 0x2000 + offset
			}
		case MMC1_MIRROR_SINGLE_LOWER, MMC1_MIRROR_SINGLE_UPPER:
			addr = 0x2000 + uint16(mirroringMode&0x01)*0x400 + offset
		case 0x04: // Four-screen
			addr = 0x2000 + uint16(table)*0x400 + offset
		default:
			log.Printf("Warning: Invalid mirroring mode: %02X\n", mirroringMode)
			addr = 0x2000 + offset
		}
	}

	return addr
}

// Write handles writes to the MMC1 mapper's registers.
func (m *MMC1) Write(addr uint16, value byte) {
	// Log the write operation for debugging
	log.Printf("MMC1 Write: Addr=%04X, Value=%02X, Revision=%d, ShiftRegister=%02X, WriteCount=%d\n", addr, value, m.state.MMC1Revision, m.state.ShiftRegister, m.state.WriteCount)

	// Consecutive write-cycle detection (MMC1 specific behavior)
	if m.state.CurrentCycleCount-m.state.LastWriteCycle < 3 {
		log.Printf("MMC1 Consecutive write detected - ignoring write")
		m.state.LastWriteCycle = m.state.CurrentCycleCount
		return
	}
	m.state.LastWriteCycle = m.state.CurrentCycleCount

	if (value & 0x80) != 0 {
		// Reset shift register and set PRG bank mode to 3 (16KB, last bank fixed)
		log.Printf("MMC1 Shift Register Reset\n")
		m.state.ShiftRegister = 0x10
		m.state.WriteCount = 0
		m.state.Control |= MMC1_PRG_MODE_LAST_FIXED
		m.updateControlRegister(m.state.Control) // Apply changes to control register
		return
	}

	// Shift the new bit into the shift register
	m.state.ShiftRegister = ((m.state.ShiftRegister >> 1) | ((value & 0x01) << 4))
	m.state.WriteCount++

	if m.state.WriteCount == 5 {
		// On the fifth write, determine the target register and write to it
		registerData := m.state.ShiftRegister
		switch {
		case addr >= 0x8000 && addr <= 0x9FFF:
			m.updateControlRegister(registerData)
		case addr >= 0xA000 && addr <= 0xBFFF:
			m.state.CHRBank0 = registerData
			if m.cart.GetMapperNumber() == 105 {
				// NES-EVENT board PRG RAM handling
				m.state.PRGRAMEnable = (registerData & 0x10) == 0
			}
		case addr >= 0xC000 && addr <= 0xDFFF:
			m.state.CHRBank1 = registerData
		case addr >= 0xE000:
			m.state.PRGBank = registerData
			if m.state.MMC1Revision != 0 { // PRG RAM enable/disable (except on MMC1A)
				m.state.PRGRAMEnable = (registerData & 0x10) == 0
			}
		}

		// Reset shift register for the next write
		m.state.ShiftRegister = 0x10
		m.state.WriteCount = 0

		// Update bank mapping after writing to registers
		if err := m.UpdateBankMapping(); err != nil {
			log.Printf("Error updating bank mapping: %v\n", err)
		}
	}
}

// updateControlRegister updates the MMC1 control register and mirroring settings.
func (m *MMC1) updateControlRegister(value byte) {
	m.state.Control = value
	log.Printf("MMC1 Control Register updated: %02X", m.state.Control)

	// Update mirroring mode based on the new control value
	newMirroringMode := m.state.Control & 0x03
	if m.state.MirroringMode != newMirroringMode {
		m.state.MirroringMode = newMirroringMode
		log.Printf("MMC1 Mirroring Mode updated: %02X", m.state.MirroringMode)
	}

	m.cart.SetMirroringMode(
		(value&MMC1_MIRROR_VERTICAL) == MMC1_MIRROR_VERTICAL,
		(value&MMC1_MIRROR_HORIZONTAL) == MMC1_MIRROR_HORIZONTAL,
		false,
		(value & 0x03),
	)
}

// UpdateBankMapping updates the PRG and CHR ROM bank mapping based on the current MMC1 state.
func (m *MMC1) UpdateBankMapping() error {
	numPRGBanks := m.cart.GetPRGSize() / PRG_BANK_SIZE

	// Check for invalid PRG ROM size
	if m.cart.GetPRGSize() == 0 || numPRGBanks == 0 {
		return &MapperError{"UpdateBankMapping", "Invalid PRG ROM Size"}
	}

	if err := m.updatePRGBanks(numPRGBanks); err != nil {
		return err
	}

	if err := m.updateCHRBanks(); err != nil {
		return err
	}

	log.Printf("MMC1 Bank Mapping Updated: PRG Mode=%02X, CHR Mode=%02X, PRG Bank=%02X, CHR Bank 0=%02X, CHR Bank 1=%02X",
		m.state.Control&MMC1_PRG_MODE_LAST_FIXED, m.state.Control&MMC1_CHR_MODE_8K, m.state.PRGBank, m.state.CHRBank0, m.state.CHRBank1)

	return nil
}

// updatePRGBanks updates the PRG ROM bank mapping based on the current MMC1 state.
func (m *MMC1) updatePRGBanks(numPRGBanks uint32) error {
	prgMode := m.state.Control & MMC1_PRG_MODE_LAST_FIXED

	switch prgMode {
	case MMC1_PRG_MODE_32K: // 32KB switching mode
		// For 32KB switching mode, ignore the low bit of the bank number
		bankBase := uint32(m.state.PRGBank&0x0E) * PRG_BANK_SIZE
		if bankBase >= m.cart.GetPRGSize() {
			// Wrap around if the bank number is too large
			bankBase = (m.cart.GetPRGSize() - 2*PRG_BANK_SIZE) % (numPRGBanks * PRG_BANK_SIZE)
		}
		if bankBase+2*PRG_BANK_SIZE > m.cart.GetPRGSize() {
			return &MapperError{"updatePRGBanks", "PRG bank out of bounds in 32KB mode"}
		}
		// Copy two consecutive 16KB banks
		m.cart.CopyPRGData(0, bankBase, PRG_BANK_SIZE)
		m.cart.CopyPRGData(PRG_BANK_SIZE, bankBase+PRG_BANK_SIZE, PRG_BANK_SIZE)

	case MMC1_PRG_MODE_FIRST_FIXED: // First bank fixed, last bank switchable
		// First bank (0x8000-0xBFFF) is fixed to bank 0
		m.cart.CopyPRGData(0, 0, PRG_BANK_SIZE)

		// Last bank (0xC000-0xFFFF) is switchable
		bankNum := uint32(m.state.PRGBank&0x0F) * PRG_BANK_SIZE
		if bankNum >= m.cart.GetPRGSize() {
			bankNum = (m.cart.GetPRGSize() - PRG_BANK_SIZE) % (numPRGBanks * PRG_BANK_SIZE)
		}
		if bankNum+PRG_BANK_SIZE > m.cart.GetPRGSize() {
			return &MapperError{"updatePRGBanks", "PRG bank out of bounds in first-fixed mode"}
		}
		m.cart.CopyPRGData(PRG_BANK_SIZE, bankNum, PRG_BANK_SIZE)

	case MMC1_PRG_MODE_LAST_FIXED: // Switchable first bank, fixed last bank
		// In MMC1A, bit 3 selects 16KB bank at $8000 if '0', and controls PRG A17 if '1'.
		if m.state.MMC1Revision == 0 && (m.state.PRGBank&0x08) != 0 {
			// MMC1A with bit 3 set in PRG bank register
			bankNum := uint32(m.state.PRGBank&0x07) * PRG_BANK_SIZE // Ignore bit 3
			if bankNum >= m.cart.GetPRGSize() {
				bankNum = (m.cart.GetPRGSize() - PRG_BANK_SIZE) % (numPRGBanks * PRG_BANK_SIZE)
			}
			if bankNum+PRG_BANK_SIZE > m.cart.GetPRGSize() {
				return &MapperError{"updatePRGBanks", "PRG bank out of bounds in MMC1A fixed-last mode"}
			}
			m.cart.CopyPRGData(0, bankNum, PRG_BANK_SIZE)

			// For MMC1A, when bit 3 is set, the last bank is not fixed and is determined by bit 3
			lastBankNum := uint32(m.state.PRGBank&0x08) * PRG_BANK_SIZE
			if lastBankNum >= m.cart.GetPRGSize() {
				lastBankNum = (m.cart.GetPRGSize() - PRG_BANK_SIZE) % (numPRGBanks * PRG_BANK_SIZE)
			}
			m.cart.CopyPRGData(PRG_BANK_SIZE, lastBankNum, PRG_BANK_SIZE)
		} else {
			// MMC1B behavior or MMC1A with bit 3 clear
			// First bank (0x8000-0xBFFF) is switchable
			bankNum := uint32(m.state.PRGBank&0x0F) * PRG_BANK_SIZE
			if bankNum >= m.cart.GetPRGSize() {
				bankNum = (m.cart.GetPRGSize() - PRG_BANK_SIZE) % (numPRGBanks * PRG_BANK_SIZE)
			}
			if bankNum+PRG_BANK_SIZE > m.cart.GetPRGSize() {
				return &MapperError{"updatePRGBanks", "PRG bank out of bounds in fixed-last mode"}
			}
			m.cart.CopyPRGData(0, bankNum, PRG_BANK_SIZE)

			// Last bank (0xC000-0xFFFF) is fixed to the last bank
			lastBankStart := (numPRGBanks - 1) * PRG_BANK_SIZE
			m.cart.CopyPRGData(PRG_BANK_SIZE, lastBankStart, PRG_BANK_SIZE)
		}
	}

	return nil
}

// updateCHRBanks updates the CHR ROM bank mapping based on the current MMC1 state.
func (m *MMC1) updateCHRBanks() error {
	// Handle no CHR data case (use 8KB CHR RAM)
	if m.cart.GetCHRSize() == 0 {
		if m.cart.GetCHRRAMSize() != 8192 {
			m.cart.SetCHRRAMSize(8192) // Ensure 8KB CHR RAM is allocated
			log.Printf("Allocated 8KB CHR RAM")
		}
		return nil // Nothing to copy, just return
	}

	chrMode := m.state.Control & MMC1_CHR_MODE_8K
	numCHRBanks := uint32(m.cart.GetCHRSize()) / CHR_BANK_SIZE

	// Check for invalid CHR ROM size
	if numCHRBanks == 0 {
		return &MapperError{"updateCHRBanks", "Invalid CHR ROM size"}
	}

	if chrMode == MMC1_CHR_MODE_8K {
		// 8KB CHR ROM mode
		bankBase := uint32(m.state.CHRBank0&0xFE) * 4096
		if bankBase >= m.cart.GetCHRSize() {
			bankBase = (m.cart.GetCHRSize() - 8192) % (numCHRBanks * 8192)
		}
		if bankBase+8192 > m.cart.GetCHRSize() {
			return &MapperError{"updateCHRBanks", "CHR bank out of bounds in 8KB mode"}
		}
		m.cart.CopyCHRData(0, bankBase, 8192)
	} else {
		// 4KB CHR ROM mode
		bank0 := uint32(m.state.CHRBank0) * 4096
		bank1 := uint32(m.state.CHRBank1) * 4096

		if bank0 >= m.cart.GetCHRSize() {
			bank0 = (m.cart.GetCHRSize() - 4096) % (numCHRBanks * 4096)
		}
		if bank1 >= m.cart.GetCHRSize() {
			bank1 = (m.cart.GetCHRSize() - 4096) % (numCHRBanks * 4096)
		}
		if bank0+4096 > m.cart.GetCHRSize() {
			return &MapperError{"updateCHRBanks", "CHR bank 0 out of bounds in 4KB mode"}
		}
		if bank1+4096 > m.cart.GetCHRSize() {
			return &MapperError{"updateCHRBanks", "CHR bank 1 out of bounds in 4KB mode"}
		}

		m.cart.CopyCHRData(0, bank0, 4096)
		m.cart.CopyCHRData(4096, bank1, 4096)
	}

	return nil
}