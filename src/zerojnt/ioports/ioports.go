/*
Copyright 2014, 2015 Jonathan da Silva SAntos
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
package ioports

import (
	"log" // Import log package for DMA warning if needed
	"zerojnt/cartridge"
)

// PPU_STATUS ($2002) Register Representation
type PPU_STATUS struct {
	// Lower 5 bits contain the last value written to any PPU register (bus noise/decay)
	// We can simplify and just track the flags, or use LastRegWrite field.
	// Unused          byte // Bits 0-4: Least significant bits previously written into a PPU register
	SPRITE_OVERFLOW bool // Bit 5: Set when >8 sprites found during evaluation for next scanline. Cleared at dot 1 of pre-render line.
	SPRITE_0_BIT    bool // Bit 6: Set when non-zero pixel of sprite 0 overlaps non-zero background pixel. Cleared at dot 1 of pre-render line.
	VBLANK          bool // Bit 7: Vertical Blank flag. Set at dot 1 of scanline 241. Cleared by reading $2002 or at dot 1 of pre-render line.
}

// Get returns the byte value of the PPUSTATUS register
func (s *PPU_STATUS) Get() byte {
	var status byte = 0
	// Or retrieve LastRegWrite & mask? For now, just flags.
	// status |= (LastRegWrite & 0x1F) // Simulate lower 5 bits (requires access to LastRegWrite)
	if s.SPRITE_OVERFLOW {
		status |= 0x20
	}
	if s.SPRITE_0_BIT {
		status |= 0x40
	}
	if s.VBLANK {
		status |= 0x80
	}
	return status
}

// Set initializes the PPUSTATUS flags from a byte (usually 0 at reset)
func (s *PPU_STATUS) Set(data byte) {
	// Typically only flags are set internally, not directly written by CPU
	s.SPRITE_OVERFLOW = (data & 0x20) != 0
	s.SPRITE_0_BIT = (data & 0x40) != 0
	s.VBLANK = (data & 0x80) != 0
	// Lower 5 bits are ignored here as they reflect internal bus state
}

// PPU_MASK ($2001) Register Representation
type PPU_MASK struct {
	GREYSCALE                  bool // Bit 0: 0 = Color, 1 = Grayscale
	SHOW_LEFTMOST_8_BACKGROUND bool // Bit 1: 0 = Hide, 1 = Show background in leftmost 8 pixels
	SHOW_LEFTMOST_8_SPRITE     bool // Bit 2: 0 = Hide, 1 = Show sprites in leftmost 8 pixels
	SHOW_BACKGROUND            bool // Bit 3: 0 = Hide, 1 = Show background
	SHOW_SPRITE                bool // Bit 4: 0 = Hide, 1 = Show sprites
	EMPHASIZE_RED              bool // Bit 5: Emphasize Red (Not usually implemented fully in basic emulators)
	EMPHASIZE_GREEN            bool // Bit 6: Emphasize Green
	EMPHASIZE_BLUE             bool // Bit 7: Emphasize Blue
}

// Set updates the PPU_MASK flags from a byte written by the CPU
func (m *PPU_MASK) Set(data byte) {
	m.GREYSCALE = (data & 0x01) != 0
	m.SHOW_LEFTMOST_8_BACKGROUND = (data & 0x02) != 0
	m.SHOW_LEFTMOST_8_SPRITE = (data & 0x04) != 0
	m.SHOW_BACKGROUND = (data & 0x08) != 0
	m.SHOW_SPRITE = (data & 0x10) != 0
	m.EMPHASIZE_RED = (data & 0x20) != 0
	m.EMPHASIZE_GREEN = (data & 0x40) != 0
	m.EMPHASIZE_BLUE = (data & 0x80) != 0
}

// Get returns the byte value of the PPUMASK register (rarely read)
func (m *PPU_MASK) Get() byte {
	var data byte = 0
	if m.GREYSCALE {
		data |= 0x01
	}
	if m.SHOW_LEFTMOST_8_BACKGROUND {
		data |= 0x02
	}
	if m.SHOW_LEFTMOST_8_SPRITE {
		data |= 0x04
	}
	if m.SHOW_BACKGROUND {
		data |= 0x08
	}
	if m.SHOW_SPRITE {
		data |= 0x10
	}
	if m.EMPHASIZE_RED {
		data |= 0x20
	}
	if m.EMPHASIZE_GREEN {
		data |= 0x40
	}
	if m.EMPHASIZE_BLUE {
		data |= 0x80
	}
	return data
}

// PPU_CTRL ($2000) Register Representation
type PPU_CTRL struct {
	BASE_NAMETABLE_ADDR_ID byte   // Bits 0,1: Nametable select (0 = $2000, 1 = $2400, 2 = $2800, 3 = $2C00) - PPU uses this internally via 't' register
	VRAM_INCREMENT_32      bool   // Bit 2: VRAM address increment per CPU read/write of $2007 (0: add 1, 1: add 32)
	SPRITE_8_ADDR          uint16 // Bit 3: Sprite pattern table address for 8x8 sprites (0: $0000, 1: $1000)
	BACKGROUND_ADDR        uint16 // Bit 4: Background pattern table address (0: $0000, 1: $1000)
	SPRITE_SIZE_16         bool   // Bit 5: Sprite size (0: 8x8, 1: 8x16)
	MASTER_SLAVE_SELECT    bool   // Bit 6: PPU master/slave select (0: read backdrop from EXT pins; 1: output color on EXT pins) - Rarely used
	GEN_NMI                bool   // Bit 7: Generate NMI on VBlank (0: disabled, 1: enabled)
}

// Set updates the PPU_CTRL flags from a byte written by the CPU
func (c *PPU_CTRL) Set(data byte) {
	c.BASE_NAMETABLE_ADDR_ID = data & 0x03 // Store the ID bits
	c.VRAM_INCREMENT_32 = (data & 0x04) != 0
	if (data & 0x08) == 0 {
		c.SPRITE_8_ADDR = 0x0000
	} else {
		c.SPRITE_8_ADDR = 0x1000
	}
	if (data & 0x10) == 0 {
		c.BACKGROUND_ADDR = 0x0000
	} else {
		c.BACKGROUND_ADDR = 0x1000
	}
	c.SPRITE_SIZE_16 = (data & 0x20) != 0
	c.MASTER_SLAVE_SELECT = (data & 0x40) != 0
	c.GEN_NMI = (data & 0x80) != 0
}

// Get returns the byte value of the PPUCTRL register (rarely read)
func (c *PPU_CTRL) Get() byte {
	var data byte = 0
	data |= c.BASE_NAMETABLE_ADDR_ID
	if c.VRAM_INCREMENT_32 {
		data |= 0x04
	}
	if c.SPRITE_8_ADDR == 0x1000 {
		data |= 0x08
	}
	if c.BACKGROUND_ADDR == 0x1000 {
		data |= 0x10
	}
	if c.SPRITE_SIZE_16 {
		data |= 0x20
	}
	if c.MASTER_SLAVE_SELECT {
		data |= 0x40
	}
	if c.GEN_NMI {
		data |= 0x80
	}
	return data
}

// IOPorts holds memory and state shared or directly accessed by CPU/PPU interaction
type IOPorts struct {
	// *** FIX 1: Added CPU_RAM field ***
	CPU_RAM [2048]byte // 2KB CPU Internal RAM ($0000-$07FF mirrored)

	// PPU Specific Memory
	VRAM       [2048]byte // 2KB Nametable RAM (used if not FourScreen)
	PaletteRAM [32]byte   // 32 bytes Palette RAM
	OAM        [256]byte  // 256 bytes Primary Object Attribute Memory (Sprites)

	// PPU Registers State (as seen by CPU)
	PPUCTRL   PPU_CTRL
	PPUMASK   PPU_MASK
	PPUSTATUS PPU_STATUS
	OAMADDR   byte // OAM Address Register ($2003) value

	// PPU Data Buffer (for $2007 reads)
	PPU_DATA_BUFFER byte

	// Last value written to any PPU register ($2000-2007), for open bus reads
	LastRegWrite byte

	// NMI Control
	// *** FIX 2: Renamed NMI_REQ to NMI ***
	NMI bool // Flag indicating PPU wants to assert NMI line (set by PPU, cleared by PPU on $2002 read)

	// Cartridge Reference (might be needed for mappers interacting with IO)
	CART *cartridge.Cartridge

	// DMA State (potentially managed here or in CPU/Bus)
	OAMDMA_Page       byte // Source page for OAM DMA ($xx00-$xxFF) written to $4014
	OAMDMA_Transfer   bool // Flag indicating OAM DMA transfer is active
	OAMDMA_Addr       byte // Current address within the source page for DMA read
	OAMDMA_WaitCycles int  // CPU cycles to wait before starting DMA

	// Placeholder for CPU cycle impact (e.g., from OAM DMA)
	CPU_CYC_INCREASE uint16 // Cycles to add to CPU counter after PPU write (e.g., $4014)
}

// StartIOPorts initializes the shared IO resources.
func StartIOPorts(cart *cartridge.Cartridge) IOPorts {
	var io IOPorts

	// CPU_RAM, VRAM, PaletteRAM, OAM are zero-initialized by default.
	// Explicitly initialize if needed (e.g., OAM to FF).
	// for i := range io.CPU_RAM { io.CPU_RAM[i] = 0 }
	// for i := range io.VRAM { io.VRAM[i] = 0 }
	// for i := range io.PaletteRAM { io.PaletteRAM[i] = 0 } // Palette should probably load defaults? See PPU Start.
	// for i := range io.OAM { io.OAM[i] = 0xFF } // OAM often initialized to $FF

	io.PPUCTRL.Set(0)
	io.PPUMASK.Set(0)
	io.PPUSTATUS.Set(0)
	io.OAMADDR = 0
	io.PPU_DATA_BUFFER = 0
	io.LastRegWrite = 0

	io.NMI = false // Initialize the renamed field

	io.CART = cart // Store cartridge reference
	io.CPU_CYC_INCREASE = 0

	io.OAMDMA_Transfer = false
	io.OAMDMA_WaitCycles = 0

	return io
}

// --- NMI Helper Functions ---

// TriggerNMI sets the NMI request flag. CPU should detect this.
func (io *IOPorts) TriggerNMI() {
	// *** FIX 2: Use renamed field ***
	io.NMI = true
}

// ClearNMI clears the NMI request flag. Called by PPU after $2002 read.
func (io *IOPorts) ClearNMI() {
	// *** FIX 2: Use renamed field ***
	io.NMI = false
}

// --- OAM DMA ($4014) Handling ---
// This is tricky. $4014 is usually handled by the CPU/Bus write logic.
// When CPU writes to $4014, it should:
// 1. Call a function like `StartOAMDMA(io, page)` here.
// 2. Stall itself for ~513/514 cycles.
// 3. During the stall, the PPU/Bus logic performs the 256 byte transfer.

// StartOAMDMA initiates the OAM DMA process state.
// `page` is the value written to $4014 (high byte of CPU source address).
func (io *IOPorts) StartOAMDMA(page byte) {
	io.OAMDMA_Page = page
	io.OAMDMA_Transfer = true
	io.OAMDMA_Addr = 0       // Start reading from $xx00
	io.CPU_CYC_INCREASE = 513 // Base stall cycles (may vary slightly)
	// DMA starts after 1 (or 2 on odd CPU cycles) dummy cycles
	// Let the CPU handle the exact stall timing. We just set the flags.
	// The actual transfer needs to happen during CPU stall, likely driven by CPU/Bus clock ticks
}

// DoOAMDMATransfer performs one byte transfer during OAM DMA.
// This should be called 256 times by the main loop/CPU while DMA is active.
// `cpuRead` is a function passed in to read from CPU memory space.
// Note: cpuRead function MUST handle the memory mapping for the source address $xx00 - $xxFF.
func (io *IOPorts) DoOAMDMATransfer(cpuRead func(addr uint16) byte) {
	if !io.OAMDMA_Transfer {
		return
	}

	// Calculate the source address in CPU space
	dmaSourceAddr := (uint16(io.OAMDMA_Page) << 8) | uint16(io.OAMDMA_Addr)

	// Read the data using the provided CPU read function (which handles CPU RAM, ROM, etc.)
	data := cpuRead(dmaSourceAddr)

	// Write the data to the PPU's OAM
	// NesDev Wiki: "The PPU accesses OAM through the PPU's OAM address port $2003/4.
	// OAMADDR is set to 0 before the transfer begins."
	// Although $2003/OAMADDR might not be 0 *before* the write to $4014,
	// the DMA hardware itself effectively writes to OAM starting at index 0.
	// So we use io.OAMDMA_Addr as the index into io.OAM directly.
	if int(io.OAMDMA_Addr) < len(io.OAM) {
		io.OAM[io.OAMDMA_Addr] = data
	} else {
		// This should theoretically not happen if OAMDMA_Addr wraps correctly
		log.Printf("Warning: OAM DMA write index %d out of bounds", io.OAMDMA_Addr)
	}

	io.OAMDMA_Addr++ // Increment source address offset (wraps automatically due to byte type)

	// Check if transfer is complete (after 256 bytes, OAMDMA_Addr wraps to 0)
	if io.OAMDMA_Addr == 0 {
		io.OAMDMA_Transfer = false
		io.CPU_CYC_INCREASE = 0 // Reset cycle impact after completion (CPU stall ends)
	}
}

// RMPPU / WMPPU are removed. CPU calls ppu.ReadRegister / ppu.WriteRegister directly.
// The exception is $4014 OAM DMA write, which should be handled by the CPU/Bus write logic,
// potentially calling io.StartOAMDMA.

// SetNMI is removed, use io.TriggerNMI()