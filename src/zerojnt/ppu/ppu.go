// File: ./ppu/ppu.go
// Contains the core PPU emulation logic, memory access, register handling, and rendering pipeline.

/*
Copyright 2014, 2014 Jonathan da Silva Santos
Modifications Copyright 2023-2024 (by AI based on request and refinement)

This file is part of Alphanes.

    Alphanes is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    Alphanes is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with Alphanes. If not, see <http://www.gnu.org/licenses/>.
*/
package ppu

import (
	"fmt"
	"log" // Use log for errors/warnings
	"zerojnt/cartridge"
	"zerojnt/ioports"
	// "zerojnt/debug" // Keep commented if not actively used

	"github.com/veandco/go-sdl2/sdl" // Still needed for PPU struct definition
)

const (
	SCREEN_WIDTH  = 256
	SCREEN_HEIGHT = 240

	TOTAL_SCANLINES     = 262 // Includes VBlank and pre-render line (-1 to 260)
	CYCLES_PER_SCANLINE = 341

	// PPU Memory Map Addresses (Logical)
	PATTERN_TABLE_0 uint16 = 0x0000
	PATTERN_TABLE_1 uint16 = 0x1000
	NAMETABLE_0     uint16 = 0x2000
	NAMETABLE_1     uint16 = 0x2400
	NAMETABLE_2     uint16 = 0x2800
	NAMETABLE_3     uint16 = 0x2C00
	PALETTE_RAM     uint16 = 0x3F00
)

type PPU struct {
	// Framebuffer: Stores the pixel data for the current frame.
	// Updated by renderPixel during visible scanlines.
	SCREEN_DATA []uint32 // Use uint32 for ARGB8888 format directly

	CYC      int // Current cycle in scanline (0-340)
	SCANLINE int // Current scanline (-1 to 260)
	// D           *debug.PPUDebug // Keep for potential future use (assuming debug package exists)

	// SDL Resources - Defined here but initialized/used in ppu_display.go
	texture  *sdl.Texture  // SDL texture to display the framebuffer
	renderer *sdl.Renderer // SDL renderer
	window   *sdl.Window   // SDL window

	// Shared Resources
	IO   *ioports.IOPorts
	Cart *cartridge.Cartridge

	// Internal PPU Registers / State (Loopy Registers Style)
	v uint16 // Current VRAM address (15 bits) - Used for rendering fetching
	t uint16 // Temporary VRAM address (15 bits) - Typically holds address written by CPU via $2006/$2005
	x byte   // Fine X scroll (3 bits)
	w byte   // Write toggle (1 bit) - Toggles between high/low byte write for $2005/$2006

	// Background rendering pipeline state
	nt_byte             byte   // Nametable byte fetched
	at_byte             byte   // Attribute table byte fetched
	tile_data_lo        byte   // Low byte of tile pattern fetched
	tile_data_hi        byte   // High byte of tile pattern fetched
	bg_pattern_shift_lo uint16 // Background pattern shift registers (16-bit)
	bg_pattern_shift_hi uint16
	bg_attr_shift_lo    uint16 // Background attribute shift registers (16-bit, stores palette index bits 0,1)
	bg_attr_shift_hi    uint16

	// Sprite rendering state
	secondaryOAM [32]byte // Sprites for the *next* scanline (8 sprites * 4 bytes/sprite)
	spriteCount  int      // Number of sprites found for the *next* scanline (0-8)

	// Sprite shift registers and latches for the *current* scanline
	spritePatternsLo [8]byte // Pattern low bytes for up to 8 sprites
	spritePatternsHi [8]byte // Pattern high bytes for up to 8 sprites
	spriteCountersX  [8]byte // X position counters for sprites
	spriteLatches    [8]byte // Attribute latches for sprites
	spriteIsSprite0  [8]bool // Tracks if a secondary OAM slot holds sprite 0 (More accurately: tracks if sprite 0 *could be* in this slot)

	spriteZeroHitPossible bool // Sprite 0 is in secondary OAM for the next scanline
	spriteZeroBeingRendered bool // Sprite 0 is potentially outputting an opaque pixel on the current cycle

	// Frame state
	frameOdd bool // Tracks odd/even frames for cycle skip

	// Color Palette (loaded once)
	colors [64]uint32 // ARGB format
}

// loadPalette loads the standard NES palette into a [64]uint32 array (ARGB)
func loadPalette() [64]uint32 {
	// Standard NES Palette (e.g., NTSC Bisqwit)
	// Format: 0xAARRGGBB (Alpha is FF for opaque)
	palette := [64]uint32{
		0xFF7C7C7C, 0xFF0000FC, 0xFF0000BC, 0xFF4428BC, 0xFF940084, 0xFFA80020, 0xFFA81000, 0xFF881400,
		0xFF503000, 0xFF007800, 0xFF006800, 0xFF005800, 0xFF004058, 0xFF000000, 0xFF000000, 0xFF000000,
		0xFFBCBCBC, 0xFF0078F8, 0xFF0058F8, 0xFF6844FC, 0xFFD800CC, 0xFFE40058, 0xFFF83800, 0xFFE45C10,
		0xFFAC7C00, 0xFF00B800, 0xFF00A800, 0xFF00A844, 0xFF008888, 0xFF000000, 0xFF000000, 0xFF000000,
		0xFFF8F8F8, 0xFF3CBCFC, 0xFF6888FC, 0xFF9878F8, 0xFFF878F8, 0xFFF85898, 0xFFF87858, 0xFFFCA044,
		0xFFF8B800, 0xFFB8F818, 0xFF58D854, 0xFF58F898, 0xFF00E8D8, 0xFF787878, 0xFF000000, 0xFF000000,
		0xFFFCFCFC, 0xFFA4E4FC, 0xFFB8B8F8, 0xFFD8B8F8, 0xFFF8B8F8, 0xFFF8A4C0, 0xFFF0D0B0, 0xFFFCE0A8,
		0xFFF8D878, 0xFFD8F878, 0xFFB8F8B8, 0xFFB8F8D8, 0xFF00FCFC, 0xFFF8D8F8, 0xFF000000, 0xFF000000,
	}
	return palette
}

// MirrorNametableAddress maps a VRAM address (0x2000-0x2FFF range) based on mirroring mode.
// Returns the effective address *within the PPU's 2KB internal VRAM* (0x2000-0x27FF range logically)
// or the original address if FourScreen.
// Returns the effective address and a boolean indicating if it maps to internal VRAM.
func (ppu *PPU) MirrorNametableAddress(addr uint16) (effectiveAddr uint16, isInternalVRAM bool) {
	if addr < 0x2000 || addr >= 0x3F00 {
		log.Printf("Warning: MirrorNametableAddress called with non-nametable address %04X", addr)
		return addr, false // Return original address, marked as not internal
	}

	relativeAddr := addr & 0x0FFF                                                    // Address relative to 0x2000 (0x0000 - 0x0FFF)
	vMirror, hMirror, fourScreen, singleScreen, singleScreenBank := ppu.Cart.GetCurrentMirroringType() // Use the correct method

	if fourScreen {
		effectiveAddr = 0x2000 | relativeAddr // Use full 4KB range, based at $2000
		isInternalVRAM = false                // Handled by mapper/cartridge RAM
	} else if singleScreen {
		if singleScreenBank == 0 { // Low bank
			effectiveAddr = relativeAddr & 0x03FF // Always map to first 1KB (physical NT0)
		} else { // High bank
			effectiveAddr = 0x0400 | (relativeAddr & 0x03FF) // Always map to second 1KB (physical NT1)
		}
		isInternalVRAM = true
	} else if vMirror { // Vertical Mirroring
		effectiveAddr = relativeAddr & 0x07FF // Mask to 0x0000-0x07FF range (physical NT0/NT1)
		isInternalVRAM = true
	} else if hMirror { // Horizontal Mirroring
		if relativeAddr < 0x0800 { // Top half (NT0 or NT1 -> maps to physical NT0)
			effectiveAddr = relativeAddr & 0x03FF // Mask to 0x0000-0x03FF range
		} else { // Bottom half (NT2 or NT3 -> maps to physical NT1)
			effectiveAddr = 0x0400 | (relativeAddr & 0x03FF) // Mask to 0x0400-0x07FF range
		}
		isInternalVRAM = true
	} else {
		log.Printf("Warning: Unknown mirroring state (v:%v h:%v 4s:%v ss:%v bank:%d), defaulting to HORIZONTAL", vMirror, hMirror, fourScreen, singleScreen, singleScreenBank)
		// Default to HORIZONTAL mirroring logic
		if relativeAddr < 0x0800 {
			effectiveAddr = relativeAddr & 0x03FF
		} else {
			effectiveAddr = 0x0400 | (relativeAddr & 0x03FF)
		}
		isInternalVRAM = true
	}

	// Add the $2000 base back only if mapping to internal VRAM for indexing VRAM array
	if isInternalVRAM {
		effectiveAddr += 0x2000
	}
	// For external VRAM (like FourScreen), effectiveAddr already includes the $2000 base.

	return effectiveAddr, isInternalVRAM
}

// ReadPPUMemory reads a byte from PPU mapped memory (Pattern tables, Nametables, Palettes)
func (ppu *PPU) ReadPPUMemory(addr uint16) byte {
	addr &= 0x3FFF // PPU address space mask

	switch {
	case addr < 0x2000: // Pattern Tables (CHR ROM/RAM via Cartridge/Mapper)
		physicalCHRAddr := ppu.Cart.Mapper.MapPPU(addr)
		chrData := ppu.Cart.CHR // Access the potentially modified CHR RAM or ROM copy

		// Ensure physicalCHRAddr is valid before accessing chrData
		if physicalCHRAddr == 0xFFFF { // 0xFFFF indicates unmapped/invalid
			//log.Printf("Warning: PPU Read CHR mapped to invalid address %04X from PPU address %04X", physicalCHRAddr, addr)
			return 0 // Return 0 for invalid mapping
		}

		if int(physicalCHRAddr) < len(chrData) {
			return chrData[physicalCHRAddr]
		}

		//log.Printf("Warning: PPU Read CHR mapped address %04X out of CHR buffer bounds (%d) for PPU address %04X", physicalCHRAddr, len(chrData), addr)
		return 0 // Return 0 to prevent crash on out-of-bounds read

	case addr >= 0x2000 && addr < 0x3F00: // Nametables
		mappedAddr, isInternal := ppu.MirrorNametableAddress(addr)

		if isInternal {
			offset := mappedAddr - 0x2000 // Calculate offset within the 2KB VRAM
			if offset < uint16(len(ppu.IO.VRAM)) {
				return ppu.IO.VRAM[offset]
			}
			log.Printf("Warning: PPU Read internal VRAM mapped address %04X (offset %04X) out of bounds", mappedAddr, offset)
			return 0
		} else {
			// Four-screen or other mapper-handled VRAM
			// Let the mapper handle the read via MapPPU
			physicalAddr := ppu.Cart.Mapper.MapPPU(mappedAddr) // Use the mirrored address for MapPPU
			if physicalAddr == 0xFFFF {                        // Check if mapper returned invalid
				log.Printf("Warning: PPU Read mapper-handled VRAM %04X mapped to invalid address %04X", mappedAddr, physicalAddr)
				return 0
			}
			chrData := ppu.Cart.CHR // Assume mapped to CHR space
			if int(physicalAddr) < len(chrData) {
				return chrData[physicalAddr]
			}
			log.Printf("Warning: PPU Read attempted for mapper-handled VRAM at %04X (mapped to %04X) - Out of CHR bounds?", addr, physicalAddr)
			return 0
		}

	case addr >= 0x3F00: // Palettes
		paletteAddr := (addr - 0x3F00) % 32
		// Mirroring: $3F10/$3F14/$3F18/$3F1C mirror $3F00/$3F04/$3F08/$3F0C
		if paletteAddr == 0x10 || paletteAddr == 0x14 || paletteAddr == 0x18 || paletteAddr == 0x1C {
			paletteAddr -= 0x10
		}
		if paletteAddr < uint16(len(ppu.IO.PaletteRAM)) {
			// Palette reads are not buffered
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
	addr &= 0x3FFF // PPU address space mask

	switch {
	case addr < 0x2000: // Pattern Tables (CHR RAM via Cartridge/Mapper)
		// Only allow writes if the cartridge uses CHR RAM
		if ppu.Cart.GetCHRSize() == 0 { // CHR Size 0 implies RAM
			physicalCHRAddr := ppu.Cart.Mapper.MapPPU(addr)
			if physicalCHRAddr == 0xFFFF {
				//log.Printf("Warning: PPU Write CHR RAM mapped to invalid address %04X from PPU address %04X", physicalCHRAddr, addr)
				return
			}
			chrRAM := ppu.Cart.CHR // Access the CHR slice, which should be RAM
			if int(physicalCHRAddr) < len(chrRAM) {
				chrRAM[physicalCHRAddr] = data
			} else {
				log.Printf("Warning: PPU Write CHR RAM mapped address %04X out of CHR RAM bounds (%d)", physicalCHRAddr, len(chrRAM))
			}
		} else {
			// Attempting to write to CHR ROM, usually ignored or handled by mapper
			// Let the mapper decide if it needs this write.
			ppu.Cart.Mapper.Write(addr, data)
		}

	case addr >= 0x2000 && addr < 0x3F00: // Nametables
		mappedAddr, isInternal := ppu.MirrorNametableAddress(addr)

		if isInternal {
			offset := mappedAddr - 0x2000
			if offset < uint16(len(ppu.IO.VRAM)) {
				ppu.IO.VRAM[offset] = data
			} else {
				log.Printf("Warning: PPU Write internal VRAM mapped address %04X (offset %04X) out of bounds", mappedAddr, offset)
			}
		} else {
			// Four-screen or other mapper-handled VRAM
			// Let the mapper handle the write
			ppu.Cart.Mapper.Write(mappedAddr, data) // Pass the mapped address to the mapper's generic write handler
		}

	case addr >= 0x3F00: // Palettes
		paletteAddr := (addr - 0x3F00) % 32
		// Mirroring
		if paletteAddr == 0x10 || paletteAddr == 0x14 || paletteAddr == 0x18 || paletteAddr == 0x1C {
			paletteAddr -= 0x10
		}
		if paletteAddr < uint16(len(ppu.IO.PaletteRAM)) {
			ppu.IO.PaletteRAM[paletteAddr] = data
		} else {
			log.Printf("Warning: PPU Write Palette RAM address %04X (offset %02X) out of bounds", addr, paletteAddr)
		}
	}
}

// ReadRegister handles CPU reads from PPU registers ($2000-$2007)
func (ppu *PPU) ReadRegister(addr uint16) byte {
	reg := addr & 0x07 // Mask to handle mirroring
	var data byte

	switch reg {
	case 0x02: // PPUSTATUS ($2002)
		status := ppu.IO.PPUSTATUS.Get() | (ppu.IO.LastRegWrite & 0x1F) // Combine flags and bus noise
		ppu.IO.PPUSTATUS.VBLANK = false                                 // Reading $2002 clears VBlank flag
		ppu.w = 0                                                       // Reading $2002 resets the address latch toggle
		// NMI flag in IO struct is cleared *after* status is read (delayed effect)
		ppu.IO.NMI = false // Clear NMI flag immediately in our model after reading status
		data = status

	case 0x04: // OAMDATA ($2004)
		// Reads during rendering (visible scanlines 0-239, cycles 1-64 for sprite eval) can return garbage/FF.
		// Reads during VBLANK or HBLANK (cycles > 256) return valid data. OAMADDR is not incremented by reads.
		// Simplified: Always return current OAM data based on OAMADDR.
		data = ppu.IO.OAM[ppu.IO.OAMADDR]
		// OAMADDR does not increment on read.

	case 0x07: // PPUDATA ($2007)
		// Read from VRAM/CHR/Palette via PPU address 'v'
		dataToReturn := ppu.IO.PPU_DATA_BUFFER // Get value from the internal data buffer (read from previous cycle)

		// Read new value from VRAM/etc into buffer for the *next* read
		currentRead := ppu.ReadPPUMemory(ppu.v)

		// Palette addresses ($3F00-$3FFF) bypass the buffer on read
		if ppu.v >= 0x3F00 {
			dataToReturn = currentRead // Return the palette value read this cycle
			// The buffer gets filled with the data from the mirrored Nametable address *underneath* the palette
			mirroredAddr := 0x2000 | (ppu.v & 0x0FFF) // Nametable mirror of the palette address
			ppu.IO.PPU_DATA_BUFFER = ppu.ReadPPUMemory(mirroredAddr)
		} else {
			// For non-palette reads, return the previously buffered value,
			// and update the buffer with the value read this cycle.
			ppu.IO.PPU_DATA_BUFFER = currentRead
		}

		// Increment 'v' after the read cycle completes
		ppu.incrementVramAddress()
		data = dataToReturn

	default:
		// Reading write-only registers ($2000, $2001, $2003, $2005, $2006) returns open bus value.
		data = ppu.IO.LastRegWrite
	}
	return data
}

// WriteRegister handles CPU writes to PPU registers ($2000-$2007)
func (ppu *PPU) WriteRegister(addr uint16, data byte) {
	ppu.IO.LastRegWrite = data // Store last write for open bus emulation
	reg := addr & 0x07         // Mask to handle mirroring

	switch reg {
	case 0x00: // PPUCTRL ($2000)
		oldNMIEnable := ppu.IO.PPUCTRL.GEN_NMI
		ppu.IO.PPUCTRL.Set(data)
		ppu.t = (ppu.t & 0xF3FF) | (uint16(data&0x03) << 10) // Update 't' nametable select bits (NN)

		// Trigger NMI edge if VBLANK is currently set and NMI output was just enabled
		if ppu.IO.PPUSTATUS.VBLANK && ppu.IO.PPUCTRL.GEN_NMI && !oldNMIEnable {
			ppu.IO.TriggerNMI()
		}

	case 0x01: // PPUMASK ($2001)
		ppu.IO.PPUMASK.Set(data)

	case 0x03: // OAMADDR ($2003)
		ppu.IO.OAMADDR = data

	case 0x04: // OAMDATA ($2004)
		// Write to OAM[OAMADDR] and increment OAMADDR.
		// Writes during rendering are ignored/corrupted on real HW.
		// Simplified: Allow writes anytime. Add accurate timing later if needed.
		ppu.IO.OAM[ppu.IO.OAMADDR] = data
		ppu.IO.OAMADDR++ // Increment after write (wraps automatically due to byte type)

	case 0x05: // PPUSCROLL ($2005)
		if ppu.w == 0 { // First write (X scroll)
			ppu.t = (ppu.t & 0xFFE0) | (uint16(data) >> 3) // Coarse X scroll (5 bits) into t
			ppu.x = data & 0x07                         // Fine X scroll (3 bits)
			ppu.w = 1
		} else { // Second write (Y scroll)
			ppu.t = (ppu.t & 0x8C1F) | (uint16(data&0xF8) << 2) // Coarse Y scroll (5 bits) into t
			ppu.t = (ppu.t & 0x0FFF) | (uint16(data&0x07) << 12) // Fine Y scroll (3 bits) into t
			ppu.w = 0
		}

	case 0x06: // PPUADDR ($2006)
		if ppu.w == 0 { // First write (High byte)
			ppu.t = (ppu.t & 0x00FF) | (uint16(data&0x3F) << 8) // High 6 bits written to t (top 2 bits are cleared)
			ppu.t &= 0x3FFF                                     // Ensure address is max 14 bits ($0000-$3FFF)
			ppu.w = 1
		} else { // Second write (Low byte)
			ppu.t = (ppu.t & 0xFF00) | uint16(data) // Low 8 bits written to t
			ppu.v = ppu.t                           // Copy temporary address (t) to current address (v)
			ppu.w = 0
		}

	case 0x07: // PPUDATA ($2007)
		ppu.WritePPUMemory(ppu.v, data)
		ppu.incrementVramAddress() // Increment 'v' after write
	}
}

// incrementVramAddress increments 'v' by 1 or 32 based on PPUCTRL bit 2.
func (ppu *PPU) incrementVramAddress() {
	inc := uint16(1)
	if ppu.IO.PPUCTRL.VRAM_INCREMENT_32 {
		inc = 32
	}
	// Increment v, wrapping around at $4000 (effectively masking to 14 bits)
	ppu.v = (ppu.v + inc) & 0x3FFF
}

// StartPPU initializes the PPU state.
func StartPPU(io *ioports.IOPorts, cart *cartridge.Cartridge) (*PPU, error) {
	if io == nil || cart == nil {
		return nil, fmt.Errorf("cannot start PPU with nil IOPorts or Cartridge")
	}
	if cart.Mapper == nil {
		return nil, fmt.Errorf("cannot start PPU with uninitialized Mapper in Cartridge")
	}

	ppu := &PPU{}
	fmt.Printf("Starting PPU: RICOH RP-2C02 (Fullscreen)\n")

	ppu.IO = io
	ppu.Cart = cart

	ppu.CYC = 0
	ppu.SCANLINE = -1 // Start at pre-render scanline
	ppu.frameOdd = false
	ppu.SCREEN_DATA = make([]uint32, SCREEN_WIDTH*SCREEN_HEIGHT) // Initialize framebuffer

	// Reset internal PPU state and IO port registers related to PPU
	ppu.v = 0
	ppu.t = 0
	ppu.x = 0
	ppu.w = 0

	ppu.IO.PPUCTRL.Set(0)
	ppu.IO.PPUMASK.Set(0)
	ppu.IO.PPUSTATUS.Set(0) // Flags should be clear initially
	ppu.IO.OAMADDR = 0
	ppu.IO.PPU_DATA_BUFFER = 0
	ppu.IO.LastRegWrite = 0
	ppu.IO.NMI = false

	// Reset background pipeline state
	ppu.nt_byte = 0
	ppu.at_byte = 0
	ppu.tile_data_lo = 0
	ppu.tile_data_hi = 0
	ppu.bg_pattern_shift_lo = 0
	ppu.bg_pattern_shift_hi = 0
	ppu.bg_attr_shift_lo = 0
	ppu.bg_attr_shift_hi = 0

	// Reset sprite pipeline state
	ppu.spriteCount = 0
	ppu.spriteZeroHitPossible = false
	ppu.spriteZeroBeingRendered = false
	for i := range ppu.secondaryOAM {
		ppu.secondaryOAM[i] = 0xFF // Init secondary OAM (clear with FF)
	}
	for i := 0; i < 8; i++ {
		ppu.spritePatternsLo[i] = 0
		ppu.spritePatternsHi[i] = 0
		ppu.spriteCountersX[i] = 0xFF // Mark as inactive
		ppu.spriteLatches[i] = 0
		ppu.spriteIsSprite0[i] = false
	}

	// Initialize OAM memory (often to $FF or 0, depends on test ROMs)
	for i := range ppu.IO.OAM {
		ppu.IO.OAM[i] = 0xFF // Common default, helps hide garbage sprites
	}
	// Palette RAM is often undefined on power-up, but 0 is a safe default
	for i := range ppu.IO.PaletteRAM {
		ppu.IO.PaletteRAM[i] = 0
	}
	// Internal VRAM is also often undefined, 0 is safe default
	for i := range ppu.IO.VRAM {
		ppu.IO.VRAM[i] = 0
	}

	ppu.colors = loadPalette()

	// Initialize SDL Canvas (now fullscreen) - Call the method from ppu_display.go
	err := ppu.initCanvas()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize SDL canvas: %w", err)
	}

	fmt.Printf("PPU Initialization complete (Fullscreen Mode)\n")
	return ppu, nil
}

// isRenderingEnabled checks if background or sprite rendering is enabled via PPUMASK.
func (ppu *PPU) isRenderingEnabled() bool {
	return ppu.IO.PPUMASK.SHOW_BACKGROUND || ppu.IO.PPUMASK.SHOW_SPRITE
}

// incrementScrollX handles the horizontal VRAM address increment (coarse X and nametable).
func (ppu *PPU) incrementScrollX() {
	if !ppu.isRenderingEnabled() {
		return
	}
	// Increment coarse X (bits 0-4 of v)
	if (ppu.v & 0x001F) == 31 { // If coarse X = 31
		ppu.v &= ^uint16(0x001F) // Coarse X = 0
		ppu.v ^= 0x0400          // Switch horizontal nametable (flip bit 10)
	} else {
		ppu.v++ // Increment coarse X
	}
}

// incrementScrollY handles the vertical VRAM address increment (fine Y, coarse Y, nametable).
func (ppu *PPU) incrementScrollY() {
	if !ppu.isRenderingEnabled() {
		return
	}
	// Increment fine Y (bits 12-14 of v)
	if (ppu.v & 0x7000) != 0x7000 { // If fine Y < 7
		ppu.v += 0x1000 // Increment fine Y
	} else {
		ppu.v &= ^uint16(0x7000) // Fine Y = 0
		// Increment coarse Y (bits 5-9 of v)
		y := (ppu.v & 0x03E0) >> 5 // Get coarse Y (0-31)
		if y == 29 {              // If coarse Y is 29 (last row of tiles in a nametable)
			y = 0               // Coarse Y = 0
			ppu.v ^= 0x0800     // Switch vertical nametable (flip bit 11)
		} else if y == 31 { // Coarse Y can wrap from 31 to 0 without switching nametable (e.g., entering attribute table rows)
			y = 0 // Coarse Y = 0
			// Don't switch vertical nametable here
		} else {
			y++ // Increment coarse Y normally
		}
		ppu.v = (ppu.v & ^uint16(0x03E0)) | (y << 5) // Put coarse Y back into v
	}
}

// transferAddressX copies horizontal bits from t to v.
// Happens at cycle 257 of visible/pre-render scanlines if rendering enabled.
func (ppu *PPU) transferAddressX() {
	if !ppu.isRenderingEnabled() {
		return
	}
	// Copy coarse X (bits 0-4) and horizontal nametable select (bit 10)
	ppu.v = (ppu.v & 0xFBE0) | (ppu.t & 0x041F) // Keep V's Y bits, copy T's X bits
}

// transferAddressY copies vertical bits from t to v.
// Happens during cycles 280-304 of pre-render scanline if rendering enabled.
func (ppu *PPU) transferAddressY() {
	if !ppu.isRenderingEnabled() {
		return
	}
	// Copy fine Y (bits 12-14), coarse Y (bits 5-9), and vertical nametable select (bit 11)
	ppu.v = (ppu.v & 0x841F) | (ppu.t & 0x7BE0) // Keep V's X bits, copy T's Y bits
}

// loadBackgroundShifters loads fetched tile data into background shift registers.
func (ppu *PPU) loadBackgroundShifters() {
	// Load pattern data into lower bytes of 16-bit shifters
	ppu.bg_pattern_shift_lo = (ppu.bg_pattern_shift_lo & 0xFF00) | uint16(ppu.tile_data_lo)
	ppu.bg_pattern_shift_hi = (ppu.bg_pattern_shift_hi & 0xFF00) | uint16(ppu.tile_data_hi)

	// Determine attribute bits for the current tile based on 'v' address
	shift := ((ppu.v >> 4) & 4) | (ppu.v & 2)     // Selects the correct 2 bits from AT byte (0, 2, 4, or 6)
	palette_bits := (ppu.at_byte >> shift) & 0x03 // Get 2 palette index bits for the quadrant

	// Expand these 2 bits into 8-bit values to fill the attribute shifters' lower bytes
	attr_fill_lo := uint16(0x0000) // Holds palette bit 0 expanded
	if (palette_bits & 0x01) != 0 {
		attr_fill_lo = 0x00FF
	}
	attr_fill_hi := uint16(0x0000) // Holds palette bit 1 expanded
	if (palette_bits & 0x02) != 0 {
		attr_fill_hi = 0x00FF
	}

	// Load attribute data into the lower bytes of 16-bit shifters
	ppu.bg_attr_shift_lo = (ppu.bg_attr_shift_lo & 0xFF00) | attr_fill_lo
	ppu.bg_attr_shift_hi = (ppu.bg_attr_shift_hi & 0xFF00) | attr_fill_hi
}

// updateShifters shifts background and sprite registers each cycle if rendering is enabled.
func (ppu *PPU) updateShifters() {
	// Shift Background Registers
	if ppu.IO.PPUMASK.SHOW_BACKGROUND {
		ppu.bg_pattern_shift_lo <<= 1
		ppu.bg_pattern_shift_hi <<= 1
		ppu.bg_attr_shift_lo <<= 1
		ppu.bg_attr_shift_hi <<= 1
	}

	// Shift Sprite Registers
	if ppu.IO.PPUMASK.SHOW_SPRITE {
		// Iterate through the sprites loaded for the current scanline
		for i := 0; i < ppu.spriteCount; i++ {
			if ppu.spriteCountersX[i] > 0 {
				ppu.spriteCountersX[i]-- // Decrement X counter if sprite is not yet active
			} else {
				// If counter is 0, the sprite is active; shift its pattern data
				ppu.spritePatternsLo[i] <<= 1
				ppu.spritePatternsHi[i] <<= 1
			}
		}
	}
}

// --- Memory Fetch Helper Functions ---

// fetchNTByte fetches the Nametable byte based on the current VRAM address 'v'.
func (ppu *PPU) fetchNTByte() {
	if !ppu.isRenderingEnabled() {
		return
	}
	addr := 0x2000 | (ppu.v & 0x0FFF) // Nametable base + 12 lower bits of v
	ppu.nt_byte = ppu.ReadPPUMemory(addr)
}

// fetchATByte fetches the Attribute Table byte based on 'v'.
func (ppu *PPU) fetchATByte() {
	if !ppu.isRenderingEnabled() {
		return
	}
	// Address: 0x23C0 | Nametable select | Coarse Y / 4 | Coarse X / 4
	addr := 0x23C0 | (ppu.v & 0x0C00) | ((ppu.v >> 4) & 0x38) | ((ppu.v >> 2) & 0x07)
	ppu.at_byte = ppu.ReadPPUMemory(addr)
}

// fetchTileDataLow fetches the low byte of the background tile pattern based on 'v' and PPUCTRL.
func (ppu *PPU) fetchTileDataLow() {
	if !ppu.isRenderingEnabled() {
		return
	}
	fineY := (ppu.v >> 12) & 7                  // Fine Y scroll from v (bits 12-14)
	patternTable := ppu.IO.PPUCTRL.BACKGROUND_ADDR // BG Pattern Table base ($0000 or $1000)
	tileIndex := uint16(ppu.nt_byte)            // Tile index from Nametable byte
	// Address: pattern_table + tile_index * 16 + fine_y
	addr := patternTable + tileIndex*16 + fineY
	ppu.tile_data_lo = ppu.ReadPPUMemory(addr)
}

// fetchTileDataHigh fetches the high byte of the background tile pattern.
func (ppu *PPU) fetchTileDataHigh() {
	if !ppu.isRenderingEnabled() {
		return
	}
	fineY := (ppu.v >> 12) & 7
	patternTable := ppu.IO.PPUCTRL.BACKGROUND_ADDR
	tileIndex := uint16(ppu.nt_byte)
	// Address: pattern_table + tile_index * 16 + fine_y + 8 (high byte plane)
	addr := patternTable + tileIndex*16 + fineY + 8
	ppu.tile_data_hi = ppu.ReadPPUMemory(addr)
}

// evaluateSprites scans primary OAM to find sprites visible on the *next* scanline.
// Populates secondary OAM and sets sprite overflow flag.
func (ppu *PPU) evaluateSprites() {
	// This evaluation happens during cycles 1-256 of visible/pre-render scanlines
	// The result (secondary OAM) is used for fetching on cycles 257-320.

	// Clear secondary OAM (prepare for next scanline's sprites)
	for i := range ppu.secondaryOAM {
		ppu.secondaryOAM[i] = 0xFF // Fill with $FF (indicates empty slot)
	}
	ppu.spriteCount = 0
	ppu.IO.PPUSTATUS.SPRITE_OVERFLOW = false // Clear overflow flag for this evaluation
	ppu.spriteZeroHitPossible = false        // Reset sprite 0 possibility for the next line

	spriteHeight := 8
	if ppu.IO.PPUCTRL.SPRITE_SIZE_16 {
		spriteHeight = 16
	}

	// Scan primary OAM (ppu.IO.OAM) - 64 sprites, 4 bytes each
	oamIdx := 0 // Start at OAM[0]
	primaryOAM := ppu.IO.OAM
	numSpritesFound := 0

	for n := 0; n < 64; n++ {
		spriteY := int(primaryOAM[oamIdx]) // OAM Y is top edge coordinate (0-239)
		scanlineToCheck := ppu.SCANLINE    // We evaluate for the *next* scanline, which is currently being rendered (SCANLINE)

		// Check if the sprite is vertically in range for the next scanline.
		// Sprite is visible if scanline >= spriteY and scanline < spriteY + height
		if scanlineToCheck >= 0 && scanlineToCheck >= spriteY && scanlineToCheck < (spriteY+spriteHeight) {
			// Sprite is vertically in range. Add to secondary OAM if space.
			if numSpritesFound < 8 {
				targetIdx := numSpritesFound * 4
				ppu.secondaryOAM[targetIdx+0] = primaryOAM[oamIdx+0] // Y
				ppu.secondaryOAM[targetIdx+1] = primaryOAM[oamIdx+1] // Tile Index
				ppu.secondaryOAM[targetIdx+2] = primaryOAM[oamIdx+2] // Attributes
				ppu.secondaryOAM[targetIdx+3] = primaryOAM[oamIdx+3] // X

				// Check if this is sprite 0 being added to secondary OAM
				if n == 0 {
					ppu.spriteZeroHitPossible = true // Mark that sprite 0 is present for the *next* scanline
				}
				numSpritesFound++
			} else {
				// More than 8 sprites found. Set overflow flag.
				ppu.IO.PPUSTATUS.SPRITE_OVERFLOW = true
				// Hardware bug emulation: OAM scan continues with complex buggy reads/writes.
				// Simplified: Stop evaluation once overflow is detected for performance.
				break
			}
		}
		oamIdx += 4 // Move to next sprite entry (Y, Tile, Attr, X)
	} // End OAM scan loop
	ppu.spriteCount = numSpritesFound // Store the actual number of sprites found (0-8)
}

// fetchSprites loads pattern data for the sprites found during evaluation (for the *current* rendering scanline).
// Uses data from secondary OAM populated during the *previous* scanline's evaluation.
func (ppu *PPU) fetchSprites() {
	// Fetching happens during cycles 257-320 of visible/pre-render scanlines.
	// The data fetched here is used for rendering *this* scanline.

	// Clear sprite buffers first to prevent rendering stale data if sprites are disabled
	for i := 0; i < 8; i++ {
		ppu.spriteCountersX[i] = 0xFF // Mark inactive
		ppu.spriteLatches[i] = 0
		ppu.spritePatternsLo[i] = 0
		ppu.spritePatternsHi[i] = 0
		ppu.spriteIsSprite0[i] = false // Reset sprite 0 flag for all slots
	}

	if !ppu.IO.PPUMASK.SHOW_SPRITE {
		return // Don't fetch if sprites aren't shown
	}

	spriteHeight := 8
	if ppu.IO.PPUCTRL.SPRITE_SIZE_16 {
		spriteHeight = 16
	}

	// Fetch data for sprites placed in secondaryOAM (up to spriteCount found previously)
	for i := 0; i < ppu.spriteCount; i++ {
		// Data from secondary OAM for the sprite being loaded
		spriteY := uint16(ppu.secondaryOAM[i*4+0]) // OAM Y coordinate
		tileIndex := ppu.secondaryOAM[i*4+1]
		attributes := ppu.secondaryOAM[i*4+2]
		spriteX := ppu.secondaryOAM[i*4+3]

		// Load sprite state for the rendering pipeline
		ppu.spriteCountersX[i] = spriteX    // X position counter for shifting
		ppu.spriteLatches[i] = attributes // Attribute latch (palette, priority, flip)

		// Determine if this slot *might* correspond to sprite 0.
		// This relies on spriteZeroHitPossible being set if OAM[0] was found AND
		// assuming the first sprite found (if it was OAM[0]) goes into slot 0.
		// This is an approximation. A more robust method would track the OAM index (0-63).
		ppu.spriteIsSprite0[i] = ppu.spriteZeroHitPossible && (i == 0)

		// Determine pattern row based on vertical flip and current scanline
		flipHoriz := (attributes & 0x40) != 0
		flipVert := (attributes & 0x80) != 0

		scanlineToRender := uint16(ppu.SCANLINE) // Current scanline being rendered
		// Calculate row relative to sprite's top edge (spriteY is the screen Y coord where sprite top appears)
		row := scanlineToRender - spriteY

		if flipVert {
			row = uint16(spriteHeight-1) - row // Adjust row for vertical flip
		}

		// Determine pattern table and tile address
		var tileAddr uint16
		var patternTable uint16

		if spriteHeight == 8 { // 8x8 sprites
			patternTable = ppu.IO.PPUCTRL.SPRITE_8_ADDR // Select $0000 or $1000 based on PPUCTRL bit 3
			row &= 7                                    // Ensure row is within 0-7
			tileAddr = patternTable + uint16(tileIndex)*16 + row
		} else { // 8x16 sprites
			// Pattern table determined by bit 0 of tile index
			patternTable = uint16(tileIndex & 0x01) * 0x1000 // $0000 or $1000
			tileIndexBase := tileIndex & 0xFE                // Mask off bit 0 to get the index of the top tile

			if row >= 8 {        // Rendering the bottom half of the 8x16 sprite
				tileIndexBase++ // Use the next tile index (bottom tile)
				row -= 8        // Adjust row to be 0-7 for the bottom tile
			}
			row &= 7 // Ensure row is within 0-7
			tileAddr = patternTable + uint16(tileIndexBase)*16 + row
		}

		// Fetch pattern bytes from CHR ROM/RAM
		tileLo := ppu.ReadPPUMemory(tileAddr)
		tileHi := ppu.ReadPPUMemory(tileAddr + 8) // High plane is 8 bytes offset

		// Apply horizontal flip if needed by reversing bits
		if flipHoriz {
			tileLo = reverseByte(tileLo)
			tileHi = reverseByte(tileHi)
		}

		// Load fetched data into the sprite pipeline registers for this slot
		ppu.spritePatternsLo[i] = tileLo
		ppu.spritePatternsHi[i] = tileHi
	}
}

// Helper to reverse bits in a byte (used for horizontal flip).
func reverseByte(b byte) byte {
	b = (b&0xF0)>>4 | (b&0x0F)<<4
	b = (b&0xCC)>>2 | (b&0x33)<<2
	b = (b&0xAA)>>1 | (b&0x55)<<1
	return b
}

// renderPixel determines and outputs the final pixel color for the current CYC and SCANLINE
// into the SCREEN_DATA framebuffer.
func (ppu *PPU) renderPixel() {
	// Pixel coordinates derived from current cycle and scanline
	pixelX := ppu.CYC - 1  // X coordinate on screen (0-255 for cycles 1-256)
	pixelY := ppu.SCANLINE // Y coordinate on screen (0-239)

	// Bounds check for safety, although should be ensured by calling context
	if pixelX < 0 || pixelX >= SCREEN_WIDTH || pixelY < 0 || pixelY >= SCREEN_HEIGHT {
		return
	}

	// --- Determine Background Pixel ---
	bgPixel := byte(0)   // 2-bit pixel value (0-3) from pattern tables
	bgPalette := byte(0) // 2-bit palette index (0-3) from attribute table
	bgIsOpaque := false

	if ppu.IO.PPUMASK.SHOW_BACKGROUND {
		// Check horizontal clipping mask (leftmost 8 pixels)
		if !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_BACKGROUND) {
			// Select bit from background shifters based on fine X scroll
			mux := uint16(0x8000) >> ppu.x // Mask to select the correct bit based on fine X (0..7)

			// Get pixel bits from pattern shifters
			p0_bg := boolToByte((ppu.bg_pattern_shift_lo & mux) > 0) // Bit 0
			p1_bg := boolToByte((ppu.bg_pattern_shift_hi & mux) > 0) // Bit 1
			bgPixel = (p1_bg << 1) | p0_bg                           // Combine bits (00, 01, 10, 11)

			// Get palette bits from attribute shifters only if pixel is not transparent (pixel value != 0)
			if bgPixel != 0 {
				bgIsOpaque = true
				pal0_bg := boolToByte((ppu.bg_attr_shift_lo & mux) > 0) // Palette Bit 0
				pal1_bg := boolToByte((ppu.bg_attr_shift_hi & mux) > 0) // Palette Bit 1
				bgPalette = (pal1_bg << 1) | pal0_bg                   // Combine palette bits
			}
		}
	}

	// --- Determine Sprite Pixel ---
	sprPixel := byte(0)   // 2-bit pixel value (0-3)
	sprPalette := byte(0) // 2-bit palette index (0-3)
	sprPriority := byte(1) // 0 = In front of background, 1 = Behind background
	sprIsOpaque := false
	spriteZeroPixelRendered := false // Track if an *opaque* pixel from sprite 0 is being rendered *at this specific cycle*

	if ppu.IO.PPUMASK.SHOW_SPRITE {
		// Check horizontal clipping mask (leftmost 8 pixels)
		if !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_SPRITE) {
			// Iterate through the 8 sprite slots loaded for this scanline
			for i := 0; i < ppu.spriteCount; i++ { // Iterate only active sprites
				// Check if this sprite is active at the current pixel X (counter is 0)
				if ppu.spriteCountersX[i] == 0 {
					// Get pixel bits from the sprite's pattern shifters (highest bit = leftmost pixel)
					p0_spr := (ppu.spritePatternsLo[i] >> 7) & 1
					p1_spr := (ppu.spritePatternsHi[i] >> 7) & 1
					currentSprPixelData := (p1_spr << 1) | p0_spr

					// If this is an *opaque* pixel from an active sprite, and we haven't found an opaque sprite pixel yet
					if currentSprPixelData != 0 && !sprIsOpaque {
						// This is the highest priority opaque sprite pixel found *so far* for this X coordinate.
						sprPixel = currentSprPixelData
						sprPalette = (ppu.spriteLatches[i] & 0x03)       // Lower 2 bits of attributes = palette index
						sprPriority = (ppu.spriteLatches[i] & 0x20) >> 5 // Bit 5 = priority (0=FG, 1=BG)
						sprIsOpaque = true

						// Check if this opaque pixel belongs to sprite 0 using our tracked flag
						if ppu.spriteIsSprite0[i] { // Check if this slot was identified as holding sprite 0
							spriteZeroPixelRendered = true // An opaque pixel from sprite 0 is rendering now
						}

						// Found the highest priority sprite for this X, stop searching (hardware behavior)
						break
					}
				}
			} // End sprite slot loop
		}
	}

	// --- Combine Background & Sprite based on priority and transparency ---
	finalPixel := byte(0)
	finalPalette := byte(0) // This will be the high bits (palette select) for the final color lookup

	// Priority Multiplexer Logic
	if !bgIsOpaque && !sprIsOpaque { // Both transparent
		finalPixel = 0 // Use universal background color 0
		finalPalette = 0
	} else if !bgIsOpaque && sprIsOpaque { // BG transparent, Sprite opaque
		finalPixel = sprPixel
		finalPalette = sprPalette + 4 // Sprite palettes start at index 4 (addresses $3F10, $3F14, ..)
	} else if bgIsOpaque && !sprIsOpaque { // BG opaque, Sprite transparent
		finalPixel = bgPixel
		finalPalette = bgPalette
	} else { // Both BG and Sprite are opaque
		// --- Sprite 0 Hit Detection ---
		// Occurs if an opaque BG pixel and an opaque Sprite 0 pixel are rendered on the same cycle (pixelX 0-254).
		if spriteZeroPixelRendered && bgIsOpaque && pixelX >= 0 && pixelX < 255 {
			// Also check clipping windows. Hit cannot occur in leftmost 8 pixels if either BG or SP rendering is disabled there.
			showBG := !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_BACKGROUND)
			showSP := !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_SPRITE)
			if showBG && showSP {
				// Check PPUSTATUS hasn't already been set this frame (it's cleared on pre-render)
				if !ppu.IO.PPUSTATUS.SPRITE_0_BIT {
					ppu.IO.PPUSTATUS.SPRITE_0_BIT = true // Set the Sprite 0 Hit flag
				}
			}
		}

		// Now resolve priority
		if sprPriority == 0 { // Sprite has priority (in front of background)
			finalPixel = sprPixel
			finalPalette = sprPalette + 4
		} else { // Background has priority (sprite behind background)
			finalPixel = bgPixel
			finalPalette = bgPalette
		}
	}

	// --- Final Color Lookup ---
	// Determine the address in palette RAM
	var paletteAddr uint16
	if finalPixel == 0 {
		paletteAddr = 0x3F00 // Universal background color always from $3F00
	} else {
		paletteAddr = 0x3F00 | (uint16(finalPalette) << 2) | uint16(finalPixel)
	}

	// Read the 6-bit color index from palette RAM (handles mirroring internally)
	colorEntryIndex := ppu.ReadPPUMemory(paletteAddr)

	// Apply Grayscale if enabled
	if ppu.IO.PPUMASK.GREYSCALE {
		colorEntryIndex &= 0x30 // Mask to grey component (use bits 4-5 as index)
	}

	// Look up the final ARGB color from the pre-loaded palette table
	finalColor := ppu.colors[colorEntryIndex&0x3F] // Mask index to 6 bits (0-63)

	// --- Write to Screen Buffer (Framebuffer) ---
	bufferIndex := pixelX + (pixelY * SCREEN_WIDTH)
	if bufferIndex >= 0 && bufferIndex < len(ppu.SCREEN_DATA) {
		ppu.SCREEN_DATA[bufferIndex] = finalColor
	} else {
		// This should ideally not happen if logic is correct
		log.Printf("Warning: RenderPixel calculated out-of-bounds index %d (X:%d, Y:%d)", bufferIndex, pixelX, pixelY)
	}
}

// Helper to convert bool to byte (0 or 1).
func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// Process executes one PPU cycle, updating state and potentially rendering a pixel.
func Process(ppu *PPU) {

	// --- Scanline -1: Pre-render Scanline ---
	if ppu.SCANLINE == -1 {
		// Cycle 1: Clear VBlank, Sprite 0 Hit, Sprite Overflow flags. NMI line goes low.
		if ppu.CYC == 1 {
			ppu.IO.PPUSTATUS.VBLANK = false
			ppu.IO.PPUSTATUS.SPRITE_0_BIT = false
			ppu.IO.PPUSTATUS.SPRITE_OVERFLOW = false
			ppu.IO.NMI = false // Explicitly clear NMI flag here (if it wasn't already cleared by CPU reading $2002)
		}

		// Handle background fetches/shifts for the *upcoming* scanline 0.
		ppu.handleBackgroundFetchingAndShifting()

		// Cycles 280-304: If rendering enabled, repeatedly copy vertical bits (Y scroll, nametable) from t to v.
		if ppu.isRenderingEnabled() && ppu.CYC >= 280 && ppu.CYC <= 304 {
			ppu.transferAddressY()
		}

		// Cycles 257-320: Sprite Evaluation & Fetching for Scanline 0
		// Sprite Evaluation (Simplified: Happens conceptually during cycles 1-256, result ready by 257)
		if ppu.CYC == 256 && ppu.isRenderingEnabled() {
			ppu.evaluateSprites() // Evaluate sprites for scanline 0
		}
		// Sprite Fetching (Simplified: Happens conceptually during cycles 257-320, patterns loaded into shifters)
		if ppu.CYC == 257 && ppu.isRenderingEnabled() {
			ppu.fetchSprites() // Fetch patterns for scanline 0 based on above evaluation
		}

		// Cycle 257 also copies horizontal address bits if rendering is enabled
		if ppu.isRenderingEnabled() && ppu.CYC == 257 {
			ppu.transferAddressX()
		}

		// *** MMC3 IRQ Clocking ***
		// Clock the mapper's IRQ counter near the end of the visible rendering fetches.
		// Cycle 260 is a common approximation. Only clock if rendering is enabled.
		if ppu.isRenderingEnabled() && ppu.CYC == 260 {
			ppu.Cart.ClockIRQCounter()
		}

	// --- Scanlines 0-239: Visible Scanlines ---
	} else if ppu.SCANLINE >= 0 && ppu.SCANLINE <= 239 {

		// Cycles 1-256: Render pixel if rendering enabled.
		if ppu.isRenderingEnabled() && ppu.CYC >= 1 && ppu.CYC <= 256 {
			ppu.renderPixel() // Writes to SCREEN_DATA framebuffer
		}

		// Background Fetches & Shifting (Cycles 1-256, 321-336)
		ppu.handleBackgroundFetchingAndShifting()

		// Cycle 256: If rendering enabled, increment vertical scroll (fine Y, coarse Y, nametable).
		if ppu.isRenderingEnabled() && ppu.CYC == 256 {
			ppu.incrementScrollY()
		}

		// Cycle 257: If rendering enabled, copy horizontal bits from t to v.
		if ppu.isRenderingEnabled() && ppu.CYC == 257 {
			ppu.transferAddressX()
		}

		// Cycles 257-320: Sprite Evaluation & Fetching for NEXT scanline (SL+1)
		// Sprite Evaluation (Simplified: Happens conceptually during cycles 1-256)
		if ppu.CYC == 256 && ppu.isRenderingEnabled() {
			ppu.evaluateSprites() // Evaluate sprites for scanline SL+1
		}
		// Sprite Fetching (Simplified: Happens conceptually during cycles 257-320)
		if ppu.CYC == 257 && ppu.isRenderingEnabled() {
			ppu.fetchSprites() // Fetch patterns for scanline SL based on eval from SL-1
		}

		// *** MMC3 IRQ Clocking ***
		// Clock the mapper's IRQ counter near the end of the visible rendering fetches.
		// Cycle 260 is a common approximation. Only clock if rendering is enabled.
		if ppu.isRenderingEnabled() && ppu.CYC == 260 {
			ppu.Cart.ClockIRQCounter()
		}

	// --- Scanline 240: Post-render Scanline ---
	} else if ppu.SCANLINE == 240 {
		// PPU is idle. Frame data in SCREEN_DATA is complete.
		// No rendering, no VRAM access related to rendering pipeline.

	// --- Scanlines 241-260: Vertical Blanking Interval ---
	} else if ppu.SCANLINE >= 241 && ppu.SCANLINE <= 260 {
		// VBlank Start (Scanline 241, Cycle 1)
		if ppu.SCANLINE == 241 && ppu.CYC == 1 {
			ppu.IO.PPUSTATUS.VBLANK = true // Set VBlank flag
			if ppu.IO.PPUCTRL.GEN_NMI {
				ppu.IO.TriggerNMI() // Signal NMI if enabled
			}
			// ---- FRAME BUFFER UPDATE TO TEXTURE ----
			// Update screen & Check Keyboard once per frame AFTER VBlank starts
			// This is where the completed SCREEN_DATA buffer is copied to the SDL texture.
			ppu.ShowScreen()      // Call method defined in ppu_display.go
			ppu.CheckKeyboard() // Call method defined in ppu_display.go
		}
	} // End of scanline type checks

	// --- Advance Cycle and Scanline ---
	ppu.CYC++
	if ppu.CYC >= CYCLES_PER_SCANLINE {
		ppu.CYC = 0 // Reset cycle count for next scanline
		ppu.SCANLINE++
		if ppu.SCANLINE > 260 { // Finished VBlank scanline 260
			ppu.SCANLINE = -1 // Wrap to pre-render scanline
			ppu.frameOdd = !ppu.frameOdd // Toggle frame oddness

			// Odd Frame Cycle Skip: On odd frames, if rendering is enabled,
			// the first cycle (cycle 0) of the pre-render scanline (-1) is skipped.
			if ppu.frameOdd && ppu.isRenderingEnabled() {
				ppu.CYC = 1 // Start scanline -1 at cycle 1 instead of 0
			}
		}
	}
} // End Process function

// handleBackgroundFetchingAndShifting contains the logic for fetching BG data and shifting registers.
// Called during pre-render and visible scanlines.
func (ppu *PPU) handleBackgroundFetchingAndShifting() {
	if !ppu.isRenderingEnabled() {
		return
	}

	// Update Shifters (Cycles 2-257 and 322-337)
	// Shifting happens before fetching/loading within the 8-cycle pattern.
	if (ppu.CYC >= 2 && ppu.CYC <= 257) || (ppu.CYC >= 322 && ppu.CYC <= 337) {
		ppu.updateShifters()
	}

	// Background Memory Fetches (Cycles 1-256 and 321-336)
	// Fetch happens based on cycle within an 8-cycle pattern
	isFetchRange := (ppu.CYC >= 1 && ppu.CYC <= 256) || (ppu.CYC >= 321 && ppu.CYC <= 336)

	if isFetchRange {
		fetchCycleMod8 := ppu.CYC % 8
		switch fetchCycleMod8 {
		case 1: // Cycle 1, 9, 17, ..., 249, 321, 329 -> Start of fetch cycle
			// Load shifters with next tile data (fetched on previous cycle 7)
			ppu.loadBackgroundShifters()
			// Fetch Nametable byte for the *next* tile (address based on v)
			ppu.fetchNTByte()
		case 3: // Cycle 3, 11, ..., 251, 323, 331
			ppu.fetchATByte() // Fetch Attribute Table byte for the next tile
		case 5: // Cycle 5, 13, ..., 253, 325, 333
			ppu.fetchTileDataLow() // Fetch low plane of pattern data for the next tile
		case 7: // Cycle 7, 15, ..., 255, 327, 335
			ppu.fetchTileDataHigh() // Fetch high plane of pattern data
		case 0: // Cycle 8, 16, ..., 256, 328, 336 -> End of 8-cycle pattern
			// Increment horizontal scroll position in 'v' *after* the last fetch of the group
			// This happens ONLY if the cycle is within the active fetch ranges
			// Increment happens on cycles 8, 16, ... 256, 328, 336
			if ppu.CYC <= 256 || (ppu.CYC >= 328 && ppu.CYC <= 336) {
				ppu.incrementScrollX()
			}
			// Loading of shifters with the data fetched on cycle 7 happens at the *start* of the next cycle (case 1)
		}
	}
}