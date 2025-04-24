package mapper

import (
	"log"
)

// UNROM represents the UNROM mapper (Mapper 2).
type UNROM struct {
	cart MapperAccessor

	prgBankCount16k      uint32
	prgBankMask          uint32
	selectedPrgBankOffset uint32
	lastPrgBankOffset    uint32

	isChrRAM bool

	prgSize uint32
	chrSize uint32
	hasSRAM bool
}

var _ Mapper = (*UNROM)(nil)

// Initialize sets up the UNROM mapper state.
func (m *UNROM) Initialize(cart MapperAccessor) {
	m.cart = cart
	header := cart.GetHeader()

	m.prgSize = cart.GetPRGSize()
	m.chrSize = cart.GetCHRSize()
	m.hasSRAM = cart.HasSRAM()
	m.isChrRAM = (m.chrSize == 0)

	if m.prgSize == 0 || m.prgSize%PRG_BANK_SIZE != 0 {
		log.Printf("UNROM Warning: Invalid PRG ROM size %d bytes. Must be a non-zero multiple of %dKB.", m.prgSize, PRG_BANK_SIZE/1024)
		m.prgBankCount16k = m.prgSize / PRG_BANK_SIZE
		if m.prgBankCount16k == 0 {
			m.prgBankCount16k = 1
		}
	} else {
		m.prgBankCount16k = m.prgSize / PRG_BANK_SIZE
	}

	if m.prgBankCount16k > 0 {
		m.prgBankMask = m.prgBankCount16k - 1
		if !isPowerOfTwo(m.prgBankCount16k) {
			log.Printf("UNROM Warning: PRG bank count (%d) is not a power of two. Bank masking will wrap.", m.prgBankCount16k)
		}
	} else {
		m.prgBankMask = 0
	}

	if m.prgBankCount16k > 0 {
		m.lastPrgBankOffset = (m.prgBankCount16k - 1) * PRG_BANK_SIZE
	} else {
		m.lastPrgBankOffset = 0
	}

	cart.SetMirroringMode(header.VerticalMirroring, header.HorizontalMirroring, header.FourScreenVRAM, 0)

	log.Printf("UNROM Initializing: PRG: %dKB (%d banks, mask %X), CHR: %dKB (RAM: %v), SRAM: %v, Mirroring: %s",
		m.prgSize/1024, m.prgBankCount16k, m.prgBankMask,
		m.chrSize/1024, m.isChrRAM, m.hasSRAM,
		getMirroringModeString(header.VerticalMirroring, header.HorizontalMirroring, header.FourScreenVRAM))
}

// Reset handles mapper reset.
func (m *UNROM) Reset() {
	m.selectedPrgBankOffset = 0 * PRG_BANK_SIZE

	if m.prgSize >= PRG_BANK_SIZE {
		m.cart.CopyPRGData(0, m.selectedPrgBankOffset, PRG_BANK_SIZE)
	} else {
		log.Println("UNROM Reset Warning: PRG ROM too small for even one bank.")
	}

	if m.prgSize >= PRG_BANK_SIZE {
		if m.lastPrgBankOffset < m.prgSize {
			m.cart.CopyPRGData(PRG_BANK_SIZE, m.lastPrgBankOffset, PRG_BANK_SIZE)
		} else {
			log.Printf("UNROM Reset Error: Calculated last bank offset %X is out of bounds for PRG size %X. Mapping bank 0 instead.", m.lastPrgBankOffset, m.prgSize)
			m.cart.CopyPRGData(PRG_BANK_SIZE, 0, PRG_BANK_SIZE)
		}
	}

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

// MapCPU maps a CPU address to a PRG ROM/RAM offset.
func (m *UNROM) MapCPU(addr uint16) (isROM bool, mappedAddr uint16) {
	switch {
	case addr >= 0xC000:
		if m.prgSize == 0 {
			return false, 0xFFFF
		}
		mappedAddr = (addr & 0x3FFF) | 0x4000
		isROM = true
		return

	case addr >= 0x8000:
		if m.prgSize == 0 {
			return false, 0xFFFF
		}
		mappedAddr = addr & 0x3FFF
		isROM = true
		return

	case addr >= 0x6000:
		if m.hasSRAM {
			sramSize := uint16(m.cart.GetPRGRAMSize())
			offset := addr - 0x6000
			if offset < sramSize {
				mappedAddr = offset
				isROM = false
				return
			}
		}
		return false, 0xFFFF

	default:
		return false, 0xFFFF
	}
}

// MapPPU maps a PPU address to a CHR ROM/RAM offset.
func (m *UNROM) MapPPU(addr uint16) uint16 {
	if addr < 0x2000 {
		if m.isChrRAM || m.chrSize > 0 {
			return addr & 0x1FFF
		} else {
			return 0xFFFF
		}
	}
	return 0xFFFF
}

// Write handles CPU writes, primarily for PRG bank switching.
func (m *UNROM) Write(addr uint16, value byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		if m.hasSRAM {
			sramSize := uint16(m.cart.GetPRGRAMSize())
			offset := addr - 0x6000
			if offset < sramSize {
				m.cart.WriteSRAM(offset, value)
			}
		}
		return
	}

	if addr >= 0x8000 {
		selectedBank := uint32(value) & m.prgBankMask
		newOffset := selectedBank * PRG_BANK_SIZE

		if newOffset < m.prgSize {
			if newOffset != m.selectedPrgBankOffset {
				m.selectedPrgBankOffset = newOffset
				m.cart.CopyPRGData(0, m.selectedPrgBankOffset, PRG_BANK_SIZE)
			}
		} else {
			log.Printf("UNROM Write Warning: Addr=%04X Val=%02X -> Selected bank %d (masked value) results in invalid offset %X for PRG size %X. Write ignored.", addr, value, selectedBank, newOffset, m.prgSize)
		}
	}
}

// IRQState returns false as UNROM does not generate IRQs.
func (m *UNROM) IRQState() bool {
	return false
}

// ClockIRQCounter does nothing for UNROM.
func (m *UNROM) ClockIRQCounter() {
}

// Helper for logging mirroring state.
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
	return "Single Screen (Fixed Wiring)"
}