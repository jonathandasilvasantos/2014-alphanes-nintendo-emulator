// File: ./cartridge/cartridge.go
package cartridge

import (
	"fmt"
	"log"
	"os"
	"zerojnt/mapper"
)

const (
	INES_HEADER_SIZE = 16  // iNES header size
	TRAINER_SIZE     = 512 // Size of trainer if present
	PRG_BANK_SIZE_KB = 16  // 16KB
	CHR_BANK_SIZE_KB = 8   // 8KB
	SRAM_DEFAULT_SIZE = 8192 // Default 8KB SRAM if flag is set
	CHR_RAM_SIZE      = 8192 // Always use 8KB for CHR RAM when needed
	MAPPED_PRG_SIZE   = 32 * 1024 // Mappers like NROM, MMC1 map 32KB PRG window
	MAPPED_CHR_SIZE   = 8 * 1024  // Mappers like NROM, MMC1 map 8KB CHR window
)

// Header represents the parsed iNES header fields.
type Header struct {
	ID        [4]byte // NES<EOF>
	PRGSizeKB byte    // PRG Size in 16KB units
	CHRSizeKB byte    // CHR Size in 8KB units (0 means CHR RAM)
	Flags6    byte    // Mapper low nibble, mirroring, SRAM, trainer
	Flags7    byte    // Mapper high nibble, NES 2.0 flag
	// NES 2.0 fields would go here if supported
	Reserved [8]byte // Unused padding in iNES 1.0

	// Parsed information derived from flags
	MapperNum             int
	VerticalMirroring     bool // Initial state from header
	HorizontalMirroring   bool // Initial state from header
	SRAMEnabled           bool // SRAM $6000-$7FFF potentially present
	TrainerPresent        bool
	FourScreenVRAM        bool
	SingleScreenMirroring bool // Calculated
	SingleScreenBank      byte // 0 or 1 if SingleScreenMirroring is true
	MMC1Variant           string
}

// Cartridge holds all data and state for a loaded NES cartridge.
type Cartridge struct {
	Header Header

	OriginalPRG []byte        // Raw PRG-ROM data from the file
	OriginalCHR []byte        // Raw CHR-ROM data from the file (empty if CHR RAM)

	PRG []byte // 32KB window mapped for CPU ($8000-$FFFF) - Mappers write banks here
	CHR []byte // 8KB window mapped for PPU ($0000-$1FFF) - Mappers write banks here (or PPU writes directly if CHR RAM)
	SRAM []byte // PRG RAM ($6000-$7FFF) - Size varies, typically 8KB or 32KB for MMC1

	Mapper mapper.Mapper // The mapper implementation

	// Runtime state derived/controlled by mapper
	currentVerticalMirroring     bool
	currentHorizontalMirroring   bool
	currentFourScreenVRAM        bool
	currentSingleScreenMirroring bool
	currentSingleScreenBank      byte

	SRAMDirty bool // Flag for saving battery-backed RAM

	// No explicit IRQLine needed here, CPU checks Mapper.IRQState() via accessor
}

// --- Cartridge Loading ---

// LoadRom loads a .nes file, parses it, initializes memory and the mapper.
func LoadRom(filename string) (*Cartridge, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open ROM file '%s': %w", filename, err)
	}
	defer file.Close()

	// Read header
	headerBytes := make([]byte, INES_HEADER_SIZE)
	_, err = file.Read(headerBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read ROM header: %w", err)
	}

	cart := &Cartridge{}
	if err := parseHeader(&cart.Header, headerBytes); err != nil {
		return nil, fmt.Errorf("invalid NES header: %w", err)
	}

	// Read remaining data (Trainer, PRG, CHR)
	// Get file size to read the rest efficiently
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	remainingSize := fileInfo.Size() - INES_HEADER_SIZE
	if remainingSize <= 0 {
		return nil, fmt.Errorf("ROM file contains only a header")
	}

	allData := make([]byte, remainingSize)
	_, err = file.ReadAt(allData, INES_HEADER_SIZE) // Read starting after the header
	if err != nil {
		return nil, fmt.Errorf("failed to read ROM data: %w", err)
	}

	// Extract Trainer, PRG, CHR from the read data
	offset := 0
	if cart.Header.TrainerPresent {
		if len(allData) < TRAINER_SIZE {
			return nil, fmt.Errorf("ROM header indicates trainer, but data is too short")
		}
		// Trainer data is usually ignored by emulators
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
		cart.OriginalCHR = make([]byte, 0) // Explicitly empty
		log.Println("Cartridge uses CHR RAM")
	}

	// --- Initialize Memory Buffers ---
	cart.PRG = make([]byte, MAPPED_PRG_SIZE) // Always 32KB for CPU view
	cart.CHR = make([]byte, MAPPED_CHR_SIZE) // Always 8KB for PPU view (used for ROM banks or direct RAM access)

	// Allocate CHR RAM if needed (must happen before mapper init)
	if cart.Header.CHRSizeKB == 0 {
		cart.CHR = make([]byte, CHR_RAM_SIZE) // Allocate 8KB for CHR RAM
		log.Printf("Allocated %d KB CHR RAM", CHR_RAM_SIZE/1024)
	}

	if cart.Header.SRAMEnabled {
		// TODO: Determine SRAM size more accurately if needed (e.g., from NES 2.0 header or common mapper sizes)
		// MMC3 often uses 8KB.
		sramSize := SRAM_DEFAULT_SIZE
		cart.SRAM = make([]byte, sramSize)
		log.Printf("Initialized %d KB SRAM", sramSize/1024)
		// TODO: Load battery-backed SRAM from file if implemented
	}

	// Detect MMC1 variant *after* loading sizes, *before* initializing mapper
	if cart.Header.MapperNum == 1 {
		detectMMC1Variant(cart) // Updates cart.Header.MMC1Variant
	}

	// --- Initialize Mapper ---
	switch cart.Header.MapperNum {
	case 0:
		cart.Mapper = &mapper.NROM{}
	case 1:
		cart.Mapper = &mapper.MMC1{}
	case 2: // Add UNROM support
		cart.Mapper = &mapper.UNROM{}
	case 4: // *** ADDED MMC3 CASE ***
		cart.Mapper = &mapper.MMC3{}
	
		// Add other mappers here
	default:
		return nil, fmt.Errorf("unsupported mapper number: %d", cart.Header.MapperNum)
	}

	if cart.Mapper == nil {
		return nil, fmt.Errorf("failed to instantiate mapper %d", cart.Header.MapperNum)
	}

	cart.Mapper.Initialize(cart) // Pass the cartridge itself as the accessor
	cart.Mapper.Reset()          // Perform initial bank/mirroring setup

	log.Printf("Cartridge loaded successfully. Mapper: %d (%T)", cart.Header.MapperNum, cart.Mapper)
	verifyPRGROM(cart) // Optional debug verification

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
	copy(h.Reserved[:], b[8:16]) // Should be zero for iNES 1.0

	if h.PRGSizeKB == 0 {
		return fmt.Errorf("invalid header: PRG ROM size cannot be 0")
	}

	// --- Decode Flags ---
	h.MapperNum = int((h.Flags6 >> 4) | (h.Flags7 & 0xF0))

	h.VerticalMirroring = (h.Flags6 & 0x01) != 0
	h.HorizontalMirroring = !h.VerticalMirroring // Default assumption if not four-screen

	h.SRAMEnabled = (h.Flags6 & 0x02) != 0
	h.TrainerPresent = (h.Flags6 & 0x04) != 0
	h.FourScreenVRAM = (h.Flags6 & 0x08) != 0

	// Correct mirroring based on FourScreenVRAM
	if h.FourScreenVRAM {
		h.VerticalMirroring = false
		h.HorizontalMirroring = false
		h.SingleScreenMirroring = false
	} else {
		// Determine if single screen mirroring is implied (uncommon, usually set by mapper)
		// Header flag 6 bit 0 determines Vertical (1) or Horizontal (0)
		// Mappers like MMC1/MMC3 override this. We store the header default here.
		h.SingleScreenMirroring = false // Assume not single screen initially
		h.SingleScreenBank = 0
	}

	// Check for NES 2.0 identifier (partially)
	if (h.Flags7 & 0x0C) == 0x08 {
		log.Println("NES 2.0 format detected (limited support)")
		// Add NES 2.0 parsing here if needed
	}

	log.Printf("Header Parsed: PRG:%dKB CHR:%dKB Map:%d VMir:%v SRAM:%v Trn:%v 4Scr:%v",
		int(h.PRGSizeKB)*PRG_BANK_SIZE_KB,
		int(h.CHRSizeKB)*CHR_BANK_SIZE_KB,
		h.MapperNum,
		h.VerticalMirroring, // Initial vertical state
		h.SRAMEnabled,
		h.TrainerPresent,
		h.FourScreenVRAM)

	return nil
}

// detectMMC1Variant attempts to identify the specific MMC1 board based on ROM/RAM sizes.
// This updates the Header.MMC1Variant field.
func detectMMC1Variant(cart *Cartridge) {
	if cart.Header.MapperNum != 1 {
		return // Only for MMC1
	}

	prgSizeKB := int(cart.Header.PRGSizeKB) * PRG_BANK_SIZE_KB
	chrSizeKB := int(cart.Header.CHRSizeKB) * CHR_BANK_SIZE_KB
	hasChrRAM := cart.Header.CHRSizeKB == 0
	hasSRAM := cart.Header.SRAMEnabled

	// Based on https://wiki.nesdev.org/w/index.php/MMC1
	variant := "UNKNOWN"
	switch {
	// Specific Size Matches
	case prgSizeKB == 128 && hasChrRAM && hasSRAM: // SOROM (often 128KB PRG, CHR RAM, SRAM)
		variant = "SOROM" // Example: Final Fantasy 1/2

	// General Rules (can overlap, specific matches take precedence)
	case prgSizeKB >= 512 && !hasChrRAM: // SUROM (512KB PRG, 8KB CHR RAM/ROM, 8KB SRAM), SXROM (>512KB)
		if prgSizeKB == 512 {
			variant = "SUROM" // Example: Final Fantasy III (J)
		} else {
			variant = "SXROM" // Example: Mechanized Attack
		}
	case prgSizeKB <= 256 && chrSizeKB == 8 && hasSRAM: // SNROM (<=256KB PRG, 8KB CHR RAM, 8KB SRAM)
		variant = "SNROM" // Example: Zelda 1/2, Metroid
	case prgSizeKB <= 256 && chrSizeKB == 8 && !hasSRAM: // SGROM (<=256KB PRG, 8KB CHR RAM, no SRAM)
		variant = "SGROM" // Example: Mega Man 2, Bionic Commando
	case prgSizeKB <= 256 && chrSizeKB >= 16 && !hasChrRAM: // SKROM (<=256KB PRG, >=16KB CHR ROM, no SRAM)
		variant = "SKROM" // Example: SMB3 (uses 32KB CHR), Kirby's Adventure
	case prgSizeKB <= 256 && chrSizeKB >= 16 && hasSRAM && !hasChrRAM: // SZROM? Maybe overlaps SKROM with SRAM
		variant = "SZROM" // Example: Crystalis
	// Fallback for common cases if specific sizes didn't match
	case hasChrRAM && hasSRAM:
		variant = "SNROM/SOROM/SUROM/SXROM" // Could be any with CHR RAM + SRAM
	case !hasChrRAM && hasSRAM:
		variant = "SKROM/SZROM" // Could be CHR ROM + SRAM
	case hasChrRAM && !hasSRAM:
		variant = "SGROM" // Likely SGROM
	case !hasChrRAM && !hasSRAM:
		variant = "SKROM" // Likely SKROM

	}

	cart.Header.MMC1Variant = variant
	log.Printf("Detected MMC1 variant: %s (PRG:%dKB CHR:%dKB CHR-RAM:%v SRAM:%v)",
		variant, prgSizeKB, chrSizeKB, hasChrRAM, hasSRAM)
}

// --- MapperAccessor Interface Implementations ---

func (c *Cartridge) GetHeader() mapper.HeaderInfo {
	// Return a copy of relevant parsed info for the mapper
	return mapper.HeaderInfo{
		ROM_SIZE:              c.Header.PRGSizeKB,
		VROM_SIZE:             c.Header.CHRSizeKB,
		Mapper:                c.Header.MapperNum,
		VerticalMirroring:     c.Header.VerticalMirroring, // Initial state
		HorizontalMirroring:   c.Header.HorizontalMirroring, // Initial state
		SRAM:                  c.Header.SRAMEnabled,
		Trainer:               c.Header.TrainerPresent,
		FourScreenVRAM:        c.Header.FourScreenVRAM,
		SingleScreenMirroring: c.Header.SingleScreenMirroring, // Initial state
		SingleScreenBank:      c.Header.SingleScreenBank,     // Initial state
		MMC1Variant:           c.Header.MMC1Variant,
	}
}

func (c *Cartridge) GetPRGSize() uint32 {
	return uint32(len(c.OriginalPRG))
}

func (c *Cartridge) GetCHRSize() uint32 {
	// Return size of CHR ROM. If CHR RAM is used, this is 0.
	return uint32(len(c.OriginalCHR))
}

func (c *Cartridge) GetPRGRAMSize() uint32 {
	return uint32(len(c.SRAM))
}

// WriteSRAM writes to the cartridge's SRAM, handling potential banking if necessary.
// For standard 8KB SRAM, offset is direct. For banked (like SUROM 32KB), this might need logic.
func (c *Cartridge) WriteSRAM(offset uint16, value byte) {
	if c.Header.SRAMEnabled && int(offset) < len(c.SRAM) {
		// Basic write for now. If implementing banking (e.g. 32KB SUROM SRAM),
		// the offset might need translation based on mapper state, which adds complexity.
		// Let's assume direct offset for now, matching the mapper's calculation.
		if c.SRAM[offset] != value {
			c.SRAM[offset] = value
			c.SRAMDirty = true // Flag for saving later
		}
	}
	// Ignore writes if SRAM disabled or out of bounds
}

func (c *Cartridge) GetCHRRAMSize() uint32 {
	// Return the size of the CHR buffer IF it's being used as RAM.
	if c.Header.CHRSizeKB == 0 {
		return uint32(len(c.CHR)) // Should be CHR_RAM_SIZE (8KB)
	}
	return 0 // Not using CHR RAM
}

// CopyPRGData copies requested bank from OriginalPRG to the mapped PRG window.
func (c *Cartridge) CopyPRGData(destOffset uint32, srcOffset uint32, length uint32) {
	// Check source bounds (OriginalPRG)
	if len(c.OriginalPRG) == 0 {
		// log.Printf("DEBUG: CopyPRGData skipped - OriginalPRG size is 0.")
		return
	}
	if srcOffset+length > uint32(len(c.OriginalPRG)) {
		log.Printf("ERROR: CopyPRGData source out of bounds - srcOffset: %X, length: %X, OriginalPRG size: %X",
			srcOffset, length, len(c.OriginalPRG))
		// Optionally fill destination with a pattern? Or just return?
		// Adjust length if it exceeds source boundary partially
		if srcOffset >= uint32(len(c.OriginalPRG)) {
			return // Cannot copy anything
		}
		length = uint32(len(c.OriginalPRG)) - srcOffset
		if length == 0 { return } // Should not happen if srcOffset check passed
	}

	// Check destination bounds (Mapped PRG window)
	if destOffset+length > uint32(len(c.PRG)) {
		log.Printf("ERROR: CopyPRGData destination out of bounds - destOffset: %X, length: %X, PRG size: %X",
			destOffset, length, len(c.PRG))
		// Adjust length if it exceeds destination boundary partially
		if destOffset >= uint32(len(c.PRG)) {
			return // Cannot copy anything
		}
		length = uint32(len(c.PRG)) - destOffset
		if length == 0 { return } // Should not happen if destOffset check passed
	}

	// Perform the copy
	copy(c.PRG[destOffset:destOffset+length], c.OriginalPRG[srcOffset:srcOffset+length])
}

// CopyCHRData copies requested bank from OriginalCHR to the mapped CHR window.
func (c *Cartridge) CopyCHRData(destOffset uint32, srcOffset uint32, length uint32) {
	// Cannot copy if using CHR RAM
	if c.Header.CHRSizeKB == 0 {
		// log.Printf("DEBUG: CopyCHRData skipped - CHR RAM in use.")
		return
	}

	// Check source bounds (OriginalCHR)
	if len(c.OriginalCHR) == 0 {
		// log.Printf("DEBUG: CopyCHRData skipped - OriginalCHR size is 0.")
		return
	}
	if srcOffset+length > uint32(len(c.OriginalCHR)) {
		log.Printf("ERROR: CopyCHRData source out of bounds - srcOffset: %X, length: %X, OriginalCHR size: %X",
			srcOffset, length, len(c.OriginalCHR))
		if srcOffset >= uint32(len(c.OriginalCHR)) { return }
		length = uint32(len(c.OriginalCHR)) - srcOffset
		if length == 0 { return }
	}

	// Check destination bounds (Mapped CHR window)
	if destOffset+length > uint32(len(c.CHR)) {
		log.Printf("ERROR: CopyCHRData destination out of bounds - destOffset: %X, length: %X, CHR size: %X",
			destOffset, length, len(c.CHR))
		if destOffset >= uint32(len(c.CHR)) { return }
		length = uint32(len(c.CHR)) - destOffset
		if length == 0 { return }
	}

	// Perform the copy
	copy(c.CHR[destOffset:destOffset+length], c.OriginalCHR[srcOffset:srcOffset+length])
}

func (c *Cartridge) HasSRAM() bool {
	return c.Header.SRAMEnabled
}

func (c *Cartridge) HasFourScreenVRAM() bool {
	return c.Header.FourScreenVRAM
}

// SetMirroringMode updates the cartridge's *current* mirroring state based on mapper control.
func (c *Cartridge) SetMirroringMode(vertical, horizontal, fourScreen bool, singleScreenBank byte) {
	// The mapper tells us the effective mode now.
	c.currentVerticalMirroring = vertical
	c.currentHorizontalMirroring = horizontal
	c.currentFourScreenVRAM = fourScreen // Usually dictated by header, but mapper might confirm
	c.currentSingleScreenMirroring = !vertical && !horizontal && !fourScreen
	c.currentSingleScreenBank = singleScreenBank

	// Log the change if desired
	// modeStr := "Unknown"
	// if vertical { modeStr = "Vertical" }
	// if horizontal { modeStr = "Horizontal" }
	// if fourScreen { modeStr = "FourScreen" }
	// if c.currentSingleScreenMirroring { modeStr = fmt.Sprintf("SingleScreen Bank %d", singleScreenBank)}
	// log.Printf("Cartridge Mirroring Set: %s", modeStr)
}

// GetCurrentMirroringType returns the current mirroring mode for the PPU.
// This is what the PPU should query.
func (c *Cartridge) GetCurrentMirroringType() (v, h, four, single bool, bank byte) {
	return c.currentVerticalMirroring, c.currentHorizontalMirroring, c.currentFourScreenVRAM, c.currentSingleScreenMirroring, c.currentSingleScreenBank
}

// --- IRQ Handling for MapperAccessor ---

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


// verifyPRGROM - Optional debug helper
func verifyPRGROM(cart *Cartridge) {
	fmt.Println("--- PRG ROM Verification ---")
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

	// Check Reset Vector in the *last* bank of Original PRG
	if len(cart.OriginalPRG) >= 0x4000 {
		lastBankOffset := len(cart.OriginalPRG) - 0x4000
		resetVectorLow := cart.OriginalPRG[lastBankOffset+0x3FFC]
		resetVectorHigh := cart.OriginalPRG[lastBankOffset+0x3FFD]
		fmt.Printf("Original Reset Vector (@%06X): $%02X%02X\n", lastBankOffset+0x3FFC, resetVectorHigh, resetVectorLow)
	}

	// Check Reset Vector in the *mapped* PRG window (should reflect the currently mapped last bank)
	if len(cart.PRG) == MAPPED_PRG_SIZE {
		// Correct index: $FFFC relative to $8000 start is index 0x7FFC ($FFFF - $8000 + 1 = $8000 -> index 0x7FFF)
		// Indices are $8000 -> 0, $8001 -> 1, ..., $FFFC -> 0x7FFC, $FFFD -> 0x7FFD
		resetVectorLow := cart.PRG[0x7FFC]  // <-- FIXED: Was 'c.PRG', now 'cart.PRG'
		resetVectorHigh := cart.PRG[0x7FFD] // <-- FIXED: Was 'c.PRG', now 'cart.PRG'
		fmt.Printf("Mapped Reset Vector (@$FFFC): $%02X%02X\n", resetVectorHigh, resetVectorLow)
	}
	fmt.Println("----------------------------")
}