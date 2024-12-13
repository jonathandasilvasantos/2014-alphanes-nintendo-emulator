package cartridge

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

const (
	PRG_BANK_SIZE = 16384 // 16KB
	CHR_BANK_SIZE = 8192  // 8KB
	HEADER_SIZE   = 16    // iNES header size
	TRAINER_SIZE  = 512   // Size of trainer if present
)

type Header struct {
	ID        [4]byte // NES<EOF>
	ROM_SIZE  byte    // PRG Size in 16KB units
	VROM_SIZE byte    // CHR Size in 8KB units
	ROM_TYPE  byte    // Flags 6
	ROM_TYPE2 byte    // Flags 7
	Reserved  [8]byte // Reserved space in header (should be all zeros)
	RomType   RomType // Parsed ROM type information
}

type Cartridge struct {
	Header      Header
	Data        []byte // Raw ROM data
	PRG         []byte // Program ROM (may be mapped by the mapper)
	CHR         []byte // Character ROM/RAM (may be mapped by the mapper)
	SRAM        []byte // Save RAM if enabled
	OriginalPRG []byte // Original PRG ROM data (for mapper bank switching)
	OriginalCHR []byte // Original CHR ROM data (for mapper bank switching)
}

type RomType struct {
	Mapper                int
	HorizontalMirroring   bool
	VerticalMirroring     bool
	SRAM                  bool
	Trainer               bool
	FourScreenVRAM        bool
	SingleScreenMirroring bool // Indicates single-screen mirroring is used
	SingleScreenBank      byte   // Which nametable bank to use for single-screen (0 or 1)
	MMC1Variant          string // Added to distinguish different MMC1 board variants (SOROM, SUROM, etc.)
}

func verifyPRGROM(cart *Cartridge) {
	expectedSize := int(cart.Header.ROM_SIZE) * PRG_BANK_SIZE
	if len(cart.PRG) != 32768 || len(cart.OriginalPRG) != expectedSize {
		fmt.Printf("WARNING: PRG ROM size mismatch! Expected: %d, Got: %d (Original: %d)\n",
			expectedSize, len(cart.PRG), len(cart.OriginalPRG))
	}

	if len(cart.OriginalPRG) >= 16384 {
		fmt.Printf("First PRG bank begins with: %02X %02X %02X %02X\n",
			cart.OriginalPRG[0], cart.OriginalPRG[1],
			cart.OriginalPRG[2], cart.OriginalPRG[3])

		// Print reset vector
		resetVectorLow := cart.OriginalPRG[0x3FFC]
		resetVectorHigh := cart.OriginalPRG[0x3FFD]
		fmt.Printf("Reset Vector: $%02X%02X\n", resetVectorHigh, resetVectorLow)

		lastOffset := len(cart.OriginalPRG) - 16384
		fmt.Printf("Last PRG bank begins with: %02X %02X %02X %02X\n",
			cart.OriginalPRG[lastOffset],
			cart.OriginalPRG[lastOffset+1],
			cart.OriginalPRG[lastOffset+2],
			cart.OriginalPRG[lastOffset+3])
	}
}

func LoadRom(filename string) Cartridge {
	fmt.Println("Loading ROM:", filename)

	var cart Cartridge

	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Failed to open ROM file: %v", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		log.Fatalf("Failed to get ROM file info: %v", err)
	}

	size := info.Size()
	if size < HEADER_SIZE {
		log.Fatal("ROM file is too small to be valid")
	}

	cart.Data = make([]byte, size)

	buffer := bufio.NewReader(file)
	_, err = buffer.Read(cart.Data)
	if err != nil {
		log.Fatalf("Failed to read ROM data: %v", err)
	}

	LoadHeader(&cart.Header, cart.Data)

	// Store original PRG/CHR data before any mapper modifications
	cart.OriginalPRG, cart.OriginalCHR = LoadPRGCHR(&cart)

	// Print PRG ROM details before mapping
	fmt.Printf("Original PRG ROM size: %d bytes\n", len(cart.OriginalPRG))
	if len(cart.OriginalPRG) >= 0x4000 {
		resetVectorLow := cart.OriginalPRG[0x3FFC]
		resetVectorHigh := cart.OriginalPRG[0x3FFD]
		fmt.Printf("Original Reset Vector: $%02X%02X\n", resetVectorHigh, resetVectorLow)
	}

	// Load PRG and CHR with proper mapping
	LoadPRG(&cart)
	LoadCHR(&cart)

	// Initialize SRAM if enabled
	if cart.Header.RomType.SRAM {
		cart.SRAM = make([]byte, 8192) // 8KB of SRAM
	}

	// Detect MMC1 board variant based on PRG and CHR sizes
	detectMMC1Variant(&cart)

	// Print mapped PRG ROM details
	fmt.Printf("Mapped PRG ROM size: %d bytes\n", len(cart.PRG))
	if len(cart.PRG) >= 0x4000 {
		resetVectorLow := cart.PRG[0x3FFC]
		resetVectorHigh := cart.PRG[0x3FFD]
		fmt.Printf("Mapped Reset Vector: $%02X%02X\n", resetVectorHigh, resetVectorLow)
	}

	// Verify final PRG ROM state
	verifyPRGROM(&cart)

	return cart
}

func LoadHeader(h *Header, b []byte) {
	fmt.Println("Loading iNES header...")

	copy(h.ID[:], b[0:4])

	if h.ID[0] != 'N' || h.ID[1] != 'E' || h.ID[2] != 'S' || h.ID[3] != 0x1A {
		log.Fatal("Invalid NES ROM: Missing header identifier")
	}

	h.ROM_SIZE = b[4]
	h.VROM_SIZE = b[5]

	fmt.Printf("PRG-ROM size: %d x 16KB = %d bytes (%dKB)\n",
		h.ROM_SIZE,
		int(h.ROM_SIZE)*PRG_BANK_SIZE,
		(int(h.ROM_SIZE)*PRG_BANK_SIZE)/1024)

	fmt.Printf("CHR-ROM size: %d x 8KB = %d bytes (%dKB)\n",
		h.VROM_SIZE,
		int(h.VROM_SIZE)*CHR_BANK_SIZE,
		(int(h.VROM_SIZE)*CHR_BANK_SIZE)/1024)

	h.ROM_TYPE = b[6]
	h.ROM_TYPE2 = b[7]

	copy(h.Reserved[:], b[8:16])
	for i, val := range h.Reserved {
		if val != 0 {
			fmt.Printf("Warning: Reserved byte %d is non-zero: %02X\n", i, val)
		}
	}

	TranslateRomType(h)
}

func TranslateRomType(h *Header) {
	fmt.Printf("Parsing ROM flags: %08b | %08b\n", h.ROM_TYPE, h.ROM_TYPE2)

	lowerMapper := (h.ROM_TYPE & 0xF0) >> 4
	upperMapper := h.ROM_TYPE2 & 0xF0
	h.RomType.Mapper = int(upperMapper | lowerMapper)

	fmt.Printf("Mapper: %d\n", h.RomType.Mapper)

	mirroring := h.ROM_TYPE & 0x01
	h.RomType.HorizontalMirroring = mirroring == 0
	h.RomType.VerticalMirroring = mirroring != 0

	fmt.Printf("Mirroring: %s\n",
		map[bool]string{true: "Vertical", false: "Horizontal"}[h.RomType.VerticalMirroring])

	h.RomType.SRAM = (h.ROM_TYPE & 0x02) != 0
	if h.RomType.SRAM {
		fmt.Println("Battery-backed SRAM enabled")
	}

	h.RomType.Trainer = (h.ROM_TYPE & 0x04) != 0
	if h.RomType.Trainer {
		fmt.Println("512-byte Trainer present")
	}

	h.RomType.FourScreenVRAM = (h.ROM_TYPE & 0x08) != 0
	if h.RomType.FourScreenVRAM {
		fmt.Println("Four-screen VRAM mode enabled")
	}

	// Default to no single-screen mirroring unless set by mapper
	h.RomType.SingleScreenMirroring = false
	h.RomType.SingleScreenBank = 0

	h.RomType.MMC1Variant = "" // Initialize to empty string
}

func LoadPRGCHR(c *Cartridge) ([]byte, []byte) {
	offset := HEADER_SIZE
	if c.Header.RomType.Trainer {
		offset += TRAINER_SIZE
	}

	prgSize := int(c.Header.ROM_SIZE) * PRG_BANK_SIZE
	originalPRG := make([]byte, prgSize)
	copy(originalPRG, c.Data[offset:offset+prgSize])

	offset += prgSize
	chrSize := int(c.Header.VROM_SIZE) * CHR_BANK_SIZE
	var originalCHR []byte
	if chrSize > 0 {
		originalCHR = make([]byte, chrSize)
		copy(originalCHR, c.Data[offset:offset+chrSize])
	} else {
		originalCHR = make([]byte, CHR_BANK_SIZE)
	}

	return originalPRG, originalCHR
}

func LoadPRG(c *Cartridge) {
	c.PRG = make([]byte, 32*1024)

	if c.Header.RomType.Mapper == 1 {
		// Copy first 16KB bank
		copy(c.PRG[0:16384], c.OriginalPRG[0:16384])
		// Copy last 16KB bank
		lastBankOffset := len(c.OriginalPRG) - 16384
		copy(c.PRG[16384:32768], c.OriginalPRG[lastBankOffset:lastBankOffset+16384])
	} else {
		// For NROM (Mapper 0) and others, mirror if necessary
		if len(c.OriginalPRG) == 16*1024 {
			// For 16KB ROMs, mirror the bank
			copy(c.PRG[0:16384], c.OriginalPRG[0:16384])
			copy(c.PRG[16384:32768], c.OriginalPRG[0:16384])
			fmt.Println("16KB PRG ROM mirrored to fill 32KB space")
		} else {
			// For 32KB ROMs, copy directly
			copy(c.PRG[0:32768], c.OriginalPRG[0:32768])
		}
	}
}

func LoadCHR(c *Cartridge) {
	if len(c.OriginalCHR) > 0 {
		c.CHR = make([]byte, len(c.OriginalCHR))
		copy(c.CHR, c.OriginalCHR)
	} else {
		c.CHR = make([]byte, CHR_BANK_SIZE)
	}
}

// detectMMC1Variant detects the specific MMC1 board variant based on PRG and CHR sizes
func detectMMC1Variant(cart *Cartridge) {
	if cart.Header.RomType.Mapper != 1 {
		return // Not MMC1
	}

	prgSizeKB := int(cart.Header.ROM_SIZE) * 16
	chrSizeKB := int(cart.Header.VROM_SIZE) * 8

	switch {
	case prgSizeKB == 32:
		cart.Header.RomType.MMC1Variant = "SEROM" // Or SHROM/SH1ROM
	case prgSizeKB > 512:
		cart.Header.RomType.MMC1Variant = "SXROM"
	case prgSizeKB == 512:
		cart.Header.RomType.MMC1Variant = "SUROM"
	case prgSizeKB <= 256 && chrSizeKB == 8 && cart.Header.RomType.SRAM:
		cart.Header.RomType.MMC1Variant = "SNROM"
	case prgSizeKB <= 256 && chrSizeKB == 8 && !cart.Header.RomType.SRAM:
		cart.Header.RomType.MMC1Variant = "SGROM"
	case prgSizeKB <= 256 && chrSizeKB > 8:
		cart.Header.RomType.MMC1Variant = "SKROM" // Assuming SKROM for >= 16KB CHR
	case prgSizeKB <= 256 && cart.Header.RomType.SRAM && cart.Header.VROM_SIZE == 0:
		if cart.Header.ROM_SIZE * 16 == 128 {
			cart.Header.RomType.MMC1Variant = "SOROM"
		} else {
			cart.Header.RomType.MMC1Variant = "SXROM"
		}
	case prgSizeKB <= 256 && chrSizeKB >= 16 && chrSizeKB <= 64 && cart.Header.RomType.SRAM:
		cart.Header.RomType.MMC1Variant = "SZROM"
	default:
		cart.Header.RomType.MMC1Variant = "UNKNOWN" // Indicate unknown MMC1 variant
	}

	fmt.Printf("Detected MMC1 Variant: %s\n", cart.Header.RomType.MMC1Variant)
}