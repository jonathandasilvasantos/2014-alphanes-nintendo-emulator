package ppu

import (
	"fmt"
	"zerojnt/cartridge"
	"zerojnt/ioports"

	"github.com/veandco/go-sdl2/sdl"
)

// Constants for PPU operation
const (
	SCREEN_WIDTH  = 256
	SCREEN_HEIGHT = 240

	TOTAL_SCANLINES     = 262
	CYCLES_PER_SCANLINE = 341

	// PPU Memory Map Addresses
	PATTERN_TABLE_0 uint16 = 0x0000
	PATTERN_TABLE_1 uint16 = 0x1000
	NAMETABLE_0     uint16 = 0x2000
	NAMETABLE_1     uint16 = 0x2400
	NAMETABLE_2     uint16 = 0x2800
	NAMETABLE_3     uint16 = 0x2C00
	PALETTE_RAM     uint16 = 0x3F00
)

// PPU struct definition
type PPU struct {
	// Framebuffer
	SCREEN_DATA []uint32

	// Timing
	CYC      int
	SCANLINE int
	frameOdd bool

	// SDL Resources
	texture  *sdl.Texture
	renderer *sdl.Renderer
	window   *sdl.Window

	// Shared Resources
	IO   *ioports.IOPorts
	Cart *cartridge.Cartridge

	// Internal PPU Registers
	v uint16 // Current VRAM address (15 bit)
	t uint16 // Temporary VRAM address (15 bit); can also be thought of as the address of the top-left corner of the screen
	x byte   // Fine X scroll (3 bit)
	w byte   // First or second write toggle (1 bit)

	// Background rendering pipeline state
	nt_byte             byte   // Fetched Nametable byte
	at_byte             byte   // Fetched Attribute Table byte
	tile_data_lo        byte   // Fetched low byte of background tile pattern
	tile_data_hi        byte   // Fetched high byte of background tile pattern
	bg_pattern_shift_lo uint16 // Shift register for low pattern bits (16-bit wide)
	bg_pattern_shift_hi uint16 // Shift register for high pattern bits (16-bit wide)
	bg_attr_shift_lo    uint16 // Shift register for low attribute bits (palette index) (16-bit wide)
	bg_attr_shift_hi    uint16 // Shift register for high attribute bits (palette index) (16-bit wide)

	// Sprite rendering state
	secondaryOAM [32]byte // Holds sprite data for the *next* scanline (8 sprites * 4 bytes/sprite)
	spriteCount  int      // Number of sprites found for the next scanline (0-8)

	// Sprite shift registers and latches for the *current* scanline
	spritePatternsLo [8]byte // Low pattern bits for up to 8 sprites
	spritePatternsHi [8]byte // High pattern bits for up to 8 sprites
	spriteCountersX  [8]byte // X position counters for up to 8 sprites
	spriteLatches    [8]byte // Attribute latches for up to 8 sprites
	spriteIsSprite0  [8]bool // Flags if the sprite in this pipeline slot is the original sprite 0

	spriteZeroHitPossible   bool // Set during evaluation if sprite 0 is found for the next line
	spriteZeroBeingRendered bool // Set during rendering if sprite 0's pixel is currently being output (internal flag)

	skipRenderThisFrame bool // Flag to control rendering skip // *** ADDED FOR FRAME SKIP ***
	lastA12State        bool // Tracks the previous state of A12 for edge detection (used by mapper clock)

	// Color Palette
	colors [64]uint32 // Pre-calculated ARGB8888 palette colors
}

// StartPPU initializes the PPU state
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
	ppu.SCREEN_DATA = make([]uint32, SCREEN_WIDTH*SCREEN_HEIGHT)

	// Reset internal PPU state
	ppu.v = 0
	ppu.t = 0
	ppu.x = 0
	ppu.w = 0
	ppu.IO.PPUCTRL.Set(0)
	ppu.IO.PPUMASK.Set(0)
	ppu.IO.PPUSTATUS.Set(0)
	ppu.IO.OAMADDR = 0
	ppu.IO.PPU_DATA_BUFFER = 0
	ppu.IO.LastRegWrite = 0
	ppu.IO.NMI = false
	ppu.skipRenderThisFrame = false // Initialize skip flag // *** ADDED FOR FRAME SKIP ***

	// Reset pipelines
	ppu.nt_byte, ppu.at_byte, ppu.tile_data_lo, ppu.tile_data_hi = 0, 0, 0, 0
	ppu.bg_pattern_shift_lo, ppu.bg_pattern_shift_hi = 0, 0
	ppu.bg_attr_shift_lo, ppu.bg_attr_shift_hi = 0, 0
	ppu.spriteCount = 0
	ppu.spriteZeroHitPossible = false
	ppu.lastA12State = false // Initialize A12 tracking
	ppu.spriteZeroBeingRendered = false
	for i := range ppu.secondaryOAM {
		ppu.secondaryOAM[i] = 0xFF
	}
	for i := 0; i < 8; i++ {
		ppu.spritePatternsLo[i], ppu.spritePatternsHi[i] = 0, 0
		ppu.spriteCountersX[i] = 0xFF
		ppu.spriteLatches[i] = 0
		ppu.spriteIsSprite0[i] = false
	}

	// Initialize memories
	for i := range ppu.IO.OAM {
		ppu.IO.OAM[i] = 0xFF
	} // Typically FF on power-up, but 0 often used
	for i := range ppu.IO.PaletteRAM {
		ppu.IO.PaletteRAM[i] = 0
	}
	for i := range ppu.IO.VRAM {
		ppu.IO.VRAM[i] = 0
	}

	ppu.colors = loadPalette()

	// Initialize SDL Canvas
	err := ppu.initCanvas()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize SDL canvas: %w", err)
	}

	fmt.Printf("PPU Initialization complete (Fullscreen Mode)\n")
	return ppu, nil
}

// loadPalette loads the standard NES palette into a [64]uint32 array (ARGB)
func loadPalette() [64]uint32 {
	// Standard NES Palette (ARGB8888 format: Alpha Red Green Blue)
	palette := [64]uint32{
		0xFF7C7C7C, 0xFF0000FC, 0xFF0000BC, 0xFF4428BC, 0xFF940084, 0xFFA80020, 0xFFA81000, 0xFF881400, // 0x00-0x07
		0xFF503000, 0xFF007800, 0xFF006800, 0xFF005800, 0xFF004058, 0xFF000000, 0xFF000000, 0xFF000000, // 0x08-0x0F
		0xFFBCBCBC, 0xFF0078F8, 0xFF0058F8, 0xFF6844FC, 0xFFD800CC, 0xFFE40058, 0xFFF83800, 0xFFE45C10, // 0x10-0x17
		0xFFAC7C00, 0xFF00B800, 0xFF00A800, 0xFF00A844, 0xFF008888, 0xFF000000, 0xFF000000, 0xFF000000, // 0x18-0x1F
		0xFFF8F8F8, 0xFF3CBCFC, 0xFF6888FC, 0xFF9878F8, 0xFFF878F8, 0xFFF85898, 0xFFF87858, 0xFFFCA044, // 0x20-0x27
		0xFFF8B800, 0xFFB8F818, 0xFF58D854, 0xFF58F898, 0xFF00E8D8, 0xFF787878, 0xFF000000, 0xFF000000, // 0x28-0x2F
		0xFFFCFCFC, 0xFFA4E4FC, 0xFFB8B8F8, 0xFFD8B8F8, 0xFFF8B8F8, 0xFFF8A4C0, 0xFFF0D0B0, 0xFFFCE0A8, // 0x30-0x37
		0xFFF8D878, 0xFFD8F878, 0xFFB8F8B8, 0xFFB8F8D8, 0xFF00FCFC, 0xFFF8D8F8, 0xFF000000, 0xFF000000, // 0x38-0x3F
	}
	return palette
}

// Helper methods

// isRenderingEnabled checks if background or sprite rendering is enabled
func (ppu *PPU) isRenderingEnabled() bool {
	return ppu.IO.PPUMASK.SHOW_BACKGROUND || ppu.IO.PPUMASK.SHOW_SPRITE
}

// incrementVramAddress increments 'v' by 1 or 32 based on PPUCTRL bit
func (ppu *PPU) incrementVramAddress() {
	inc := uint16(1)
	if ppu.IO.PPUCTRL.VRAM_INCREMENT_32 {
		inc = 32
	}
	ppu.v = (ppu.v + inc) & 0x3FFF // Wrap around PPU memory map (0x0000-0x3FFF)
}

// Convert bool to byte (0 or 1)
func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// SetSkipRender sets the flag indicating whether the PPU should skip rendering the current frame.
// This is called by the main emulator loop.
// *** ADDED FOR FRAME SKIP ***
func (ppu *PPU) SetSkipRender(skip bool) {
	ppu.skipRenderThisFrame = skip
}