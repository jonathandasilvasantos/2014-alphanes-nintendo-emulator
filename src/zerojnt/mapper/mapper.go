package mapper

import (
	"fmt"
)

// MapperAccessor interface defines methods needed by mappers
// This replaces the old CartridgeAccessor and uses a simplified HeaderInfo
type MapperAccessor interface {
	GetHeader() HeaderInfo // Simplified header info
	GetPRG() []byte
	GetCHR() []byte
	GetPRGSize() uint32
	GetCHRSize() uint32
	GetPRGRAMSize() uint32
	SetPRGRAMSize(size uint32)
	GetCHRRAMSize() uint32
	SetCHRRAMSize(size uint32)
	CopyPRGData(destOffset uint32, srcOffset uint32, length uint32)
	CopyCHRData(destOffset uint32, srcOffset uint32, length uint32)
	HasVerticalMirroring() bool
	HasHorizontalMirroring() bool
	HasFourScreenVRAM() bool
	HasSRAM() bool
	GetSingleScreenBank() byte
	GetMirroringMode() byte
	SetMirroringMode(vertical, horizontal, fourScreen bool, singleScreenBank byte)
	GetMapperNumber() int
}

// HeaderInfo defines a simplified header structure for mapper use
type HeaderInfo struct {
	ROM_SIZE              byte
	VROM_SIZE             byte
	Mapper                int
	VerticalMirroring     bool
	HorizontalMirroring   bool
	SRAM                  bool
	Trainer               bool
	FourScreenVRAM        bool
	SingleScreenMirroring bool
	SingleScreenBank      byte
	MMC1Variant           string
}

// MapperError represents mapper-specific errors
type MapperError struct {
	Operation string
	Message   string
}

func (e *MapperError) Error() string {
	return fmt.Sprintf("Mapper Error during %s: %s", e.Operation, e.Message)
}

// Mapper interface defines the methods that all mappers must implement.
type Mapper interface {
	// Initialize initializes the mapper with data from the cartridge.
	Initialize(cart MapperAccessor) // Now uses MapperAccessor

	// MapCPU maps a CPU address to a physical PRG ROM/RAM address.
	// Returns true if the address maps to PRG ROM, false otherwise.
	MapCPU(addr uint16) (bool, uint16)

	// MapPPU maps a PPU address to a physical CHR ROM/RAM address.
	MapPPU(addr uint16) uint16

	// Write handles writes to mapper registers.
	Write(addr uint16, value byte)
}

// Memory bank size constants
const (
	PRG_BANK_SIZE uint32 = 16384 // 16KB PRG ROM bank size
	CHR_BANK_SIZE uint32 = 8192  // 8KB CHR ROM/RAM bank size
	SRAM_SIZE     uint32 = 8192  // 8KB SRAM size
)