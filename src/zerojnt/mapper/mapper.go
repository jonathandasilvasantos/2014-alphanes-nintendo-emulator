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

// MMC1State holds the internal state of the MMC1 mapper.
type MMC1State struct {
    ShiftRegister byte   // 5-bit shift register
    WriteCount    byte   // Number of writes to shift register
    Control       byte   // Control register (PRG mode, CHR mode, mirroring)
    CHRBank0      byte   // CHR bank 0 selection
    CHRBank1      byte   // CHR bank 1 selection (used in 4KB mode)
    PRGBank       byte   // PRG bank selection
    PRGMode       byte // PRG ROM bank mode (affects how PRGBank is used)
    CHRMode       byte   // CHR ROM bank mode (0: 8KB, 1: 4KB)
}

var mmc1 MMC1State

func init() {
    resetMMC1()
}

// resetMMC1 initializes the MMC1 mapper to its default state.
func resetMMC1() {
    mmc1.ShiftRegister = 0x10  // Start with shift register in reset state (bit 4 set)
    mmc1.WriteCount = 0
    mmc1.Control = 0x0C      // PRG ROM bank mode 3 (fixed last bank, variable first bank)
    mmc1.CHRBank0 = 0
    mmc1.CHRBank1 = 0
    mmc1.PRGBank = 0
    mmc1.PRGMode = 3
    mmc1.CHRMode = 0  // 8KB CHR mode

    fmt.Printf("MMC1: Reset state - Control: %02X, PRGMode: %d, CHRMode: %d\n",
        mmc1.Control, mmc1.PRGMode, mmc1.CHRMode)
}

// updateBankMapping updates the PRG and CHR ROM bank mapping based on the current MMC1 state.
func updateBankMapping(cart *cartridge.Cartridge) {
    if len(cart.OriginalPRG) == 0 {
        log.Fatal("OriginalPRG is empty - ROM data wasn't properly loaded")
        return
    }

    numPRGBanks := uint32(len(cart.OriginalPRG)) / PRG_BANK_SIZE
    if numPRGBanks == 0 {
        log.Fatal("Invalid PRG ROM size")
        return
    }

    // Ensure PRG space is 32KB for MMC1
    if len(cart.PRG) != 32768 {
        cart.PRG = make([]byte, 32768)
    }

    // PRG ROM bank mapping
    switch mmc1.PRGMode {
    case 0, 1: // 32KB switching
        // Ignore lowest bit of PRG bank selection in this mode
        bankBase := uint32(mmc1.PRGBank & 0x0E) % numPRGBanks
        copy(cart.PRG[0:16384], cart.OriginalPRG[bankBase*PRG_BANK_SIZE:(bankBase+1)*PRG_BANK_SIZE])
        copy(cart.PRG[16384:32768], cart.OriginalPRG[(bankBase+1)*PRG_BANK_SIZE:(bankBase+2)*PRG_BANK_SIZE])

    case 2: // Fixed first bank, switchable last bank
        // First bank is fixed at 0
        copy(cart.PRG[0:16384], cart.OriginalPRG[0:16384])
        // Last bank is switchable
        bankNum := uint32(mmc1.PRGBank) % numPRGBanks
        copy(cart.PRG[16384:32768], cart.OriginalPRG[bankNum*PRG_BANK_SIZE:(bankNum+1)*PRG_BANK_SIZE])

    case 3: // Switchable first bank, fixed last bank
        // First bank is switchable
        bankNum := uint32(mmc1.PRGBank) % numPRGBanks
        copy(cart.PRG[0:16384], cart.OriginalPRG[bankNum*PRG_BANK_SIZE:(bankNum+1)*PRG_BANK_SIZE])
        // Last bank is fixed to the last bank
        copy(cart.PRG[16384:32768], cart.OriginalPRG[(numPRGBanks-1)*PRG_BANK_SIZE:numPRGBanks*PRG_BANK_SIZE])
    }

    // CHR ROM/RAM bank mapping (only if CHR ROM/RAM is present)
    numCHRBanks := uint32(len(cart.OriginalCHR)) / CHR_BANK_SIZE
    if numCHRBanks > 0 {
        if mmc1.CHRMode == 0 {
            // 8KB switching
            bankBase := uint32(mmc1.CHRBank0 & 0xFE) % numCHRBanks
            if (bankBase+2)*CHR_BANK_SIZE <= uint32(len(cart.OriginalCHR)){
               copy(cart.CHR[0:8192], cart.OriginalCHR[bankBase*CHR_BANK_SIZE:(bankBase+2)*CHR_BANK_SIZE])
            }
        } else {
            // 4KB switching
            bank0 := uint32(mmc1.CHRBank0) % numCHRBanks
            bank1 := uint32(mmc1.CHRBank1) % numCHRBanks
            if (bank0+1)*4096 <= uint32(len(cart.OriginalCHR)) {
               copy(cart.CHR[0:4096], cart.OriginalCHR[bank0*4096:(bank0+1)*4096])
            }
            if (bank1+1)*4096 <= uint32(len(cart.OriginalCHR)){
               copy(cart.CHR[4096:8192], cart.OriginalCHR[bank1*4096:(bank1+1)*4096])
            }
        }
    }
}

// MMC1Write handles writes to the MMC1 mapper's registers.
func MMC1Write(cart *cartridge.Cartridge, addr uint16, value byte) {
    // If bit 7 of the value is set, reset the shift register
    if (value & 0x80) != 0 {
        mmc1.ShiftRegister = 0x10
        mmc1.WriteCount = 0
        mmc1.Control |= 0x0C  // Also reset PRG bank mode
        return
    }

    // Load the shift register bit by bit
    shift := (mmc1.ShiftRegister >> 1) | ((value & 0x01) << 4)
    mmc1.ShiftRegister = shift
    mmc1.WriteCount++

    // If we've written 5 bits, write to the appropriate internal register
    if mmc1.WriteCount == 5 {
        registerData := mmc1.ShiftRegister

        switch {
        case addr >= 0x8000 && addr <= 0x9FFF: // Control
            mmc1.Control = registerData
            mmc1.PRGMode = (mmc1.Control >> 2) & 0x03
            mmc1.CHRMode = (mmc1.Control >> 4) & 0x01

            // Mirroring
            switch mmc1.Control & 0x03 {
            case 0: // One-screen, lower bank
                cart.Header.RomType.VerticalMirroring = false
                cart.Header.RomType.HorizontalMirroring = false
            case 1: // One-screen, upper bank
                cart.Header.RomType.VerticalMirroring = false
                cart.Header.RomType.HorizontalMirroring = false
            case 2: // Vertical
                cart.Header.RomType.VerticalMirroring = true
                cart.Header.RomType.HorizontalMirroring = false
            case 3: // Horizontal
                cart.Header.RomType.VerticalMirroring = false
                cart.Header.RomType.HorizontalMirroring = true
            }
            fmt.Printf("MMC1: Control register write - Value: %02X, PRGMode: %d, CHRMode: %d\n",
                mmc1.Control, mmc1.PRGMode, mmc1.CHRMode)

        case addr >= 0xA000 && addr <= 0xBFFF: // CHR bank 0
            mmc1.CHRBank0 = registerData
            fmt.Printf("MMC1: CHR Bank 0 write - Value: %02X\n", mmc1.CHRBank0)

        case addr >= 0xC000 && addr <= 0xDFFF: // CHR bank 1
            mmc1.CHRBank1 = registerData
            fmt.Printf("MMC1: CHR Bank 1 write - Value: %02X\n", mmc1.CHRBank1)

        case addr >= 0xE000 && addr <= 0xFFFF: // PRG bank
            mmc1.PRGBank = registerData
            fmt.Printf("MMC1: PRG Bank write - Value: %02X\n", mmc1.PRGBank)
        }

        // Reset shift register and write counter
        mmc1.ShiftRegister = 0x10
        mmc1.WriteCount = 0

        // Update bank mapping after a register write
        updateBankMapping(cart)
    }
}

// MMC1 maps addresses to PRG and CHR ROM banks based on the MMC1 mapper's current state.
func MMC1(addr uint16, cart *cartridge.Cartridge) (bool, uint16) {
    if addr < 0x2000 {
        // RAM
        return false, addr % 0x0800
    } else if addr >= 0x2000 && addr <= 0x3FFF {
        // PPU registers
        return false, 0x2000 + (addr % 8)
    } else if addr >= 0x6000 && addr <= 0x7FFF {
        // SRAM (if present)
        if cart.Header.RomType.SRAM {
            return false, addr - 0x6000
        } else {
            return false, 0 // No SRAM present
        }
    } else if addr >= 0x8000 && addr <= 0xFFFF {
        // PRG ROM
        localAddr := addr & 0x3FFF // PRG ROM offset
        bankOffset := uint16(0)
        if addr >= 0xC000 {
            bankOffset = 16384 // upper bank
        }

        return true, bankOffset + localAddr
    }

    return false, addr
}

// MemoryMapper determines which mapper to use based on the ROM's header information.
func MemoryMapper(cart *cartridge.Cartridge, addr uint16) (bool, uint16) {
    switch cart.Header.RomType.Mapper {
    case 0: // NROM
        return Zero(addr, cart.Header.ROM_SIZE)
    case 1: // MMC1
        return MMC1(addr, cart)
    default:
        log.Fatalf("Unsupported mapper: %d", cart.Header.RomType.Mapper)
        return false, 0
    }
}

// Zero is the mapper function for NROM (mapper 0).
func Zero(addr uint16, prgsize byte) (bool, uint16) {
    if addr < 0x2000 {
        return false, addr % 0x0800 // RAM
    }

    if addr >= 0x2000 && addr <= 0x3FFF {
        return false, 0x2000 + (addr % 8) // PPU registers
    }

    if addr >= 0x8000 {
        if prgsize == 1 {
            return true, addr & 0x3FFF // 16KB ROM, mirror 0xC000-0xFFFF
        }
        return true, addr & 0x7FFF // 32KB ROM
    }

    return false, addr
}

// PPU maps PPU addresses, handling nametable mirroring and palette RAM access.
func PPU(cart *cartridge.Cartridge, addr uint16) uint16 {
    // Handle Palette RAM
    if addr >= 0x3F00 && addr <= 0x3FFF {
        addr = 0x3F00 + (addr % 0x20)
        // Mirror $3F10/$3F14/$3F18/$3F1C to $3F00/$3F04/$3F08/$3F0C
        if addr == 0x3F10 || addr == 0x3F14 || addr == 0x3F18 || addr == 0x3F1C {
            addr -= 0x10
        }
        return addr
    }

    addr = addr & 0x3FFF
    if addr >= 0x3000 && addr < 0x3F00 {
        addr -= 0x1000 // Fold $3000-$3EFF to $2000-$2EFF
    }

    if addr >= 0x2000 && addr < 0x3000 {
        addr = addr - 0x2000
        table := addr / 0x400
        offset := addr % 0x400

        if cart.Header.RomType.VerticalMirroring {
            // Vertical mirroring
            if table == 1 || table == 3 {
                addr = 0x2400 + offset
            } else {
                addr = 0x2000 + offset
            }
        } else if cart.Header.RomType.HorizontalMirroring {
            // Horizontal mirroring
            if table >= 2 {
                addr = 0x2000 + offset
            } else {
                addr = addr + 0x2000
            }
        } else {
            // One-screen mirroring
            if cart.Header.RomType.FourScreenVRAM {
                // Use raw address for four-screen mode
                addr = 0x2000 + addr
            } else {
                // Use lower nametable for one-screen mirroring
                addr = 0x2000 + offset
            }
        }
    }

    return addr
}