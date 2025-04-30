package ppu

import (
	"log"
)

// MirrorNametableAddress maps a VRAM address based on mirroring mode
func (ppu *PPU) MirrorNametableAddress(addr uint16) (effectiveAddr uint16, isInternalVRAM bool) {
	if addr < 0x2000 || addr >= 0x3F00 {
		log.Printf("Warning: MirrorNametableAddress called with non-nametable address %04X", addr)
		return addr, false
	}

	relativeAddr := addr & 0x0FFF
	vMirror, hMirror, fourScreen, singleScreen, singleScreenBank := ppu.Cart.GetCurrentMirroringType()

	if fourScreen {
		effectiveAddr = 0x2000 | relativeAddr
		isInternalVRAM = false
	} else if singleScreen {
		bankOffset := uint16(singleScreenBank) * 0x0400
		effectiveAddr = (relativeAddr & 0x03FF) | bankOffset
		isInternalVRAM = true
	} else if vMirror {
		effectiveAddr = relativeAddr & 0x07FF
		isInternalVRAM = true
	} else if hMirror {
		if relativeAddr < 0x0800 {
			effectiveAddr = relativeAddr & 0x03FF
		} else {
			effectiveAddr = 0x0400 | (relativeAddr & 0x03FF)
		}
		isInternalVRAM = true
	} else {
        log.Printf("Warning: Unknown mirroring state (v:%v h:%v 4s:%v ss:%v bank:%d), defaulting to HORIZONTAL", vMirror, hMirror, fourScreen, singleScreen, singleScreenBank)
        if relativeAddr < 0x0800 {
			effectiveAddr = relativeAddr & 0x03FF
		} else {
			effectiveAddr = 0x0400 | (relativeAddr & 0x03FF)
		}
		isInternalVRAM = true
    }

	if isInternalVRAM {
		effectiveAddr += 0x2000
	}

	return effectiveAddr, isInternalVRAM
}

// ReadPPUMemory reads a byte from PPU mapped memory
func (ppu *PPU) ReadPPUMemory(addr uint16) byte {
	addr &= 0x3FFF

	switch {
	case addr < 0x2000:
		physicalCHRAddr := ppu.Cart.Mapper.MapPPU(addr)
		chrData := ppu.Cart.CHR
		if physicalCHRAddr == 0xFFFF { return 0 }
		if int(physicalCHRAddr) < len(chrData) {
			return chrData[physicalCHRAddr]
		}
		return 0

	case addr >= 0x2000 && addr < 0x3F00:
		mappedAddr, isInternal := ppu.MirrorNametableAddress(addr)
		if isInternal {
			offset := mappedAddr - 0x2000
			if offset < uint16(len(ppu.IO.VRAM)) {
				return ppu.IO.VRAM[offset]
			}
			log.Printf("Warning: PPU Read internal VRAM mapped address %04X (offset %04X) out of bounds", mappedAddr, offset)
			return 0
		} else {
			physicalAddr := ppu.Cart.Mapper.MapPPU(mappedAddr)
			if physicalAddr == 0xFFFF { return 0 }
			chrData := ppu.Cart.CHR
			if int(physicalAddr) < len(chrData) {
				return chrData[physicalAddr]
			}
            log.Printf("Warning: PPU Read mapper-handled VRAM %04X mapped to %04X - Out of CHR bounds?", mappedAddr, physicalAddr)
			return 0
		}

	case addr >= 0x3F00:
		paletteAddr := (addr - 0x3F00) % 32
		if paletteAddr&0x03 == 0 {
            if paletteAddr >= 0x10 {
                paletteAddr -= 0x10
            }
        }
		if paletteAddr < uint16(len(ppu.IO.PaletteRAM)) {
			return ppu.IO.PaletteRAM[paletteAddr]
		}
		log.Printf("Warning: PPU Read Palette RAM address %04X (offset %02X) out of bounds", addr, paletteAddr)
		return 0

	default:
		log.Printf("Error: ReadPPUMemory reached default case for address %04X", addr)
		return 0
	}
}

// WritePPUMemory writes a byte to PPU mapped memory
func (ppu *PPU) WritePPUMemory(addr uint16, data byte) {
	addr &= 0x3FFF

	switch {
	case addr < 0x2000:
		if ppu.Cart.GetCHRSize() == 0 {
			physicalCHRAddr := ppu.Cart.Mapper.MapPPU(addr)
			if physicalCHRAddr == 0xFFFF { return }
			chrRAM := ppu.Cart.CHR
			if int(physicalCHRAddr) < len(chrRAM) {
				chrRAM[physicalCHRAddr] = data
			} else {
                log.Printf("Warning: PPU Write CHR RAM mapped address %04X out of bounds (%d)", physicalCHRAddr, len(chrRAM))
            }
		} else {
			ppu.Cart.Mapper.Write(addr, data)
		}

	case addr >= 0x2000 && addr < 0x3F00:
		mappedAddr, isInternal := ppu.MirrorNametableAddress(addr)
		if isInternal {
			offset := mappedAddr - 0x2000
			if offset < uint16(len(ppu.IO.VRAM)) {
				ppu.IO.VRAM[offset] = data
			} else {
                log.Printf("Warning: PPU Write internal VRAM mapped address %04X (offset %04X) out of bounds", mappedAddr, offset)
            }
		} else {
			ppu.Cart.Mapper.Write(mappedAddr, data)
		}

	case addr >= 0x3F00:
		paletteAddr := (addr - 0x3F00) % 32
        if paletteAddr&0x03 == 0 {
            if paletteAddr >= 0x10 {
                paletteAddr -= 0x10
            }
        }
		if paletteAddr < uint16(len(ppu.IO.PaletteRAM)) {
			ppu.IO.PaletteRAM[paletteAddr] = data
		} else {
            log.Printf("Warning: PPU Write Palette RAM address %04X (offset %02X) out of bounds", addr, paletteAddr)
        }
	}
}

// ReadRegister handles CPU reads from PPU registers
func (ppu *PPU) ReadRegister(addr uint16) byte {
	reg := addr & 0x07
	var data byte

	switch reg {
	case 0x02: // PPUSTATUS ($2002)
		// Re-introduce decay simulation ONLY for the lower 5 bits. Keep upper bits clean.
		status := (ppu.IO.PPUSTATUS.Get() & 0xE0) | (ppu.IO.LastRegWrite & 0x1F)

		// Read the status byte to be returned
		data = status

		// Clear address latch flag w
		ppu.w = 0

		// Clear VBlank flag *after* reading the status into `data`
		ppu.IO.PPUSTATUS.VBLANK = false
		// NMI line itself is not cleared here; it's cleared by the PPU timing (pre-render line)

	case 0x04: // OAMDATA ($2004)
		// Handle OAMDATA reads (ensure correct behavior during rendering if needed)
		// Simple version: read directly from OAMADDR
		data = ppu.IO.OAM[ppu.IO.OAMADDR]
		// Note: Reading OAMDATA during rendering has specific behaviors,
		// but this simple read is often sufficient.

	case 0x07: // PPUDATA ($2007)
		dataToReturn := ppu.IO.PPU_DATA_BUFFER // Return buffered value for reads < 0x3F00
		currentRead := ppu.ReadPPUMemory(ppu.v) // Perform the actual read

		if ppu.v >= 0x3F00 { // Reads from Palette RAM are not buffered
			dataToReturn = currentRead
			// Buffer is filled with the value from the *mirrored* VRAM address below the palette range ($3F00-$3FFF mirrors $2F00-$2FFF)
			// --- FIX: Correctly update buffer during palette reads ---
			ppu.IO.PPU_DATA_BUFFER = ppu.ReadPPUMemory(ppu.v & 0x2FFF) // Correctly read from VRAM mirrored below palette
		} else {
			// For reads below palette, update buffer with current read for the *next* read
			ppu.IO.PPU_DATA_BUFFER = currentRead
		}

		ppu.incrementVramAddress() // Increment VRAM address after read
		data = dataToReturn

	default:
		// Reading other registers (write-only or unused) typically returns LastRegWrite
		// or has open bus behavior. Returning LastRegWrite is a common simplification.
		data = ppu.IO.LastRegWrite
	}

	// Return the final data byte
	return data
}

// WriteRegister handles CPU writes to PPU registers
func (ppu *PPU) WriteRegister(addr uint16, data byte) {
	ppu.IO.LastRegWrite = data
	reg := addr & 0x07

	switch reg {
	case 0x00: // PPUCTRL ($2000)
		oldNMIEnable := ppu.IO.PPUCTRL.GEN_NMI
		wasInVBlank := ppu.IO.PPUSTATUS.VBLANK // Check VBlank state *before* updating PPUCTRL
		ppu.IO.PPUCTRL.Set(data)
		// Update temporary VRAM address t with nametable bits
		ppu.t = (ppu.t & 0xF3FF) | (uint16(data&0x03) << 10)

		// Check NMI logic based on VBlank state and NMI enable flag transition
		newNMIEnable := ppu.IO.PPUCTRL.GEN_NMI
		if wasInVBlank && newNMIEnable && !oldNMIEnable {
			ppu.IO.NMI = true // Assert NMI line if VBlank is active and NMI got enabled
		} else if !newNMIEnable { // If NMI is disabled by this write, ensure NMI line is low
			ppu.IO.ClearNMI()
		}

	case 0x01: // PPUMASK ($2001)
		ppu.IO.PPUMASK.Set(data)

	case 0x03: // OAMADDR ($2003)
		ppu.IO.OAMADDR = data

	case 0x04: // OAMDATA ($2004)
		ppu.IO.OAM[ppu.IO.OAMADDR] = data
		ppu.IO.OAMADDR++

	case 0x05: // PPUSCROLL ($2005)
		if ppu.w == 0 {
			ppu.t = (ppu.t & 0xFFE0) | (uint16(data) >> 3)
			ppu.x = data & 0x07
			ppu.w = 1
		} else {
			ppu.t = (ppu.t & 0x8C1F) | (uint16(data&0xF8) << 2)
			ppu.t = (ppu.t & 0x0FFF) | (uint16(data&0x07) << 12)
			ppu.w = 0
		}

	case 0x06: // PPUADDR ($2006)
		if ppu.w == 0 {
			ppu.t = (ppu.t & 0x00FF) | (uint16(data&0x3F) << 8)
			ppu.t &= 0x3FFF
			ppu.w = 1
		} else {
			ppu.t = (ppu.t & 0xFF00) | uint16(data)
			ppu.v = ppu.t
			ppu.w = 0
		}

	case 0x07: // PPUDATA ($2007)
		ppu.WritePPUMemory(ppu.v, data)
		ppu.incrementVramAddress()
	}
}