package mapper

import (
    "fmt"
    "log"
    "zerojnt/cartridge"
)

const (
    PRG_BANK_SIZE uint32 = 16384 // 16KB
    CHR_BANK_SIZE uint32 = 8192  // 8KB
)

type MMC1State struct {
    ShiftRegister byte
    WriteCount    byte
    Control       byte
    CHRBank0      byte
    CHRBank1      byte
    PRGBank       byte
    PRGMode       byte
    CHRMode       byte
    LastCycle     uint64 // For tracking consecutive writes
}

var mmc1 MMC1State

func init() {
    resetMMC1()
}

func resetMMC1() {
    mmc1.ShiftRegister = 0x10
    mmc1.WriteCount = 0
    mmc1.Control = 0x0C      // Initial control value, sets PRG mode 3
    mmc1.CHRBank0 = 0
    mmc1.CHRBank1 = 0
    mmc1.PRGBank = 0
    mmc1.PRGMode = 3         // Start in 16KB mode, fixed last bank
    mmc1.CHRMode = 0
    mmc1.LastCycle = 0
    
    fmt.Printf("MMC1: Reset state - Control: %02X, PRGMode: %d, PRGBank: %d\n", 
        mmc1.Control, mmc1.PRGMode, mmc1.PRGBank)
}

func updateBankMapping(cart *cartridge.Cartridge) {
    if len(cart.OriginalPRG) == 0 {
        log.Fatal("OriginalPRG is empty - ROM data wasn't properly loaded")
        return
    }

    numBanks := uint32(len(cart.OriginalPRG)) / PRG_BANK_SIZE
    if numBanks == 0 {
        log.Fatal("Invalid PRG ROM size")
        return
    }

    // Ensure PRG space is 32KB
    if len(cart.PRG) != 32768 {
        cart.PRG = make([]byte, 32768)
    }

    switch mmc1.PRGMode {
    case 0, 1: // 32KB mode
        // Ignore least significant bit of bank number in 32KB mode
        bankPair := uint32(mmc1.PRGBank & 0xFE) % numBanks
        srcOffset := bankPair * 32768
        if srcOffset+32768 <= uint32(len(cart.OriginalPRG)) {
            copy(cart.PRG[0:32768], cart.OriginalPRG[srcOffset:srcOffset+32768])
        }

    case 2: // First bank fixed, second switchable
        // First 16KB is always first bank
        copy(cart.PRG[0:16384], cart.OriginalPRG[0:16384])
        
        // Second 16KB is switchable
        bankNum := uint32(mmc1.PRGBank) % numBanks
        srcOffset := bankNum * PRG_BANK_SIZE
        if srcOffset+16384 <= uint32(len(cart.OriginalPRG)) {
            copy(cart.PRG[16384:32768], cart.OriginalPRG[srcOffset:srcOffset+16384])
        }

    case 3: // First bank switchable, last bank fixed
        // Switch first bank
        bankNum := uint32(mmc1.PRGBank) % (numBanks - 1)  // Reserve last bank
        srcOffset := bankNum * PRG_BANK_SIZE
        if srcOffset+16384 <= uint32(len(cart.OriginalPRG)) {
            copy(cart.PRG[0:16384], cart.OriginalPRG[srcOffset:srcOffset+16384])
        }
        
        // Fix last bank
        lastBankOffset := uint32(len(cart.OriginalPRG)) - PRG_BANK_SIZE
        copy(cart.PRG[16384:32768], cart.OriginalPRG[lastBankOffset:lastBankOffset+16384])
    }

    // Handle CHR banking if present
    if len(cart.OriginalCHR) > 0 {
        if mmc1.CHRMode == 0 {
            // 8KB mode
            bank := uint32(mmc1.CHRBank0 & 0x1E) % (uint32(len(cart.OriginalCHR)) / 8192)
            if bank*8192+8192 <= uint32(len(cart.OriginalCHR)) {
                copy(cart.CHR[0:8192], cart.OriginalCHR[bank*8192:(bank+1)*8192])
            }
        } else {
            // 4KB mode
            bank0 := uint32(mmc1.CHRBank0) % (uint32(len(cart.OriginalCHR)) / 4096)
            bank1 := uint32(mmc1.CHRBank1) % (uint32(len(cart.OriginalCHR)) / 4096)
            if bank0*4096+4096 <= uint32(len(cart.OriginalCHR)) {
                copy(cart.CHR[0:4096], cart.OriginalCHR[bank0*4096:(bank0+1)*4096])
            }
            if bank1*4096+4096 <= uint32(len(cart.OriginalCHR)) {
                copy(cart.CHR[4096:8192], cart.OriginalCHR[bank1*4096:(bank1+1)*4096])
            }
        }
    }
}

func MMC1Write(cart *cartridge.Cartridge, addr uint16, value byte) {
    // Reset on bit 7 set
    if (value & 0x80) != 0 {
        mmc1.ShiftRegister = 0x10
        mmc1.WriteCount = 0
        mmc1.Control = mmc1.Control | 0x0C  // Set PRG mode to 3 on reset
        updateBankMapping(cart)
        return
    }

    // Load shift register
    mmc1.ShiftRegister = (mmc1.ShiftRegister >> 1) | ((value & 1) << 4)
    mmc1.WriteCount++

    // Process register write after 5 bits
    if mmc1.WriteCount == 5 {
        regValue := mmc1.ShiftRegister & 0x1F // Ensure 5-bit value
        
        switch {
        case addr <= 0x9FFF: // Control ($8000-$9FFF)
            oldControl := mmc1.Control
            mmc1.Control = regValue
            mmc1.PRGMode = (regValue >> 2) & 3
            mmc1.CHRMode = (regValue >> 4) & 1
            
            // Update mirroring
            switch regValue & 0x3 {
            case 0: // Single screen, lower bank
                cart.Header.RomType.VerticalMirroring = false
                cart.Header.RomType.HorizontalMirroring = false
            case 1: // Single screen, upper bank
                cart.Header.RomType.VerticalMirroring = false
                cart.Header.RomType.HorizontalMirroring = false
            case 2: // Vertical mirroring
                cart.Header.RomType.VerticalMirroring = true
                cart.Header.RomType.HorizontalMirroring = false
            case 3: // Horizontal mirroring
                cart.Header.RomType.VerticalMirroring = false
                cart.Header.RomType.HorizontalMirroring = true
            }
            
            fmt.Printf("MMC1: Control updated: %02X->%02X, PRGMode: %d\n", 
                oldControl, mmc1.Control, mmc1.PRGMode)

        case addr <= 0xBFFF: // CHR bank 0 ($A000-$BFFF)
            mmc1.CHRBank0 = regValue

        case addr <= 0xDFFF: // CHR bank 1 ($C000-$DFFF)
            mmc1.CHRBank1 = regValue

        case addr <= 0xFFFF: // PRG bank ($E000-$FFFF)
            oldBank := mmc1.PRGBank
            mmc1.PRGBank = regValue & 0x0F
            fmt.Printf("MMC1: PRG Bank switched: %02X->%02X\n", oldBank, mmc1.PRGBank)
        }

        // Reset shift register
        mmc1.ShiftRegister = 0x10
        mmc1.WriteCount = 0
        
        updateBankMapping(cart)
    }
}

func MMC1(addr uint16, cart *cartridge.Cartridge) (bool, uint16) {
    if addr < 0x2000 {
        return false, addr % 0x0800 // RAM mirroring every 2KB
    }
    
    if addr >= 0x2000 && addr <= 0x3FFF {
        return false, 0x2000 + (addr % 8) // PPU registers mirror every 8 bytes
    }

    if addr >= 0x6000 && addr < 0x8000 {
        if cart.Header.RomType.SRAM {
            return false, addr
        }
        return false, addr & 0x1FFF
    }

    if addr >= 0x8000 {
        localAddr := addr & 0x7FFF
        if int(localAddr) >= len(cart.PRG) {
            log.Printf("MMC1: PRG access out of bounds: %04X -> %04X\n", addr, localAddr)
            return true, 0
        }
        return true, localAddr
    }

    return false, addr
}

func MemoryMapper(cart *cartridge.Cartridge, addr uint16) (bool, uint16) {
    switch cart.Header.RomType.Mapper {
    case 0:
        return Zero(addr, cart.Header.ROM_SIZE)
    case 1:
        return MMC1(addr, cart)
    default:
        log.Fatalf("Unsupported mapper: %d", cart.Header.RomType.Mapper)
        return false, 0
    }
}

func Zero(addr uint16, prgsize byte) (bool, uint16) {
    if addr < 0x2000 {
        return false, addr % 0x0800
    }
    
    if addr >= 0x2000 && addr <= 0x3FFF {
        return false, 0x2000 + (addr % 8)
    }
    
    if addr >= 0x8000 {
        if prgsize == 1 {
            return true, addr & 0x3FFF // 16KB ROM
        }
        return true, addr & 0x7FFF // 32KB ROM
    }
    
    return false, addr
}

func PPU(cart *cartridge.Cartridge, addr uint16) uint16 {
    // Handle palette RAM
    if addr >= 0x3F00 {
        addr = 0x3F00 + (addr % 0x20)
        if addr == 0x3F10 || addr == 0x3F14 || addr == 0x3F18 || addr == 0x3F1C {
            addr -= 0x10 // Mirror palette entries
        }
        return addr
    }

    addr = addr % 0x4000
    if addr >= 0x3000 && addr < 0x3F00 {
        addr -= 0x1000
    }

    if addr >= 0x2000 && addr < 0x3000 {
        addr = addr - 0x2000
        table := addr / 0x400
        offset := addr % 0x400

        if cart.Header.RomType.VerticalMirroring {
            if table == 1 || table == 3 {
                addr = 0x2400 + offset
            } else {
                addr = 0x2000 + offset
            }
        } else {
            if table >= 2 {
                addr = 0x2000 + offset
            } else {
                addr = addr + 0x2000
            }
        }
    }

    return addr
}