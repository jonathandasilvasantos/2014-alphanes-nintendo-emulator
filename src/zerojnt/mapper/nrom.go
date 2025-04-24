// File: ./mapper/nrom.go
package mapper

import "log"

// NROM implements Mapper 0
type NROM struct {
	cart         MapperAccessor // Access to cartridge data
	prgBanks     byte           // Number of 16KB PRG banks
	isChrRAM     bool           // Whether CHR RAM is used
}

// Initialize sets up the mapper
func (m *NROM) Initialize(cart MapperAccessor) {
	m.cart = cart
	header := cart.GetHeader()
	m.prgBanks = header.ROM_SIZE
	m.isChrRAM = (header.VROM_SIZE == 0)

	cart.SetMirroringMode(header.VerticalMirroring, header.HorizontalMirroring, header.FourScreenVRAM, header.SingleScreenBank)
}

// Reset handles mapper reset
func (m *NROM) Reset() {
	if m.prgBanks == 1 {
		// Mirror 16KB bank
		m.cart.CopyPRGData(0, 0, PRG_BANK_SIZE)
		m.cart.CopyPRGData(PRG_BANK_SIZE, 0, PRG_BANK_SIZE)
		log.Println("NROM Reset: Mirrored 16KB PRG ROM.")
	} else {
		// Load both 16KB banks
		m.cart.CopyPRGData(0, 0, PRG_BANK_SIZE)
		m.cart.CopyPRGData(PRG_BANK_SIZE, PRG_BANK_SIZE, PRG_BANK_SIZE)
		log.Println("NROM Reset: Loaded 32KB PRG ROM.")
	}

	if !m.isChrRAM && m.cart.GetCHRSize() > 0 {
		copySize := uint32(CHR_BANK_SIZE)
		if m.cart.GetCHRSize() < copySize {
			copySize = m.cart.GetCHRSize()
		}
		m.cart.CopyCHRData(0, 0, copySize)
		log.Println("NROM Reset: Loaded CHR ROM.")
	} else if m.isChrRAM {
		log.Println("NROM Reset: CHR RAM active.")
	}
}

// MapCPU maps CPU address to PRG ROM/RAM offset
func (m *NROM) MapCPU(addr uint16) (isROM bool, mappedAddr uint16) {
	if addr >= 0x8000 {
		return true, addr & 0x7FFF
	}

	if addr >= 0x6000 && addr < 0x8000 {
		if m.cart.HasSRAM() {
			sramSize := uint16(m.cart.GetPRGRAMSize())
			offset := addr - 0x6000
			if offset < sramSize {
				return false, offset
			}
		}
		return false, 0xFFFF
	}

	return false, 0xFFFF
}

// MapPPU maps PPU address to CHR ROM/RAM offset
func (m *NROM) MapPPU(addr uint16) uint16 {
	if addr < 0x2000 {
		return addr & 0x1FFF
	}

	log.Printf("Warning: NROM MapPPU called with non-CHR address %04X", addr)
	return 0xFFFF
}

// Write handles CPU writes
func (m *NROM) Write(addr uint16, value byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		if m.cart.HasSRAM() {
			sramSize := uint16(m.cart.GetPRGRAMSize())
			offset := addr - 0x6000
			if offset < sramSize {
				m.cart.WriteSRAM(offset, value)
			}
		}
		return
	}
}

// IRQState returns false as NROM doesn't generate IRQs
func (m *NROM) IRQState() bool {
	return false
}

// ClockIRQCounter does nothing for NROM
func (m *NROM) ClockIRQCounter() {
}