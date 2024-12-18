package cartridge

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"zerojnt/mapper"
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
	Data        []byte        // Raw ROM data
	PRG         []byte        // Program ROM (may be mapped by the mapper)
	CHR         []byte        // Character ROM/RAM (may be mapped by the mapper)
	SRAM        []byte        // Save RAM if enabled
	OriginalPRG []byte        // Original PRG ROM data (for mapper bank switching)
	OriginalCHR []byte        // Original CHR ROM data (for mapper bank switching)
	Mapper      mapper.Mapper // The mapper used by this cartridge
}

// MapperAccessor Interface Implementations

func (c *Cartridge) GetHeader() mapper.HeaderInfo {
	return mapper.HeaderInfo{
		ROM_SIZE:              c.Header.ROM_SIZE,
		VROM_SIZE:             c.Header.VROM_SIZE,
		Mapper:                c.Header.RomType.Mapper,
		VerticalMirroring:     c.Header.RomType.VerticalMirroring,
		HorizontalMirroring:   c.Header.RomType.HorizontalMirroring,
		SRAM:                  c.Header.RomType.SRAM,
		Trainer:               c.Header.RomType.Trainer,
		FourScreenVRAM:        c.Header.RomType.FourScreenVRAM,
		SingleScreenMirroring: c.Header.RomType.SingleScreenMirroring,
		SingleScreenBank:      c.Header.RomType.SingleScreenBank,
		MMC1Variant:           c.Header.RomType.MMC1Variant,
	}
}

func (c *Cartridge) GetPRG() []byte {
	return c.OriginalPRG
}

func (c *Cartridge) GetCHR() []byte {
	return c.OriginalCHR
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

func (c *Cartridge) SetPRGRAMSize(size uint32) {
	if len(c.SRAM) != int(size) {
		c.SRAM = make([]byte, size)
	}
}

func (c *Cartridge) GetCHRRAMSize() uint32 {
	return CHR_BANK_SIZE // CHR RAM is typically 8KB for MMC1 and other mappers
}

func (c *Cartridge) SetCHRRAMSize(size uint32) {
	if len(c.CHR) != int(size) {
		c.CHR = make([]byte, size)
	}
}

func (c *Cartridge) CopyPRGData(destOffset, srcOffset, length uint32) {
	if srcOffset+length > uint32(len(c.OriginalPRG)) || destOffset+length > uint32(len(c.PRG)) {
		log.Printf("Error: CopyPRGData out of bounds - srcOffset: %d, length: %d, OriginalPRG size: %d, destOffset: %d, PRG size: %d",
			srcOffset, length, len(c.OriginalPRG), destOffset, len(c.PRG))
		return
	}
	copy(c.PRG[destOffset:], c.OriginalPRG[srcOffset:srcOffset+length])
}

func (c *Cartridge) CopyCHRData(destOffset, srcOffset, length uint32) {
	// Only copy if CHR ROM exists, otherwise CHR RAM is used and no copy is needed.
	if len(c.OriginalCHR) > 0 {
		if srcOffset+length > uint32(len(c.OriginalCHR)) || destOffset+length > uint32(len(c.CHR)) {
			log.Printf("Error: CopyCHRData out of bounds: srcOffset: %d, length: %d, OriginalCHR size: %d, destOffset: %d, CHR size: %d",
				srcOffset, length, len(c.OriginalCHR), destOffset, len(c.CHR))
			return
		}
		copy(c.CHR[destOffset:], c.OriginalCHR[srcOffset:srcOffset+length])
	}
}

func (c *Cartridge) HasVerticalMirroring() bool {
	return c.Header.RomType.VerticalMirroring
}

func (c *Cartridge) HasHorizontalMirroring() bool {
	return c.Header.RomType.HorizontalMirroring
}

func (c *Cartridge) HasFourScreenVRAM() bool {
	return c.Header.RomType.FourScreenVRAM
}

func (c *Cartridge) HasSRAM() bool {
	return c.Header.RomType.SRAM
}

func (c *Cartridge) GetSingleScreenBank() byte {
	return c.Header.RomType.SingleScreenBank
}

func (c *Cartridge) GetMirroringMode() byte {
	var mode byte
	switch {
	case c.Header.RomType.VerticalMirroring:
		mode = mapper.MMC1_MIRROR_VERTICAL
	case c.Header.RomType.HorizontalMirroring:
		mode = mapper.MMC1_MIRROR_HORIZONTAL
	case c.Header.RomType.SingleScreenMirroring:
		mode = mapper.MMC1_MIRROR_SINGLE_LOWER
		if c.Header.RomType.SingleScreenBank == 1 {
			mode = mapper.MMC1_MIRROR_SINGLE_UPPER
		}
	case c.Header.RomType.FourScreenVRAM:
		mode = 0x04 // Four-screen mirroring
	default:
		mode = 0xFF // Indicate an invalid or unknown mirroring mode
	}
	return mode
}

func (c *Cartridge) SetMirroringMode(vertical, horizontal, fourScreen bool, singleScreenBank byte) {
	c.Header.RomType.VerticalMirroring = vertical
	c.Header.RomType.HorizontalMirroring = horizontal
	c.Header.RomType.FourScreenVRAM = fourScreen
	c.Header.RomType.SingleScreenMirroring = !vertical && !horizontal && !fourScreen
	c.Header.RomType.SingleScreenBank = singleScreenBank
}

func (c *Cartridge) GetMapperNumber() int {
	return c.Header.RomType.Mapper
}

type RomType struct {
	Mapper                int
	HorizontalMirroring   bool
	VerticalMirroring     bool
	SRAM                  bool
	Trainer               bool
	FourScreenVRAM        bool
	SingleScreenMirroring bool
	SingleScreenBank      byte
	MMC1Variant           string
}

func verifyPRGROM(cart *Cartridge) {
	fmt.Println("Verifying PRG ROM...")
	expectedSize := int(cart.Header.ROM_SIZE) * PRG_BANK_SIZE
	if len(cart.PRG) != 32768 || len(cart.OriginalPRG) != expectedSize {
		fmt.Printf("WARNING: PRG ROM size mismatch! Expected: %d, Got: %d (Original: %d)\n",
			expectedSize, len(cart.PRG), len(cart.OriginalPRG))
	}

	if len(cart.OriginalPRG) >= 0x4000 {
		fmt.Printf("First PRG bank begins with: %02X %02X %02X %02X\n",
			cart.OriginalPRG[0], cart.OriginalPRG[1],
			cart.OriginalPRG[2], cart.OriginalPRG[3])

		resetVectorLow := cart.OriginalPRG[0x3FFC]
		resetVectorHigh := cart.OriginalPRG[0x3FFD]
		fmt.Printf("Original Reset Vector: $%02X%02X\n", resetVectorHigh, resetVectorLow)

		lastOffset := len(cart.OriginalPRG) - 0x4000
		fmt.Printf("Last PRG bank begins with: %02X %02X %02X %02X\n",
			cart.OriginalPRG[lastOffset],
			cart.OriginalPRG[lastOffset+1],
			cart.OriginalPRG[lastOffset+2],
			cart.OriginalPRG[lastOffset+3])
	}

	if len(cart.PRG) >= 0x4000 {
		fmt.Printf("Mapped PRG bank begins with: %02X %02X %02X %02X\n",
			cart.PRG[0], cart.PRG[1],
			cart.PRG[2], cart.PRG[3])

		resetVectorLow := cart.PRG[0x3FFC]
		resetVectorHigh := cart.PRG[0x3FFD]
		fmt.Printf("Mapped Reset Vector: $%02X%02X\n", resetVectorHigh, resetVectorLow)

		lastOffset := len(cart.PRG) - 0x4000
		fmt.Printf("Last mapped PRG bank begins with: %02X %02X %02X %02X\n",
			cart.PRG[lastOffset],
			cart.PRG[lastOffset+1],
			cart.PRG[lastOffset+2],
			cart.PRG[lastOffset+3])
	}
}

func LoadRom(filename string) Cartridge {
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
		log.Fatalf("ROM file is too small to be valid. Size: %d", size)
	}

	cart.Data = make([]byte, size)

	buffer := bufio.NewReader(file)
	_, err = buffer.Read(cart.Data)
	if err != nil {
		log.Fatalf("Failed to read ROM data: %v", err)
	}

	LoadHeader(&cart.Header, cart.Data)
	cart.OriginalPRG, cart.OriginalCHR = LoadPRGCHR(&cart)

	// Initialize mapper
	switch cart.Header.RomType.Mapper {
	case 0:
		cart.Mapper = &mapper.NROM{}
	case 1:
		cart.Mapper = &mapper.MMC1{}
	default:
		log.Fatalf("Unsupported mapper: %d", cart.Header.RomType.Mapper)
	}
	cart.Mapper.Initialize(&cart)

	// Allocate SRAM if used
	if cart.Header.RomType.SRAM {
		cart.SRAM = make([]byte, 8192) // 8KB of SRAM
		fmt.Println("SRAM initialized")
	}

	// Detect MMC1 variant
	detectMMC1Variant(&cart)

	// Load PRG and CHR data into the mapper
	LoadPRG(&cart)
	LoadCHR(&cart)

	// Verify PRG ROM after loading
	verifyPRGROM(&cart)

	return cart
}

func LoadHeader(h *Header, b []byte) {
	fmt.Println("Loading iNES header...")
	copy(h.ID[:], b[0:4])

	if string(h.ID[:]) != "NES\x1A" {
		log.Fatal("Invalid NES ROM: Missing or incorrect header identifier")
	}

	h.ROM_SIZE = b[4]
	h.VROM_SIZE = b[5]
	h.ROM_TYPE = b[6]
	h.ROM_TYPE2 = b[7]
	copy(h.Reserved[:], b[8:16])

	fmt.Printf("PRG-ROM size: %d x 16KB = %d bytes (%dKB)\n",
		h.ROM_SIZE, int(h.ROM_SIZE)*PRG_BANK_SIZE, (int(h.ROM_SIZE)*PRG_BANK_SIZE)/1024)
	fmt.Printf("CHR-ROM size: %d x 8KB = %d bytes (%dKB)\n",
		h.VROM_SIZE, int(h.VROM_SIZE)*CHR_BANK_SIZE, (int(h.VROM_SIZE)*CHR_BANK_SIZE)/1024)

	TranslateRomType(h)
}

func TranslateRomType(h *Header) {
	fmt.Printf("Parsing ROM flags: %08b | %08b\n", h.ROM_TYPE, h.ROM_TYPE2)

	h.RomType.Mapper = int((h.ROM_TYPE&0xF0)>>4 | (h.ROM_TYPE2 & 0xF0))
	fmt.Printf("Mapper: %d\n", h.RomType.Mapper)

	h.RomType.HorizontalMirroring = (h.ROM_TYPE & 0x01) == 0
	h.RomType.VerticalMirroring = (h.ROM_TYPE & 0x01) != 0
	fmt.Printf("Mirroring: %s\n", map[bool]string{true: "Vertical", false: "Horizontal"}[h.RomType.VerticalMirroring])

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

	h.RomType.SingleScreenMirroring = false
	h.RomType.SingleScreenBank = 0
	h.RomType.MMC1Variant = ""
}

func LoadPRGCHR(c *Cartridge) ([]byte, []byte) {
	fmt.Println("Loading PRG and CHR data...")
	offset := HEADER_SIZE
	if c.Header.RomType.Trainer {
		fmt.Println("Skipping trainer data")
		offset += TRAINER_SIZE
	}

	prgSize := int(c.Header.ROM_SIZE) * PRG_BANK_SIZE
	originalPRG := make([]byte, prgSize)
	copy(originalPRG, c.Data[offset:offset+prgSize])
	fmt.Printf("PRG data loaded: %d bytes\n", prgSize)

	offset += prgSize
	chrSize := int(c.Header.VROM_SIZE) * CHR_BANK_SIZE
	var originalCHR []byte
	if chrSize > 0 {
		originalCHR = make([]byte, chrSize)
		copy(originalCHR, c.Data[offset:offset+chrSize])
		fmt.Printf("CHR data loaded: %d bytes\n", chrSize)
	} else {
		fmt.Println("No CHR data in ROM, using CHR RAM")
		originalCHR = make([]byte, 0) // No CHR ROM, using CHR RAM
	}

	return originalPRG, originalCHR
}

func LoadPRG(c *Cartridge) {
	fmt.Println("Loading PRG ROM...")
	if c.Mapper == nil {
		log.Fatal("Mapper not initialized")
	}

	// For NROM, we just mirror the PRG ROM as needed
	if _, ok := c.Mapper.(*mapper.NROM); ok {
		c.PRG = make([]byte, 32*1024) // Allocate maximum possible size for NROM
		switch len(c.OriginalPRG) {
		case 16 * 1024: // 16KB PRG ROM
			copy(c.PRG[0:16384], c.OriginalPRG)
			copy(c.PRG[16384:32768], c.OriginalPRG) // Mirror to fill 32KB
			fmt.Println("16KB PRG ROM mirrored to fill 32KB space")
		case 32 * 1024: // 32KB PRG ROM
			copy(c.PRG, c.OriginalPRG)
			fmt.Println("32KB PRG ROM loaded")
		default:
			log.Fatalf("Unsupported PRG ROM size for NROM: %d", len(c.OriginalPRG))
		}
	} else if mapperMMC1, ok := c.Mapper.(*mapper.MMC1); ok {
		c.PRG = make([]byte, 32*1024) // Allocate PRG space for MMC1
		// Bank mapping will be handled by the mapper
		if err := mapperMMC1.UpdateBankMapping(); err != nil {
			log.Fatalf("Failed to initialize MMC1 bank mapping: %v", err)
		}
		fmt.Println("MMC1 PRG ROM bank mapping updated")
	} else {
		log.Fatalf("Unsupported mapper for PRG loading: %T", c.Mapper)
	}
}

func LoadCHR(c *Cartridge) {
    fmt.Println("Loading CHR ROM...")
    if c.Mapper == nil {
        log.Fatal("Mapper not initialized")
    }

    if len(c.OriginalCHR) == 0 {
        // Allocate 8KB for CHR RAM when no CHR ROM is present
        c.CHR = make([]byte, CHR_BANK_SIZE)
        fmt.Println("CHR RAM initialized (no CHR ROM present)")
    } else if _, ok := c.Mapper.(*mapper.NROM); ok {
        // For NROM, copy CHR ROM data directly
        c.CHR = make([]byte, len(c.OriginalCHR))
        copy(c.CHR, c.OriginalCHR)
        fmt.Println("CHR ROM loaded into CHR memory")
    } else if mapperMMC1, ok := c.Mapper.(*mapper.MMC1); ok {
        // MMC1 handles CHR loading during bank mapping
        if len(c.OriginalCHR) > 0 {
            c.CHR = make([]byte, len(c.OriginalCHR))
            copy(c.CHR, c.OriginalCHR)
            fmt.Println("CHR ROM loaded into CHR memory")
        }
        if err := mapperMMC1.UpdateBankMapping(); err != nil {
            log.Fatalf("Failed to initialize MMC1 CHR bank mapping: %v", err)
        }
        fmt.Println("MMC1 CHR ROM bank mapping updated")
    } else {
        log.Fatalf("Unsupported mapper for CHR loading: %T", c.Mapper)
    }
}

func detectMMC1Variant(cart *Cartridge) {
	fmt.Println("Detecting MMC1 variant...")
	if cart.Header.RomType.Mapper != 1 {
		fmt.Println("Not an MMC1 cartridge")
		return
	}

	prgSizeKB := int(cart.Header.ROM_SIZE) * 16
	chrSizeKB := int(cart.Header.VROM_SIZE) * 8

	switch {
	case prgSizeKB == 32:
		cart.Header.RomType.MMC1Variant = "SEROM"
	case prgSizeKB > 512:
		cart.Header.RomType.MMC1Variant = "SXROM"
	case prgSizeKB == 512:
		cart.Header.RomType.MMC1Variant = "SUROM"
	case prgSizeKB <= 256 && chrSizeKB == 8 && cart.Header.RomType.SRAM:
		cart.Header.RomType.MMC1Variant = "SNROM"
	case prgSizeKB <= 256 && chrSizeKB == 8 && !cart.Header.RomType.SRAM:
		cart.Header.RomType.MMC1Variant = "SGROM"
	case prgSizeKB <= 256 && chrSizeKB > 8:
		cart.Header.RomType.MMC1Variant = "SKROM"
	case prgSizeKB <= 256 && cart.Header.RomType.SRAM && cart.Header.VROM_SIZE == 0:
		if cart.Header.ROM_SIZE*16 == 128 {
			cart.Header.RomType.MMC1Variant = "SOROM"
		} else {
			cart.Header.RomType.MMC1Variant = "SXROM"
		}
	case prgSizeKB <= 256 && chrSizeKB >= 16 && chrSizeKB <= 64 && cart.Header.RomType.SRAM:
		cart.Header.RomType.MMC1Variant = "SZROM"
	default:
		cart.Header.RomType.MMC1Variant = "UNKNOWN"
	}

	fmt.Printf("Detected MMC1 variant: %s\n", cart.Header.RomType.MMC1Variant)
}