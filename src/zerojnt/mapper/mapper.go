// File: ./mapper/mapper.go
package mapper

import (
	"fmt"
)

// HeaderInfo defines a simplified header structure for mapper use
// This makes it easier for mappers to get needed info without parsing flags again.
type HeaderInfo struct {
	ROM_SIZE              byte   // PRG Size in 16KB units
	VROM_SIZE             byte   // CHR Size in 8KB units (0 means CHR RAM)
	Mapper                int    // Mapper number
	VerticalMirroring     bool   // Initial vertical mirroring state
	HorizontalMirroring   bool   // Initial horizontal mirroring state
	SRAM                  bool   // Cartridge has SRAM (potentially battery-backed)
	Trainer               bool   // Cartridge has trainer data
	FourScreenVRAM        bool   // Cartridge uses four-screen VRAM layout
	SingleScreenMirroring bool   // Calculated: Is it single screen?
	SingleScreenBank      byte   // Which bank for single screen (0 or 1)
	MMC1Variant           string // Detected MMC1 board type (e.g., "SNROM", "SUROM")
}

// MapperAccessor interface defines methods the Cartridge must provide for Mappers.
type MapperAccessor interface {
	// --- Data Access ---
	GetHeader() HeaderInfo                                      // Provides parsed header info
	GetPRGSize() uint32                                         // Returns size of OriginalPRG
	GetCHRSize() uint32                                         // Returns size of OriginalCHR (0 if CHR RAM)
	CopyPRGData(destOffset uint32, srcOffset uint32, length uint32) // Copy from OriginalPRG to Cartridge.PRG
	CopyCHRData(destOffset uint32, srcOffset uint32, length uint32) // Copy from OriginalCHR to Cartridge.CHR

	// --- RAM Access ---
	HasSRAM() bool                      // Check if SRAM ($6000-$7FFF) is present
	GetPRGRAMSize() uint32              // Get size of allocated SRAM
	WriteSRAM(offset uint16, value byte) // Write to SRAM (handles banking internally if needed by Cartridge)
	// Note: Reading SRAM is typically handled directly by CPU memory map via Cartridge.SRAM

	// --- CHR RAM Specific ---
	GetCHRRAMSize() uint32 // Returns size of allocated CHR RAM (typically 8KB if used)
	// Note: Reads/Writes to CHR RAM are typically handled directly by PPU memory map via Cartridge.CHR

	// --- Mirroring Control ---
	HasFourScreenVRAM() bool                                                 // Check four-screen VRAM flag
	SetMirroringMode(vertical, horizontal, fourScreen bool, singleScreenBank byte) // Tell Cartridge how to mirror nametables

	// --- IRQ Handling (Needed for MMC3 and others) ---
	IRQState() bool // Returns true if the mapper is currently asserting an IRQ
	ClockIRQCounter() // Clocks the mapper's IRQ counter (e.g., MMC3 scanline counter)
}

// Mapper interface defines the methods that all mappers must implement.
type Mapper interface {
	// Initialize sets up the mapper state based on the cartridge.
	// It should perform initial configuration but NOT necessarily the first bank copy.
	Initialize(cart MapperAccessor)

	// Reset puts the mapper into its known power-on/reset state.
	// This MUST configure the initial bank mapping and mirroring.
	Reset()

	// MapCPU maps a CPU address ($6000-$FFFF) to a PRG ROM/RAM offset.
	// Returns (isROM bool, mappedAddr uint16).
	// `isROM` is true for PRG ROM, false for RAM ($6000-$7FFF).
	// `mappedAddr` is the offset within the corresponding memory space
	// (either the 32KB mapped PRG window or the SRAM).
	// Returns mappedAddr=0xFFFF (or similar) if access is invalid/unmapped.
	MapCPU(addr uint16) (isROM bool, mappedAddr uint16)

	// MapPPU maps a PPU address ($0000-$1FFF) to a CHR ROM/RAM offset.
	// Returns the offset within the 8KB mapped CHR window.
	// Returns mappedAddr=0xFFFF (or similar) if access is invalid/unmapped.
	MapPPU(addr uint16) uint16

	// Write handles CPU writes to mapper registers ($8000-$FFFF) or potentially PRG RAM ($6000-$7FFF).
	// This method is responsible for updating internal mapper state and triggering bank/mirroring updates.
	Write(addr uint16, value byte)

	// --- IRQ Handling ---
	IRQState() bool // Check if the mapper is currently asserting the IRQ line
	ClockIRQCounter() // Signal the mapper to clock its IRQ counter (e.g., on PPU A12 edge or scanline)
}

// MapperError represents mapper-specific errors
type MapperError struct {
	Operation string
	Message   string
}

func (e *MapperError) Error() string {
	return fmt.Sprintf("Mapper Error during %s: %s", e.Operation, e.Message)
}

// Memory bank size constants (use consistent uint32)
const (
	PRG_BANK_SIZE   uint32 = 16384 // 16KB PRG ROM bank size
	PRG_BANK_SIZE_8K uint32 = 8192 // 8KB PRG ROM bank size
	CHR_BANK_SIZE   uint32 = 8192  // 8KB CHR ROM/RAM bank size
	CHR_BANK_SIZE_1K uint32 = 1024 // 1KB CHR bank size
	CHR_BANK_SIZE_2K uint32 = 2048 // 2KB CHR bank size
	SRAM_BANK_SIZE  uint32 = 8192  // 8KB SRAM size (common size)
)

// Helper function to check if a number is a power of two
// This is useful for mappers that rely on power-of-two calculations
func isPowerOfTwo(n uint32) bool {
	return n > 0 && (n&(n-1)) == 0
}