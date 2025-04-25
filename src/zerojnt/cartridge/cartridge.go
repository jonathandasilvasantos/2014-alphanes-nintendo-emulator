package cartridge

import (
	"fmt"
	"log"
	"os"
	"zerojnt/mapper"
)

const (
	INES_HEADER_SIZE = 16
	TRAINER_SIZE     = 512
	PRG_BANK_SIZE_KB = 16
	CHR_BANK_SIZE_KB = 8
	SRAM_DEFAULT_SIZE = 8192
	CHR_RAM_SIZE      = 8192
	MAPPED_PRG_SIZE   = 32 * 1024
	MAPPED_CHR_SIZE   = 8 * 1024
)

// Header represents the parsed iNES header fields.
type Header struct {
	ID        [4]byte
	PRGSizeKB byte
	CHRSizeKB byte
	Flags6    byte
	Flags7    byte
	Reserved [8]byte

	MapperNum             int
	VerticalMirroring     bool
	HorizontalMirroring   bool
	SRAMEnabled           bool
	TrainerPresent        bool
	FourScreenVRAM        bool
	SingleScreenMirroring bool
	SingleScreenBank      byte
	MMC1Variant           string
}

// Cartridge holds all data and state for a loaded NES cartridge.
type Cartridge struct {
	Header Header

	OriginalPRG []byte
	OriginalCHR []byte

	PRG []byte
	CHR []byte
	SRAM []byte

	Mapper mapper.Mapper

	currentVerticalMirroring     bool
	currentHorizontalMirroring   bool
	currentFourScreenVRAM        bool
	currentSingleScreenMirroring bool
	currentSingleScreenBank      byte

	SRAMDirty bool
}

// LoadRom loads a .nes file, parses it, initializes memory and the mapper.
func LoadRom(filename string) (*Cartridge, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open ROM file '%s': %w", filename, err)
	}
	defer file.Close()

	headerBytes := make([]byte, INES_HEADER_SIZE)
	_, err = file.Read(headerBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read ROM header: %w", err)
	}

	cart := &Cartridge{}
	if err := parseHeader(&cart.Header, headerBytes); err != nil {
		return nil, fmt.Errorf("invalid NES header: %w", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	remainingSize := fileInfo.Size() - INES_HEADER_SIZE
	if remainingSize <= 0 {
		return nil, fmt.Errorf("ROM file contains only a header")
	}

	allData := make([]byte, remainingSize)
	_, err = file.ReadAt(allData, INES_HEADER_SIZE)
	if err != nil {
		return nil, fmt.Errorf("failed to read ROM data: %w", err)
	}

	offset := 0
	if cart.Header.TrainerPresent {
		if len(allData) < TRAINER_SIZE {
			return nil, fmt.Errorf("ROM header indicates trainer, but data is too short")
		}
		offset += TRAINER_SIZE
		log.Println("Skipping 512 byte trainer")
	}

	prgSize := int(cart.Header.PRGSizeKB) * PRG_BANK_SIZE_KB * 1024
	if offset+prgSize > len(allData) {
		return nil, fmt.Errorf("PRG ROM data extends beyond file size (expected %d, file has %d after header/trainer)", prgSize, len(allData)-offset)
	}
	cart.OriginalPRG = make([]byte, prgSize)
	copy(cart.OriginalPRG, allData[offset:offset+prgSize])
	log.Printf("Loaded %d KB PRG ROM", prgSize/1024)
	offset += prgSize

	chrSize := int(cart.Header.CHRSizeKB) * CHR_BANK_SIZE_KB * 1024
	if chrSize > 0 {
		if offset+chrSize > len(allData) {
			return nil, fmt.Errorf("CHR ROM data extends beyond file size (expected %d, file has %d remaining)", chrSize, len(allData)-offset)
		}
		cart.OriginalCHR = make([]byte, chrSize)
		copy(cart.OriginalCHR, allData[offset:offset+chrSize])
		log.Printf("Loaded %d KB CHR ROM", chrSize/1024)
	} else {
		cart.OriginalCHR = make([]byte, 0)
		log.Println("Cartridge uses CHR RAM")
	}

	cart.PRG = make([]byte, MAPPED_PRG_SIZE)
	cart.CHR = make([]byte, MAPPED_CHR_SIZE)

	if cart.Header.CHRSizeKB == 0 {
		cart.CHR = make([]byte, CHR_RAM_SIZE)
		log.Printf("Allocated %d KB CHR RAM", CHR_RAM_SIZE/1024)
	}

	if cart.Header.SRAMEnabled {
		sramSize := SRAM_DEFAULT_SIZE
		cart.SRAM = make([]byte, sramSize)
		log.Printf("Initialized %d KB SRAM", sramSize/1024)
	}

	if cart.Header.MapperNum == 1 {
		detectMMC1Variant(cart)
	}

	switch cart.Header.MapperNum {
	case 0:
		cart.Mapper = &mapper.NROM{}
	case 1:
		cart.Mapper = &mapper.MMC1{}
	case 2:
		cart.Mapper = &mapper.UNROM{}
	case 4:
		cart.Mapper = &mapper.MMC3{}
	default:
		return nil, fmt.Errorf("unsupported mapper number: %d", cart.Header.MapperNum)
	}

	if cart.Mapper == nil {
		return nil, fmt.Errorf("failed to instantiate mapper %d", cart.Header.MapperNum)
	}

	cart.Mapper.Initialize(cart)
	cart.Mapper.Reset()

	log.Printf("Cartridge loaded successfully. Mapper: %d (%T)", cart.Header.MapperNum, cart.Mapper)
	verifyPRGROM(cart)

	return cart, nil
}

// parseHeader validates and extracts information from the iNES header bytes.
func parseHeader(h *Header, b []byte) error {
	if len(b) < INES_HEADER_SIZE {
		return fmt.Errorf("header data too short (%d bytes)", len(b))
	}
	copy(h.ID[:], b[0:4])
	if string(h.ID[:]) != "NES\x1A" {
		return fmt.Errorf("invalid NES identifier '%s'", string(h.ID[:]))
	}

	h.PRGSizeKB = b[4]
	h.CHRSizeKB = b[5]
	h.Flags6 = b[6]
	h.Flags7 = b[7]
	copy(h.Reserved[:], b[8:16])

	if h.PRGSizeKB == 0 {
		return fmt.Errorf("invalid header: PRG ROM size cannot be 0")
	}

	h.MapperNum = int((h.Flags6 >> 4) | (h.Flags7 & 0xF0))

	h.VerticalMirroring = (h.Flags6 & 0x01) != 0
	h.HorizontalMirroring = !h.VerticalMirroring

	h.SRAMEnabled = (h.Flags6 & 0x02) != 0
	h.TrainerPresent = (h.Flags6 & 0x04) != 0
	h.FourScreenVRAM = (h.Flags6 & 0x08) != 0

	if h.FourScreenVRAM {
		h.VerticalMirroring = false
		h.HorizontalMirroring = false
		h.SingleScreenMirroring = false
	} else {
		h.SingleScreenMirroring = false
		h.SingleScreenBank = 0
	}

	if (h.Flags7 & 0x0C) == 0x08 {
		log.Println("NES 2.0 format detected (limited support)")
	}

	log.Printf("Header Parsed: PRG:%dKB CHR:%dKB Map:%d VMir:%v SRAM:%v Trn:%v 4Scr:%v",
		int(h.PRGSizeKB)*PRG_BANK_SIZE_KB,
		int(h.CHRSizeKB)*CHR_BANK_SIZE_KB,
		h.MapperNum,
		h.VerticalMirroring,
		h.SRAMEnabled,
		h.TrainerPresent,
		h.FourScreenVRAM)

	return nil
}

// detectMMC1Variant determines the specific MMC1 board based on ROM/RAM sizes.
func detectMMC1Variant(cart *Cartridge) {
	if cart.Header.MapperNum != 1 {
		return
	}

	prgSizeKB := int(cart.Header.PRGSizeKB) * PRG_BANK_SIZE_KB
	chrSizeKB := int(cart.Header.CHRSizeKB) * CHR_BANK_SIZE_KB
	hasChrRAM := cart.Header.CHRSizeKB == 0
	hasSRAM := cart.Header.SRAMEnabled

	variant := "UNKNOWN"
	switch {
	case prgSizeKB == 128 && hasChrRAM && hasSRAM:
		variant = "SOROM"
	case prgSizeKB >= 512 && !hasChrRAM:
		if prgSizeKB == 512 {
			variant = "SUROM"
		} else {
			variant = "SXROM"
		}
	case prgSizeKB <= 256 && chrSizeKB == 8 && hasSRAM:
		variant = "SNROM"
	case prgSizeKB <= 256 && chrSizeKB == 8 && !hasSRAM:
		variant = "SGROM"
	case prgSizeKB <= 256 && chrSizeKB >= 16 && !hasChrRAM:
		variant = "SKROM"
	case prgSizeKB <= 256 && chrSizeKB >= 16 && hasSRAM && !hasChrRAM:
		variant = "SZROM"
	case hasChrRAM && hasSRAM:
		variant = "SNROM/SOROM/SUROM/SXROM"
	case !hasChrRAM && hasSRAM:
		variant = "SKROM/SZROM"
	case hasChrRAM && !hasSRAM:
		variant = "SGROM"
	case !hasChrRAM && !hasSRAM:
		variant = "SKROM"
	}

	cart.Header.MMC1Variant = variant
	log.Printf("Detected MMC1 variant: %s (PRG:%dKB CHR:%dKB CHR-RAM:%v SRAM:%v)",
		variant, prgSizeKB, chrSizeKB, hasChrRAM, hasSRAM)
}

// GetHeader returns a copy of relevant parsed info for the mapper
func (c *Cartridge) GetHeader() mapper.HeaderInfo {
	return mapper.HeaderInfo{
		ROM_SIZE:              c.Header.PRGSizeKB,
		VROM_SIZE:             c.Header.CHRSizeKB,
		Mapper:                c.Header.MapperNum,
		VerticalMirroring:     c.Header.VerticalMirroring,
		HorizontalMirroring:   c.Header.HorizontalMirroring,
		SRAM:                  c.Header.SRAMEnabled,
		Trainer:               c.Header.TrainerPresent,
		FourScreenVRAM:        c.Header.FourScreenVRAM,
		SingleScreenMirroring: c.Header.SingleScreenMirroring,
		SingleScreenBank:      c.Header.SingleScreenBank,
		MMC1Variant:           c.Header.MMC1Variant,
	}
}

func (c *Cartridge) GetPRGSize() uint32 {
	return uint32(len(c.OriginalPRG))
}

func (c *Cartridge) GetCHRSize() uint32 {
	return uint32(len(c.OriginalCHR))
}

func (c *Cartridge) GetPRGRAMSize() uint32 {
	return uint32(len(c.SRAM))
}

// WriteSRAM writes to the cartridge's SRAM
func (c *Cartridge) WriteSRAM(offset uint16, value byte) {
	if c.Header.SRAMEnabled && int(offset) < len(c.SRAM) {
		if c.SRAM[offset] != value {
			c.SRAM[offset] = value
			c.SRAMDirty = true
		}
	}
}

func (c *Cartridge) GetCHRRAMSize() uint32 {
	if c.Header.CHRSizeKB == 0 {
		return uint32(len(c.CHR))
	}
	return 0
}

// CopyPRGData copies requested bank from OriginalPRG to the mapped PRG window.
func (c *Cartridge) CopyPRGData(destOffset uint32, srcOffset uint32, length uint32) {
	if len(c.OriginalPRG) == 0 {
		return
	}
	if srcOffset+length > uint32(len(c.OriginalPRG)) {
		log.Printf("ERROR: CopyPRGData source out of bounds - srcOffset: %X, length: %X, OriginalPRG size: %X",
			srcOffset, length, len(c.OriginalPRG))
		if srcOffset >= uint32(len(c.OriginalPRG)) {
			return
		}
		length = uint32(len(c.OriginalPRG)) - srcOffset
		if length == 0 { return }
	}

	if destOffset+length > uint32(len(c.PRG)) {
		log.Printf("ERROR: CopyPRGData destination out of bounds - destOffset: %X, length: %X, PRG size: %X",
			destOffset, length, len(c.PRG))
		if destOffset >= uint32(len(c.PRG)) {
			return
		}
		length = uint32(len(c.PRG)) - destOffset
		if length == 0 { return }
	}

	copy(c.PRG[destOffset:destOffset+length], c.OriginalPRG[srcOffset:srcOffset+length])
}

// CopyCHRData copies requested bank from OriginalCHR to the mapped CHR window.
func (c *Cartridge) CopyCHRData(destOffset uint32, srcOffset uint32, length uint32) {
	if c.Header.CHRSizeKB == 0 {
		return
	}

	if len(c.OriginalCHR) == 0 {
		return
	}
	if srcOffset+length > uint32(len(c.OriginalCHR)) {
		log.Printf("ERROR: CopyCHRData source out of bounds - srcOffset: %X, length: %X, OriginalCHR size: %X",
			srcOffset, length, len(c.OriginalCHR))
		if srcOffset >= uint32(len(c.OriginalCHR)) { return }
		length = uint32(len(c.OriginalCHR)) - srcOffset
		if length == 0 { return }
	}

	if destOffset+length > uint32(len(c.CHR)) {
		log.Printf("ERROR: CopyCHRData destination out of bounds - destOffset: %X, length: %X, CHR size: %X",
			destOffset, length, len(c.CHR))
		if destOffset >= uint32(len(c.CHR)) { return }
		length = uint32(len(c.CHR)) - destOffset
		if length == 0 { return }
	}

	copy(c.CHR[destOffset:destOffset+length], c.OriginalCHR[srcOffset:srcOffset+length])
}

func (c *Cartridge) HasSRAM() bool {
	return c.Header.SRAMEnabled
}

func (c *Cartridge) HasFourScreenVRAM() bool {
	return c.Header.FourScreenVRAM
}

// SetMirroringMode updates the cartridge's current mirroring state based on mapper control.
func (c *Cartridge) SetMirroringMode(vertical, horizontal, fourScreen bool, singleScreenBank byte) {
	c.currentVerticalMirroring = vertical
	c.currentHorizontalMirroring = horizontal
	c.currentFourScreenVRAM = fourScreen
	c.currentSingleScreenMirroring = !vertical && !horizontal && !fourScreen
	c.currentSingleScreenBank = singleScreenBank
}

// GetCurrentMirroringType returns the current mirroring mode for the PPU.
func (c *Cartridge) GetCurrentMirroringType() (v, h, four, single bool, bank byte) {
	return c.currentVerticalMirroring, c.currentHorizontalMirroring, c.currentFourScreenVRAM, c.currentSingleScreenMirroring, c.currentSingleScreenBank
}

// IRQState checks the mapper's current IRQ status.
func (c *Cartridge) IRQState() bool {
	if c.Mapper != nil {
		return c.Mapper.IRQState()
	}
	return false
}

// ClockIRQCounter tells the mapper to clock its internal IRQ counter.
func (c *Cartridge) ClockIRQCounter() {
	if c.Mapper != nil {
		c.Mapper.ClockIRQCounter()
	}
}

// verifyPRGROM is a debug helper function
func verifyPRGROM(cart *Cartridge) {
	fmt.Println("PRG ROM Verification")
	expectedSize := int(cart.Header.PRGSizeKB) * PRG_BANK_SIZE_KB * 1024
	fmt.Printf("Original PRG Size: %d bytes\n", len(cart.OriginalPRG))
	fmt.Printf("Mapped PRG Window Size: %d bytes\n", len(cart.PRG))
	if len(cart.OriginalPRG) != expectedSize {
		fmt.Printf("WARNING: Original PRG ROM size mismatch! Header expected: %d, Actual: %d\n",
			expectedSize, len(cart.OriginalPRG))
	}
	if len(cart.PRG) != MAPPED_PRG_SIZE {
		fmt.Printf("WARNING: Mapped PRG window size mismatch! Expected: %d, Actual: %d\n",
			MAPPED_PRG_SIZE, len(cart.PRG))
	}

	if len(cart.OriginalPRG) >= 0x4000 {
		lastBankOffset := len(cart.OriginalPRG) - 0x4000
		resetVectorLow := cart.OriginalPRG[lastBankOffset+0x3FFC]
		resetVectorHigh := cart.OriginalPRG[lastBankOffset+0x3FFD]
		fmt.Printf("Original Reset Vector (@%06X): $%02X%02X\n", lastBankOffset+0x3FFC, resetVectorHigh, resetVectorLow)
	}

	if len(cart.PRG) == MAPPED_PRG_SIZE {
		resetVectorLow := cart.PRG[0x7FFC]
		resetVectorHigh := cart.PRG[0x7FFD]
		fmt.Printf("Mapped Reset Vector (@$FFFC): $%02X%02X\n", resetVectorHigh, resetVectorLow)
	}
	fmt.Println("----------------------------")
}