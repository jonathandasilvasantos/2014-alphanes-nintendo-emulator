package mapper

// NROM represents the NROM mapper (Mapper 0).
type NROM struct {
	cart    MapperAccessor // Use the MapperAccessor interface
	prgSize byte           // Number of PRG ROM banks (16KB or 32KB)
}

// Initialize initializes the NROM mapper.
func (m *NROM) Initialize(cart MapperAccessor) {
	m.cart = cart
	m.prgSize = cart.GetHeader().ROM_SIZE
}

// MapCPU maps a CPU address to a PRG ROM address.
func (m *NROM) MapCPU(addr uint16) (bool, uint16) {
	switch {
	case addr < 0x2000:
		// RAM access (mirroring typical)
		return false, addr % 0x0800
	case addr >= 0x2000 && addr <= 0x3FFF:
		// PPU register access (mirroring)
		return false, 0x2000 + (addr % 8)
	case addr >= 0x8000:
		// PRG ROM access
		if m.prgSize == 1 {
			// 16KB PRG ROM, mirror the single bank
			return true, (addr & 0x3FFF)
		}
		// 32KB PRG ROM
		return true, addr & 0x7FFF
	}
	return false, addr
}

// MapPPU maps a PPU address to a CHR ROM/RAM address.
func (m *NROM) MapPPU(addr uint16) uint16 {
	// Simple mapping for NROM (no bank switching for CHR)
	if addr >= 0x3F00 && addr <= 0x3FFF {
		// Palette RAM, handle mirroring and special cases
		addr = 0x3F00 + (addr % 0x20)
		if addr == 0x3F10 || addr == 0x3F14 || addr == 0x3F18 || addr == 0x3F1C {
			addr -= 0x10
		}
		return addr
	}

	addr = addr & 0x3FFF
	if addr >= 0x3000 && addr < 0x3F00 {
		addr -= 0x1000
	}

	if addr >= 0x2000 && addr < 0x3000 {
		addr = addr - 0x2000
		table := addr / 0x400
		offset := addr % 0x400

		// Handle mirroring based on cartridge configuration
		if m.cart.HasVerticalMirroring() {
			// Vertical mirroring: NT0/NT2 and NT1/NT3 are mirrors
			if table == 1 || table == 3 {
				addr = 0x2400 + offset
			} else {
				addr = 0x2000 + offset
			}
		} else if m.cart.HasHorizontalMirroring() {
			// Horizontal mirroring: NT0/NT1 and NT2/NT3 are mirrors
			if table >= 2 {
				addr = 0x2400 + offset
			} else {
				addr = 0x2000 + offset
			}
		} else if m.cart.HasFourScreenVRAM() {
			// Four-screen mirroring (uncommon, requires additional VRAM)
			addr = 0x2000 + table*0x400 + offset
		} else if m.cart.GetMirroringMode()&0x02 != 0 {
			// Single-screen mirroring
			addr = 0x2000 + uint16(m.cart.GetMirroringMode()&0x01)*0x400 + offset
		}
	}

	return addr
}

// Write handles writes to the NROM mapper.
// NROM doesn't have any registers, so this is a no-op.
func (m *NROM) Write(addr uint16, value byte) {
	// No registers to write to in NROM
}