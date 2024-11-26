package cartridge

import (
    "fmt"
    "os"
    "log"
    "bufio"
)

const (
    PRG_BANK_SIZE = 16384 // 16KB
    CHR_BANK_SIZE = 8192  // 8KB
    HEADER_SIZE   = 16    // iNES header size
    TRAINER_SIZE  = 512   // Size of trainer if present
)

type Header struct {
    ID [4]byte        // NES<EOF>
    ROM_SIZE byte     // PRG Size in 16KB units
    VROM_SIZE byte    // CHR Size in 8KB units
    ROM_TYPE byte     // Flags 6
    ROM_TYPE2 byte    // Flags 7
    ROM_BLANK [8]byte // Reserved space in header
    RomType RomType   // Parsed ROM type information
}

type Cartridge struct {
    Header Header
    Data []byte     // Raw ROM data
    PRG []byte      // Program ROM
    CHR []byte      // Character ROM/RAM
    SRAM []byte     // Save RAM if enabled
    OriginalPRG []byte // Keep original PRG for MMC1 bank switching
    OriginalCHR []byte // Keep original CHR for MMC1 bank switching
}

type RomType struct {
    Mapper int
    HorizontalMirroring bool
    VerticalMirroring bool
    SRAM bool
    Trainer bool
    FourScreenVRAM bool
}

func verifyPRGROM(cart *Cartridge) {
    // Verify PRG ROM size
    expectedSize := int(cart.Header.ROM_SIZE) * PRG_BANK_SIZE
    if len(cart.PRG) != 32768 || len(cart.OriginalPRG) != expectedSize {
        fmt.Printf("WARNING: PRG ROM size mismatch! Expected: %d, Got: %d (Original: %d)\n",
            expectedSize, len(cart.PRG), len(cart.OriginalPRG))
    }
    
    // Verify initial content
    if len(cart.OriginalPRG) >= 16384 {
        fmt.Printf("First PRG bank begins with: %02X %02X %02X %02X\n",
            cart.OriginalPRG[0], cart.OriginalPRG[1],
            cart.OriginalPRG[2], cart.OriginalPRG[3])
        
        lastOffset := len(cart.OriginalPRG) - 16384
        fmt.Printf("Last PRG bank begins with: %02X %02X %02X %02X\n",
            cart.OriginalPRG[lastOffset],
            cart.OriginalPRG[lastOffset+1],
            cart.OriginalPRG[lastOffset+2],
            cart.OriginalPRG[lastOffset+3])
    }
}

func LoadRom(Filename string) Cartridge {
    fmt.Println("Loading ROM:", Filename)
    
    var cart Cartridge
    
    file, err := os.Open(Filename)
    if err != nil {
        log.Fatalf("Failed to open ROM file: %v", err)
    }
    defer file.Close()
    
    info, err := file.Stat()
    if err != nil {
        log.Fatalf("Failed to get ROM file info: %v", err)
    }
    
    var size int64 = info.Size()
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
    LoadPRG(&cart)
    LoadCHR(&cart)
    
    // Initialize SRAM if enabled
    if cart.Header.RomType.SRAM {
        cart.SRAM = make([]byte, 8192) // 8KB of SRAM
    }

    // Verify PRG ROM loading
    verifyPRGROM(&cart)
    
    // Important: Store original data before any bank switching
    if len(cart.PRG) > 0 {
        cart.OriginalPRG = make([]byte, int(cart.Header.ROM_SIZE)*PRG_BANK_SIZE)
        copy(cart.OriginalPRG, cart.Data[HEADER_SIZE:HEADER_SIZE+len(cart.OriginalPRG)])
    }
    
    if len(cart.CHR) > 0 {
        cart.OriginalCHR = make([]byte, len(cart.CHR))
        copy(cart.OriginalCHR, cart.CHR)
    } else {
        // Create CHR-RAM if no CHR-ROM present
        cart.CHR = make([]byte, CHR_BANK_SIZE)
        cart.OriginalCHR = make([]byte, CHR_BANK_SIZE)
    }

    return cart
}

func LoadHeader(h *Header, b []byte) {
    fmt.Println("Loading iNES header...")
    
    // Load NES<EOF> identifier
    for i := 0; i < 4; i++ {
        h.ID[i] = b[i]
    }
    
    // Validate header
    if h.ID[0] != 'N' || h.ID[1] != 'E' || h.ID[2] != 'S' || h.ID[3] != 0x1A {
        log.Fatal("Invalid NES ROM: Missing header identifier")
    }

    // Load sizes
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
    
    // Load control bytes
    h.ROM_TYPE = b[6]
    h.ROM_TYPE2 = b[7]
    
    // Load reserved bytes
    for i := 0; i < 8; i++ {
        h.ROM_BLANK[i] = b[8+i]
        if h.ROM_BLANK[i] != 0 {
            fmt.Printf("Warning: Reserved byte %d is non-zero: %02X\n", i, h.ROM_BLANK[i])
        }
    }
    
    TranslateRomType(h)
}

func TranslateRomType(h *Header) {
    fmt.Printf("Parsing ROM flags: %08b | %08b\n", h.ROM_TYPE, h.ROM_TYPE2)
    
    // Extract mapper number (both lower and upper bits)
    lowerMapper := (h.ROM_TYPE & 0xF0) >> 4
    upperMapper := h.ROM_TYPE2 & 0xF0
    h.RomType.Mapper = int(upperMapper | lowerMapper)
    
    fmt.Printf("Mapper: %d\n", h.RomType.Mapper)
    
    // Parse mirroring type
    mirroring := h.ROM_TYPE & 0x01
    h.RomType.HorizontalMirroring = mirroring == 0
    h.RomType.VerticalMirroring = mirroring != 0
    
    fmt.Printf("Mirroring: %s\n", 
        map[bool]string{true: "Vertical", false: "Horizontal"}[h.RomType.VerticalMirroring])
    
    // Parse battery-backed RAM
    h.RomType.SRAM = (h.ROM_TYPE & 0x02) != 0
    if h.RomType.SRAM {
        fmt.Println("Battery-backed SRAM enabled")
    }
    
    // Parse trainer presence
    h.RomType.Trainer = (h.ROM_TYPE & 0x04) != 0
    if h.RomType.Trainer {
        fmt.Println("512-byte Trainer present")
    }
    
    // Parse four-screen VRAM
    h.RomType.FourScreenVRAM = (h.ROM_TYPE & 0x08) != 0
    if h.RomType.FourScreenVRAM {
        fmt.Println("Four-screen VRAM mode enabled")
    }
}

func LoadPRG(c *Cartridge) {
    offset := HEADER_SIZE
    if c.Header.RomType.Trainer {
        offset += TRAINER_SIZE
    }
    
    size := int(c.Header.ROM_SIZE) * PRG_BANK_SIZE
    if size == 0 {
        log.Fatal("ROM contains no PRG-ROM data")
    }
    
    // Critical: For Mapper 1, always allocate 32KB for PRG
    c.PRG = make([]byte, 32768) // Fixed at 32KB to support mapped banks
    
    // Load complete original data
    c.OriginalPRG = make([]byte, size)
    for i := 0; i < size && (i+offset) < len(c.Data); i++ {
        c.OriginalPRG[i] = c.Data[i+offset]
    }
    
    // For MMC1, initially load the first and last banks
    if c.Header.RomType.Mapper == 1 {
        // Copy first 16KB bank
        copy(c.PRG[0:16384], c.OriginalPRG[0:16384])
        // Copy last 16KB bank
        lastBankOffset := size - 16384
        copy(c.PRG[16384:32768], c.OriginalPRG[lastBankOffset:lastBankOffset+16384])
    } else {
        // For other mappers, copy first 32KB or total size if smaller
        copySize := size
        if copySize > 32768 {
            copySize = 32768
        }
        copy(c.PRG[0:copySize], c.OriginalPRG[0:copySize])
    }
}

func LoadCHR(c *Cartridge) {
    offset := HEADER_SIZE
    if c.Header.RomType.Trainer {
        offset += TRAINER_SIZE
    }
    offset += int(c.Header.ROM_SIZE) * PRG_BANK_SIZE
    
    size := int(c.Header.VROM_SIZE) * CHR_BANK_SIZE
    if size > 0 {
        c.CHR = make([]byte, size)
        for i := 0; i < size && (i+offset) < len(c.Data); i++ {
            c.CHR[i] = c.Data[i+offset]
        }
    } else {
        // Create CHR-RAM if no CHR-ROM present
        c.CHR = make([]byte, CHR_BANK_SIZE)
    }
}