// File: ./mapper/mmc1.go
package mapper

import (
	"fmt"
	"log"
	"sync"
)

// MMC1 specific constants
const (
	// Control register bits (Reg 0: $8000-$9FFF)
	MMC1_CTRL_MIRROR_MASK    = 0x03
	MMC1_CTRL_MIRROR_SINGLE_L = 0x00
	MMC1_CTRL_MIRROR_SINGLE_H = 0x01
	MMC1_CTRL_MIRROR_VERT     = 0x02
	MMC1_CTRL_MIRROR_HORZ     = 0x03
	MMC1_CTRL_PRG_MODE_MASK  = 0x0C
	MMC1_CTRL_PRG_MODE_32K   = 0x00
	MMC1_CTRL_PRG_MODE_FIX_L = 0x08
	MMC1_CTRL_PRG_MODE_FIX_H = 0x0C
	MMC1_CTRL_CHR_MODE_MASK  = 0x10
	MMC1_CTRL_CHR_MODE_8K    = 0x00
	MMC1_CTRL_CHR_MODE_4K    = 0x10

	// Other registers
	MMC1_PRG_BANK_MASK   = 0x0F
	MMC1_PRG_RAM_DISABLE = 0x10

	// Internal constants
	MMC1_SHIFT_RESET     = 0x10
	MMC1_WRITE_COUNT_MAX = 5
)

// MMC1State holds the internal state of the MMC1 mapper
type MMC1State struct {
	// Internal Shift Register state
	shiftRegister byte
	writeCount    byte

	// Registers
	control  byte
	chrBank0 byte
	chrBank1 byte
	prgBank  byte

	// Derived state
	prgRAMEnabled     bool
	prgBankOffset16k0 uint32
	prgBankOffset16k1 uint32
	chrBankOffset4k0  uint32
	chrBankOffset4k1  uint32

	// Cartridge Info Cache
	prgSize        uint32
	chrSize        uint32
	prgRAMSize     uint32
	numPrgBanks16k uint32
	numChrBanks4k  uint32
	hasSRAM        bool
	hasChrRAM      bool
	isSUROMFamily  bool
	variant        string
}

// MMC1 represents the MMC1 mapper (Mapper 1).
type MMC1 struct {
	state MMC1State
	cart  MapperAccessor
	mutex sync.RWMutex
}

// Compile-time check to ensure MMC1 implements the Mapper interface
var _ Mapper = (*MMC1)(nil)

// Initialize initializes the MMC1 mapper state based on the cartridge.
func (m *MMC1) Initialize(cart MapperAccessor) {
	m.cart = cart
	header := cart.GetHeader()

	// Cache essential info
	m.state.prgSize = cart.GetPRGSize()
	m.state.chrSize = cart.GetCHRSize()
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
		effectiveChrSize := cart.GetCHRRAMSize()
		if effectiveChrSize == 0 {
			log.Printf("MMC1 Warning: CHR RAM reported size is 0, assuming %d bytes.", CHR_BANK_SIZE)
			effectiveChrSize = uint32(CHR_BANK_SIZE)
		}
		m.state.chrSize = effectiveChrSize
		m.state.numChrBanks4k = m.state.chrSize / 4096
	} else if m.state.chrSize > 0 {
		m.state.numChrBanks4k = m.state.chrSize / 4096
	}

	// Determine variant specifics
	m.state.variant = header.MMC1Variant
	m.state.isSUROMFamily = (m.state.variant == "SUROM" || m.state.variant == "SOROM" || m.state.variant == "SXROM")
	if m.state.isSUROMFamily && m.state.prgRAMSize < 32*1024 {
		log.Printf("MMC1 Warning: SUROM family variant '%s' detected, but PRG RAM size is %dKB (expected 32KB). Banking might be affected.", m.state.variant, m.state.prgRAMSize/1024)
	}

	log.Printf("MMC1 Initializing: PRG:%dKB(%d banks) CHR:%dKB(%d banks, RAM:%v) SRAM:%v(%dKB) Variant:%s SUROM+:%v",
		m.state.prgSize/1024, m.state.numPrgBanks16k,
		cart.GetCHRSize()/1024,
		m.state.numChrBanks4k, m.state.hasChrRAM,
		m.state.hasSRAM, m.state.prgRAMSize/1024,
		m.state.variant, m.state.isSUROMFamily)
}

// Reset resets the MMC1 mapper to its power-on/reset state.
func (m *MMC1) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.state.shiftRegister = MMC1_SHIFT_RESET
	m.state.writeCount = 0

	// Power-on state
	m.state.control = MMC1_CTRL_PRG_MODE_FIX_H
	m.state.chrBank0 = 0
	m.state.chrBank1 = 0
	m.state.prgBank = 0
	m.state.prgRAMEnabled = false

	// Apply initial state
	m.updateMirroring()
	if err := m.updateBankOffsets(); err != nil {
		log.Printf("MMC1 Reset Error: Failed initial bank offset calculation: %v", err)
		// Set safe defaults
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
	m.copyBanks()
	log.Println("MMC1 Reset complete. Initial PRG Mode: Fix Last Bank. PRG RAM: Disabled.")
}

// MapCPU maps a CPU address ($6000-$FFFF) to a PRG ROM/RAM offset.
func (m *MMC1) MapCPU(addr uint16) (isROM bool, mappedAddr uint16) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if addr >= 0x6000 && addr <= 0x7FFF {
		if m.state.hasSRAM && m.state.prgRAMEnabled {
			offset := addr - 0x6000

			var ramAddr uint16
			if m.state.isSUROMFamily {
				bankSelect := uint16((m.state.chrBank0 >> 2) & 0x03)
				ramAddr = bankSelect*uint16(SRAM_BANK_SIZE) + offset
			} else {
				ramAddr = offset
			}

			if uint32(ramAddr) < m.state.prgRAMSize {
				return false, ramAddr
			} else {
				return false, 0xFFFF
			}
		}
		return false, 0xFFFF
	}

	if addr >= 0x8000 {
		return true, addr & 0x7FFF
	}

	log.Printf("Warning: MMC1 MapCPU called with unexpected address %04X", addr)
	return false, 0xFFFF
}

// MapPPU maps a PPU address ($0000-$1FFF) to a CHR ROM/RAM offset.
func (m *MMC1) MapPPU(addr uint16) uint16 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if addr < 0x2000 {
		return addr & 0x1FFF
	}

	log.Printf("Warning: MMC1 MapPPU called with non-CHR address %04X", addr)
	return 0xFFFF
}

// Write handles CPU writes to mapper registers ($8000-$FFFF) or PRG RAM ($6000-$7FFF).
func (m *MMC1) Write(addr uint16, value byte) {
	// Handle PRG RAM Writes ($6000-$7FFF)
	if addr >= 0x6000 && addr <= 0x7FFF {
		m.mutex.RLock()
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
			}
		}
		return
	}

	// Handle Mapper Register Writes ($8000-$FFFF)
	if addr < 0x8000 {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check for reset bit
	if (value & 0x80) != 0 {
		m.state.shiftRegister = MMC1_SHIFT_RESET
		m.state.writeCount = 0
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
	writeBit := (value & 0x01) << 4
	m.state.shiftRegister = (m.state.shiftRegister >> 1) | writeBit
	m.state.writeCount++

	// If this is the 5th write, commit the value to the target register
	if m.state.writeCount == MMC1_WRITE_COUNT_MAX {
		targetData := m.state.shiftRegister & 0x1F
		registerAddr := addr & 0xE000

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
					needsBankUpdate = true
				}
			}
		case 0xE000: // PRG Bank / PRG RAM Enable ($E000-$FFFF)
			newEnableState := (targetData & MMC1_PRG_RAM_DISABLE) == 0
			if m.state.prgBank != targetData || m.state.prgRAMEnabled != newEnableState {
				m.state.prgBank = targetData
				m.state.prgRAMEnabled = newEnableState
				needsBankUpdate = true
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

// updateMirroring sets the mirroring mode in the cartridge.
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

// updateBankOffsets calculates source offsets. Returns error on invalid offset.
func (m *MMC1) updateBankOffsets() error {
	// PRG Bank Calculation
	prgMode := m.state.control & MMC1_CTRL_PRG_MODE_MASK
	prgBankSelect5Bit := uint32(m.state.prgBank & 0x1F)

	if m.state.isSUROMFamily {
		prgHighBit := uint32(m.state.chrBank0 & 0x10)
		prgBankSelect5Bit = (prgBankSelect5Bit & 0x0F) | (prgHighBit << 0)
	}

	prgBankMask := uint32(0)
	if m.state.numPrgBanks16k > 0 {
		prgBankMask = m.state.numPrgBanks16k - 1
	}

	switch prgMode {
	case MMC1_CTRL_PRG_MODE_32K:
		bank32kNum := (prgBankSelect5Bit >> 1) & (prgBankMask >> 1)
		baseOffset := bank32kNum * (2 * uint32(PRG_BANK_SIZE))
		m.state.prgBankOffset16k0 = baseOffset
		m.state.prgBankOffset16k1 = baseOffset + uint32(PRG_BANK_SIZE)
	case MMC1_CTRL_PRG_MODE_FIX_L:
		bank16kNum := prgBankSelect5Bit & prgBankMask
		m.state.prgBankOffset16k0 = 0
		m.state.prgBankOffset16k1 = bank16kNum * uint32(PRG_BANK_SIZE)
	case MMC1_CTRL_PRG_MODE_FIX_H:
		bank16kNum := prgBankSelect5Bit & prgBankMask
		lastBankIndex := uint32(0)
		if m.state.numPrgBanks16k > 0 { lastBankIndex = m.state.numPrgBanks16k - 1 }
		m.state.prgBankOffset16k0 = bank16kNum * uint32(PRG_BANK_SIZE)
		m.state.prgBankOffset16k1 = lastBankIndex * uint32(PRG_BANK_SIZE)
	}

	// CHR Bank Calculation
	if m.state.hasChrRAM {
		m.state.chrBankOffset4k0 = 0 * 4096
		m.state.chrBankOffset4k1 = 1 * 4096
	} else {
		chrMode4k := (m.state.control & MMC1_CTRL_CHR_MODE_MASK) == MMC1_CTRL_CHR_MODE_4K
		chrBank0Select := uint32(m.state.chrBank0 & 0x1F)
		chrBank1Select := uint32(m.state.chrBank1 & 0x1F)

		chrBankMask := uint32(0)
		if m.state.numChrBanks4k > 0 { chrBankMask = m.state.numChrBanks4k - 1 }

		if chrMode4k {
			bank4k0Num := chrBank0Select & chrBankMask
			bank4k1Num := chrBank1Select & chrBankMask
			m.state.chrBankOffset4k0 = bank4k0Num * 4096
			m.state.chrBankOffset4k1 = bank4k1Num * 4096
		} else {
			bank8kNum := (chrBank0Select >> 1) & (chrBankMask >> 1)
			baseOffset := bank8kNum * uint32(CHR_BANK_SIZE)
			m.state.chrBankOffset4k0 = baseOffset
			m.state.chrBankOffset4k1 = baseOffset + 4096
		}
	}

	// Validation
	prgSize := m.cart.GetPRGSize()
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
		chrSize := m.cart.GetCHRSize()
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

// copyBanks performs the actual copy to mapped windows.
func (m *MMC1) copyBanks() {
	// Copy PRG Banks
	if m.cart.GetPRGSize() > 0 {
		m.cart.CopyPRGData(0, m.state.prgBankOffset16k0, uint32(PRG_BANK_SIZE))
		m.cart.CopyPRGData(uint32(PRG_BANK_SIZE), m.state.prgBankOffset16k1, uint32(PRG_BANK_SIZE))
	}

	// Copy CHR Banks
	if !m.state.hasChrRAM && m.cart.GetCHRSize() > 0 {
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