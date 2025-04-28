package ioports

import (
	"zerojnt/cartridge"
)

// PPU_STATUS register ($2002)
type PPU_STATUS struct {
	SPRITE_OVERFLOW bool // Bit 5: Set when >8 sprites on next scanline
	SPRITE_0_BIT    bool // Bit 6: Set when sprite 0 overlaps background
	VBLANK          bool // Bit 7: Set during vertical blank period
}

type Controller struct {
	CurrentButtons byte  `json:"current_buttons"` // Live input state (1=pressed)
	LatchedButtons byte  `json:"latched_buttons"` // Copy made when strobe changes
	Strobe         bool  `json:"strobe"`          // Last value written to $4016 bit0
	ShiftCounter   uint8 `json:"shift_counter"`   // Which bit is next to read
}

// Get returns the byte value of the PPUSTATUS register
func (s *PPU_STATUS) Get() byte {
	var status byte = 0
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

// Set initializes the PPUSTATUS flags from a byte
func (s *PPU_STATUS) Set(data byte) {
	s.SPRITE_OVERFLOW = (data & 0x20) != 0
	s.SPRITE_0_BIT = (data & 0x40) != 0
	s.VBLANK = (data & 0x80) != 0
}

// PPU_MASK register ($2001)
type PPU_MASK struct {
	GREYSCALE                  bool // Bit 0: Enable grayscale
	SHOW_LEFTMOST_8_BACKGROUND bool // Bit 1: Show background in leftmost 8 pixels
	SHOW_LEFTMOST_8_SPRITE     bool // Bit 2: Show sprites in leftmost 8 pixels
	SHOW_BACKGROUND            bool // Bit 3: Show background
	SHOW_SPRITE                bool // Bit 4: Show sprites
	EMPHASIZE_RED              bool // Bit 5: Emphasize red
	EMPHASIZE_GREEN            bool // Bit 6: Emphasize green
	EMPHASIZE_BLUE             bool // Bit 7: Emphasize blue
}

// Set updates the PPU_MASK flags from a byte
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

// Get returns the byte value of the PPUMASK register
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

// PPU_CTRL register ($2000)
type PPU_CTRL struct {
	BASE_NAMETABLE_ADDR_ID byte   // Bits 0,1: Nametable select
	VRAM_INCREMENT_32      bool   // Bit 2: VRAM address increment (0: add 1, 1: add 32)
	SPRITE_8_ADDR          uint16 // Bit 3: Sprite pattern table address
	BACKGROUND_ADDR        uint16 // Bit 4: Background pattern table address
	SPRITE_SIZE_16         bool   // Bit 5: Sprite size (0: 8x8, 1: 8x16)
	MASTER_SLAVE_SELECT    bool   // Bit 6: PPU master/slave select
	GEN_NMI                bool   // Bit 7: Generate NMI on VBlank
}

// Set updates the PPU_CTRL flags from a byte
func (c *PPU_CTRL) Set(data byte) {
	c.BASE_NAMETABLE_ADDR_ID = data & 0x03
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

// Get returns the byte value of the PPUCTRL register
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

// IOPorts holds memory and state for CPU/PPU interaction
type IOPorts struct {
	CPU_RAM [2048]byte // 2KB CPU Internal RAM

	// PPU Memory
	VRAM       [2048]byte // 2KB Nametable RAM
	PaletteRAM [32]byte   // Palette RAM
	OAM        [256]byte  // Object Attribute Memory (Sprites)

	Controllers [2]Controller

	// PPU Registers
	PPUCTRL   PPU_CTRL
	PPUMASK   PPU_MASK
	PPUSTATUS PPU_STATUS
	OAMADDR   byte // OAM Address Register ($2003)

	// PPU Data Buffer for $2007 reads
	PPU_DATA_BUFFER byte

	// Last value written to any PPU register
	LastRegWrite byte

	// NMI Control
	NMI bool // NMI request flag

	// Cartridge Reference
	CART *cartridge.Cartridge

	// DMA State
	OAMDMA_Page       byte // Source page for OAM DMA
	OAMDMA_Transfer   bool // OAM DMA transfer active flag
	OAMDMA_Addr       byte // Current address within DMA source page
	OAMDMA_WaitCycles int  // CPU cycles to wait before DMA

	// CPU cycle impact
	CPU_CYC_INCREASE uint16 // Cycles to add to CPU counter
}

// StartIOPorts initializes the shared IO resources
func StartIOPorts(cart *cartridge.Cartridge) IOPorts {
	var io IOPorts

	io.PPUCTRL.Set(0)
	io.PPUMASK.Set(0)
	io.PPUSTATUS.Set(0)
	io.OAMADDR = 0
	io.PPU_DATA_BUFFER = 0
	io.LastRegWrite = 0

	io.NMI = false

	io.CART = cart
	io.CPU_CYC_INCREASE = 0

	io.OAMDMA_Transfer = false
	io.OAMDMA_WaitCycles = 0

	return io
}

// TriggerNMI sets the NMI request flag
func (io *IOPorts) TriggerNMI() {
	io.NMI = true
}

// ClearNMI clears the NMI request flag
func (io *IOPorts) ClearNMI() {
	io.NMI = false
}

// StartOAMDMA initiates the OAM DMA process
func (io *IOPorts) StartOAMDMA(page byte) {
	io.OAMDMA_Page = page
	io.OAMDMA_Transfer = true
	io.OAMDMA_Addr = 0
}

// DoOAMDMATransfer performs one byte transfer during OAM DMA
func (io *IOPorts) DoOAMDMATransfer(cpuRead func(addr uint16) byte) {
	if !io.OAMDMA_Transfer {
		return
	}

	// Calculate the source address in CPU space
	dmaSourceAddr := (uint16(io.OAMDMA_Page) << 8) | uint16(io.OAMDMA_Addr)

	// Read data using CPU read function
	data := cpuRead(dmaSourceAddr)

	// Write data to PPU's OAM
	io.OAM[io.OAMDMA_Addr] = data

	io.OAMDMA_Addr++

	// Check if transfer is complete
	if io.OAMDMA_Addr == 0 {
		io.OAMDMA_Transfer = false
		io.CPU_CYC_INCREASE = 0 // Reset cycle impact
	}
}