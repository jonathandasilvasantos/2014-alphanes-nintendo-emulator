// File: ./mapper/mapper.go
package mapper

import (
	"fmt"
)

// HeaderInfo contains parsed NES ROM header information for mappers
type HeaderInfo struct {
	ROM_SIZE              byte   // PRG Size in 16KB units
	VROM_SIZE             byte   // CHR Size in 8KB units (0 means CHR RAM)
	Mapper                int    // Mapper number
	VerticalMirroring     bool   // Initial vertical mirroring state
	HorizontalMirroring   bool   // Initial horizontal mirroring state
	SRAM                  bool   // Cartridge has SRAM
	Trainer               bool   // Cartridge has trainer data
	FourScreenVRAM        bool   // Cartridge uses four-screen VRAM layout
	SingleScreenMirroring bool   // Is it single screen
	SingleScreenBank      byte   // Which bank for single screen (0 or 1)
	MMC1Variant           string // Detected MMC1 board type
}

// MapperAccessor interface defines methods the Cartridge must provide for Mappers
type MapperAccessor interface {
	GetHeader() HeaderInfo
	GetPRGSize() uint32
	GetCHRSize() uint32
	CopyPRGData(destOffset uint32, srcOffset uint32, length uint32)
	CopyCHRData(destOffset uint32, srcOffset uint32, length uint32)

	HasSRAM() bool
	GetPRGRAMSize() uint32
	WriteSRAM(offset uint16, value byte)

	GetCHRRAMSize() uint32

	HasFourScreenVRAM() bool
	SetMirroringMode(vertical, horizontal, fourScreen bool, singleScreenBank byte)

	IRQState() bool
	ClockIRQCounter()
}

// Mapper interface defines the methods that all mappers must implement
type Mapper interface {
	Initialize(cart MapperAccessor)
	Reset()
	MapCPU(addr uint16) (isROM bool, mappedAddr uint16)
	MapPPU(addr uint16) uint16
	Write(addr uint16, value byte)
	IRQState() bool
	ClockIRQCounter()
}

// MapperError represents mapper-specific errors
type MapperError struct {
	Operation string
	Message   string
}

func (e *MapperError) Error() string {
	return fmt.Sprintf("Mapper Error during %s: %s", e.Operation, e.Message)
}

// Memory bank size constants
const (
	PRG_BANK_SIZE    uint32 = 16384 // 16KB PRG ROM bank size
	PRG_BANK_SIZE_8K uint32 = 8192  // 8KB PRG ROM bank size
	CHR_BANK_SIZE    uint32 = 8192  // 8KB CHR ROM/RAM bank size
	CHR_BANK_SIZE_1K uint32 = 1024  // 1KB CHR bank size
	CHR_BANK_SIZE_2K uint32 = 2048  // 2KB CHR bank size
	SRAM_BANK_SIZE   uint32 = 8192  // 8KB SRAM size
)

// isPowerOfTwo checks if a number is a power of two
func isPowerOfTwo(n uint32) bool {
	return n > 0 && (n&(n-1)) == 0
}