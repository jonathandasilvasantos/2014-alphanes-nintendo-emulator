package mapper

import (
	"fmt"
	"log"
	"zerojnt/cartridge"
)

// Memory bank size constants
const (
	PRG_BANK_SIZE uint32 = 16384 // 16KB PRG ROM bank size
	CHR_BANK_SIZE uint32 = 8192  // 8KB CHR ROM/RAM bank size
	SRAM_SIZE     uint32 = 8192  // 8KB SRAM size
)

// MMC1 control register bits
const (
	MMC1_CHR_MODE_8K  = 0x00
	MMC1_CHR_MODE_4K  = 0x10
	MMC1_PRG_MODE_32K = 0x00
	MMC1_PRG_MODE_FIRST_FIXED = 0x08
	MMC1_PRG_MODE_LAST_FIXED  = 0x0C
	MMC1_MIRROR_SINGLE_LOWER = 0x00
	MMC1_MIRROR_SINGLE_UPPER = 0x01
	MMC1_MIRROR_VERTICAL     = 0x02
	MMC1_MIRROR_HORIZONTAL   = 0x03
)

// MMC1State represents the internal state of the MMC1 mapper
type MMC1State struct {
	ShiftRegister  byte  // 5-bit shift register for serial port
	WriteCount     byte  // Tracks number of writes (0-4) to shift register
	Control        byte  // Control register ($8000-$9FFF)
	CHRBank0       byte  // CHR bank 0 register ($A000-$BFFF)
	CHRBank1       byte  // CHR bank 1 register ($C000-$DFFF)
	PRGBank        byte  // PRG bank register ($E000-$FFFF)
	PRGRAMEnable   bool  // PRG RAM enable flag (MMC1B and later)
	LastWriteCycle uint  // Cycle count of the last write (for consecutive write detection)
	MMC1Revision   uint8 // To distinguish between MMC1A (0) and MMC1B (1) and later
}

// Global MMC1 state
var mmc1 MMC1State

// MapperError represents mapper-specific errors
type MapperError struct {
	Operation string
	Message   string
}

func (e *MapperError) Error() string {
	return fmt.Sprintf("Mapper Error during %s: %s", e.Operation, e.Message)
}

// init initializes the MMC1 mapper state when the package is loaded
func init() {
	resetMMC1()
}

// resetMMC1 resets the MMC1 mapper to its power-on state
func resetMMC1() {
	mmc1.ShiftRegister = 0x10
	mmc1.WriteCount = 0
	mmc1.Control = MMC1_PRG_MODE_LAST_FIXED // PRG mode 3, CHR mode 0, Single-screen lower bank
	mmc1.CHRBank0 = 0
	mmc1.CHRBank1 = 0
	mmc1.PRGBank = 0
	mmc1.PRGRAMEnable = false // Default to disabled for compatibility
	mmc1.LastWriteCycle = 0
	mmc1.MMC1Revision = 1 // Default to MMC1B behavior
}

// updateBankMapping updates PRG and CHR ROM bank mapping based on current MMC1 state
func updateBankMapping(cart *cartridge.Cartridge) error {
	if len(cart.OriginalPRG) == 0 {
		return &MapperError{"updateBankMapping", "OriginalPRG is empty - ROM data wasn't properly loaded"}
	}

	numPRGBanks := uint32(len(cart.OriginalPRG)) / PRG_BANK_SIZE
	if numPRGBanks == 0 {
		return &MapperError{"updateBankMapping", "Invalid PRG ROM size"}
	}

	if len(cart.PRG) != 32768 {
		cart.PRG = make([]byte, 32768)
	}

	if err := updatePRGBanks(cart, numPRGBanks); err != nil {
		return err
	}

	if err := updateCHRBanks(cart); err != nil {
		return err
	}

	return nil
}

// updatePRGBanks handles PRG ROM bank switching based on current mode
func updatePRGBanks(cart *cartridge.Cartridge, numPRGBanks uint32) error {
	prgMode := mmc1.Control & MMC1_PRG_MODE_LAST_FIXED

	// Handle PRG RAM enabling/disabling for MMC1B and later
	if mmc1.MMC1Revision > 0 && !mmc1.PRGRAMEnable {
		// PRG RAM is disabled, do not map
		return nil
	}

	switch prgMode {
	case MMC1_PRG_MODE_32K, MMC1_PRG_MODE_FIRST_FIXED: // 32KB switching mode
		bankBase := uint32(mmc1.PRGBank&0x0E) * PRG_BANK_SIZE
		if bankBase >= uint32(len(cart.OriginalPRG)) {
			bankBase = (uint32(len(cart.OriginalPRG)) - 2*PRG_BANK_SIZE) % (numPRGBanks * PRG_BANK_SIZE)
		}
		if bankBase+2*PRG_BANK_SIZE > uint32(len(cart.OriginalPRG)) {
			return &MapperError{"updatePRGBanks", "PRG bank out of bounds in 32KB mode"}
		}
		copy(cart.PRG[0:16384], cart.OriginalPRG[bankBase:bankBase+PRG_BANK_SIZE])
		copy(cart.PRG[16384:32768], cart.OriginalPRG[bankBase+PRG_BANK_SIZE:bankBase+2*PRG_BANK_SIZE])

	case MMC1_PRG_MODE_LAST_FIXED: // Switchable first bank, fixed last bank
		// In MMC1A, bit 3 selects 16KB bank at $8000 if '0', and controls PRG A17 if '1'.
		if mmc1.MMC1Revision == 0 && (mmc1.PRGBank&0x08) != 0 {
			// MMC1A with bit 3 set in PRG bank register
			bankNum := uint32(mmc1.PRGBank&0x07) * PRG_BANK_SIZE // Ignore bit 3
			if bankNum >= uint32(len(cart.OriginalPRG)) {
				bankNum = (uint32(len(cart.OriginalPRG)) - PRG_BANK_SIZE) % (numPRGBanks * PRG_BANK_SIZE)
			}
			if bankNum+PRG_BANK_SIZE > uint32(len(cart.OriginalPRG)) {
				return &MapperError{"updatePRGBanks", "PRG bank out of bounds in MMC1A fixed-last mode"}
			}
			copy(cart.PRG[0:8192], cart.OriginalPRG[bankNum:bankNum+8192])
			copy(cart.PRG[8192:16384], cart.OriginalPRG[bankNum+8192:bankNum+PRG_BANK_SIZE])

			lastBankStart := (numPRGBanks - 1) * PRG_BANK_SIZE
			copy(cart.PRG[16384:24576], cart.OriginalPRG[lastBankStart:lastBankStart+8192])
			copy(cart.PRG[24576:32768], cart.OriginalPRG[lastBankStart+8192:lastBankStart+PRG_BANK_SIZE])
		} else {
			// MMC1B behavior or MMC1A with bit 3 clear
			bankNum := uint32(mmc1.PRGBank&0x0F) * PRG_BANK_SIZE
			if bankNum >= uint32(len(cart.OriginalPRG)) {
				bankNum = (uint32(len(cart.OriginalPRG)) - PRG_BANK_SIZE) % (numPRGBanks * PRG_BANK_SIZE)
			}
			if bankNum+PRG_BANK_SIZE > uint32(len(cart.OriginalPRG)) {
				return &MapperError{"updatePRGBanks", "PRG bank out of bounds in fixed-last mode"}
			}
			copy(cart.PRG[0:16384], cart.OriginalPRG[bankNum:bankNum+PRG_BANK_SIZE])

			lastBankStart := (numPRGBanks - 1) * PRG_BANK_SIZE
			copy(cart.PRG[16384:32768], cart.OriginalPRG[lastBankStart:lastBankStart+PRG_BANK_SIZE])
		}
	}

	return nil
}

// updateCHRBanks handles CHR ROM/RAM bank switching
func updateCHRBanks(cart *cartridge.Cartridge) error {
	chrMode := mmc1.Control & MMC1_CHR_MODE_4K

	// Handle no CHR data case (use 8KB CHR RAM)
	if len(cart.OriginalCHR) == 0 {
		if len(cart.CHR) != 8192 {
			cart.CHR = make([]byte, 8192)
		}
		return nil
	}

	numCHRBanks := uint32(len(cart.OriginalCHR)) / CHR_BANK_SIZE
	if numCHRBanks == 0 {
		return &MapperError{"updateCHRBanks", "Invalid CHR ROM size"}
	}

	if chrMode == MMC1_CHR_MODE_8K {
		// 8KB CHR ROM mode
		bankBase := uint32(mmc1.CHRBank0&0xFE) * CHR_BANK_SIZE
		if bankBase >= uint32(len(cart.OriginalCHR)) {
			bankBase = (uint32(len(cart.OriginalCHR)) - CHR_BANK_SIZE) % (numCHRBanks * CHR_BANK_SIZE)
		}
		if bankBase+CHR_BANK_SIZE > uint32(len(cart.OriginalCHR)) {
			return &MapperError{"updateCHRBanks", "CHR bank out of bounds in 8KB mode"}
		}
		copy(cart.CHR[0:8192], cart.OriginalCHR[bankBase:bankBase+CHR_BANK_SIZE])
	} else {
		// 4KB CHR ROM mode
		bank0 := uint32(mmc1.CHRBank0) * CHR_BANK_SIZE / 2
		bank1 := uint32(mmc1.CHRBank1) * CHR_BANK_SIZE / 2

		if bank0 >= uint32(len(cart.OriginalCHR)) {
			bank0 = (uint32(len(cart.OriginalCHR))) % (numCHRBanks * CHR_BANK_SIZE / 2)
		}
		if bank1 >= uint32(len(cart.OriginalCHR)) {
			bank1 = (uint32(len(cart.OriginalCHR))) % (numCHRBanks * CHR_BANK_SIZE / 2)
		}
		if bank0+CHR_BANK_SIZE/2 > uint32(len(cart.OriginalCHR)) {
			return &MapperError{"updateCHRBanks", "CHR bank 0 out of bounds in 4KB mode"}
		}
		if bank1+CHR_BANK_SIZE/2 > uint32(len(cart.OriginalCHR)) {
			return &MapperError{"updateCHRBanks", "CHR bank 1 out of bounds in 4KB mode"}
		}

		copy(cart.CHR[0:4096], cart.OriginalCHR[bank0:bank0+CHR_BANK_SIZE/2])
		copy(cart.CHR[4096:8192], cart.OriginalCHR[bank1:bank1+CHR_BANK_SIZE/2])
	}

	return nil
}

// MMC1Write handles writes to MMC1 mapper registers
func MMC1Write(cart *cartridge.Cartridge, addr uint16, value byte) {
	// Detect consecutive writes
	if mmc1.LastWriteCycle > 0 {
		mmc1.LastWriteCycle = 0
		return // Ignore this write
	}

	if (value & 0x80) != 0 {
		// Reset shift register and set PRG bank mode to 3 (16KB, last bank fixed)
		mmc1.ShiftRegister = 0x10
		mmc1.WriteCount = 0
		mmc1.Control |= MMC1_PRG_MODE_LAST_FIXED
		mmc1.LastWriteCycle = 0
		return
	}

	// Write the lowest bit of the value into the shift register
	mmc1.ShiftRegister = ((mmc1.ShiftRegister >> 1) | ((value & 0x01) << 4))
	mmc1.WriteCount++

	// If we've written 5 bits, determine the target register and write the shift register's value to it
	if mmc1.WriteCount == 5 {
		registerData := mmc1.ShiftRegister

		switch {
		case addr >= 0x8000 && addr <= 0x9FFF:
			updateControlRegister(cart, registerData)
		case addr >= 0xA000 && addr <= 0xBFFF:
			mmc1.CHRBank0 = registerData
			if cart.Header.RomType.Mapper == 105 {
				// MMC1 from NES-EVENT board.
				// PRG RAM disable (0: enable, 1: open bus).
				if (registerData & 0x10) != 0 {
					mmc1.PRGRAMEnable = false
				} else {
					mmc1.PRGRAMEnable = true
				}
			}
		case addr >= 0xC000 && addr <= 0xDFFF:
			mmc1.CHRBank1 = registerData
		case addr >= 0xE000:
			mmc1.PRGBank = registerData
			// PRG RAM enable/disable based on bit 4 of PRG bank register, except on MMC1A
			if mmc1.MMC1Revision != 0 {
				mmc1.PRGRAMEnable = (registerData & 0x10) == 0
			}
		}

		// Reset the shift register and write count for the next sequence
		mmc1.ShiftRegister = 0x10
		mmc1.WriteCount = 0

		// Update the bank mapping after writing to the registers
		if err := updateBankMapping(cart); err != nil {
			log.Printf("Error updating bank mapping: %v", err)
		}
	}
	mmc1.LastWriteCycle = 0
}

// updateControlRegister updates the MMC1 control register and related state
func updateControlRegister(cart *cartridge.Cartridge, value byte) {
	mmc1.Control = value

	// Update mirroring based on the lower two bits of the control register
	switch value & MMC1_MIRROR_HORIZONTAL {
	case MMC1_MIRROR_SINGLE_LOWER: // Single-screen mirroring, lower bank
		cart.Header.RomType.VerticalMirroring = false
		cart.Header.RomType.HorizontalMirroring = false
		cart.Header.RomType.SingleScreenMirroring = true
		cart.Header.RomType.SingleScreenBank = 0
	case MMC1_MIRROR_SINGLE_UPPER: // Single-screen mirroring, upper bank
		cart.Header.RomType.VerticalMirroring = false
		cart.Header.RomType.HorizontalMirroring = false
		cart.Header.RomType.SingleScreenMirroring = true
		cart.Header.RomType.SingleScreenBank = 1
	case MMC1_MIRROR_VERTICAL: // Vertical mirroring
		cart.Header.RomType.VerticalMirroring = true
		cart.Header.RomType.HorizontalMirroring = false
		cart.Header.RomType.SingleScreenMirroring = false
	case MMC1_MIRROR_HORIZONTAL: // Horizontal mirroring
		cart.Header.RomType.VerticalMirroring = false
		cart.Header.RomType.HorizontalMirroring = true
		cart.Header.RomType.SingleScreenMirroring = false
	}
}

// MMC1 handles memory mapping for MMC1 mapper
func MMC1(addr uint16, cart *cartridge.Cartridge) (bool, uint16) {
	switch {
	case addr < 0x2000:
		return false, addr % 0x0800 // RAM mirroring
	case addr >= 0x2000 && addr <= 0x3FFF:
		return false, 0x2000 + (addr % 8) // PPU registers
	case addr >= 0x6000 && addr <= 0x7FFF:
		if mmc1.PRGRAMEnable && cart.Header.RomType.SRAM {
			// PRG RAM enabled and present
			// Check for SNROM, SOROM, SUROM, or SXROM
			if cart.Header.RomType.Mapper == 1 {
				switch {
				case (mmc1.CHRBank0 & 0x10) != 0: // SOROM, SUROM, or SXROM with extended PRG RAM
					prgRAMBank := uint16(mmc1.CHRBank0>>2) & 0x03 // Extract 2-bit bank select
					return false, prgRAMBank*8192 + uint16(addr-0x6000)
				default: // SNROM or other with 8KB PRG RAM
					return false, addr - 0x6000
				}
			}
		}
		return false, 0 // PRG RAM disabled or not present
	case addr >= 0x8000:
		// PRG ROM is already mapped by updatePRGBanks, just return the address within cart.PRG
		return true, addr - 0x8000
	}
	return false, addr
}

// MemoryMapper selects appropriate mapper based on cartridge type
func MemoryMapper(cart *cartridge.Cartridge, addr uint16) (bool, uint16) {
	switch cart.Header.RomType.Mapper {
	case 0: // NROM
		return Zero(addr, cart.Header.ROM_SIZE)
	case 1: // MMC1
		return MMC1(addr, cart)
	default:
		log.Printf("Unsupported mapper: %d", cart.Header.RomType.Mapper)
		return false, 0 // Indicate failure
	}
}

// Zero implements NROM (mapper 0) functionality
func Zero(addr uint16, prgsize byte) (bool, uint16) {
	switch {
	case addr < 0x2000:
		return false, addr % 0x0800 // RAM mirroring
	case addr >= 0x2000 && addr <= 0x3FFF:
		return false, 0x2000 + (addr % 8) // PPU registers
	case addr >= 0x8000:
		if prgsize == 1 {
			// For 16KB PRG ROM, mirror the ROM
			return true, (addr & 0x3FFF)
		}
		return true, addr & 0x7FFF // For 32KB PRG ROM
	}
	return false, addr
}

// PPU handles PPU memory mapping including mirroring
func PPU(cart *cartridge.Cartridge, addr uint16) uint16 {
	if addr >= 0x3F00 && addr <= 0x3FFF {
		// Palette RAM, handle mirroring and special cases
		addr = 0x3F00 + (addr % 0x20)
		if addr == 0x3F10 || addr == 0x3F14 || addr == 0x3F18 || addr == 0x3F1C {
			addr -= 0x10
		}
		return addr
	}

	addr = addr & 0x3FFF
	if addr >= 0x3000 && addr < 0x3F00 {
		addr -= 0x1000
	}

	if addr >= 0x2000 && addr < 0x3000 {
		addr = addr - 0x2000
		table := addr / 0x400
		offset := addr % 0x400

		// Handle mirroring based on nametable configuration
		switch {
		case cart.Header.RomType.VerticalMirroring:
			// Vertical mirroring: NT0/NT2 and NT1/NT3 are mirrors
			if table == 1 || table == 3 {
				addr = 0x2400 + offset
			} else {
				addr = 0x2000 + offset
			}
		case cart.Header.RomType.HorizontalMirroring:
			// Horizontal mirroring: NT0/NT1 and NT2/NT3 are mirrors
			if table >= 2 {
				addr = 0x2000 + offset
			} else {
				addr = addr + 0x2000
			}
		case cart.Header.RomType.SingleScreenMirroring:
			// Single screen mirroring
			addr = 0x2000 + uint16(cart.Header.RomType.SingleScreenBank)*0x400 + offset
		default:
			// Four-screen mirroring (uncommon, requires additional VRAM)
			if cart.Header.RomType.FourScreenVRAM {
				addr = 0x2000 + addr
			} else {
				addr = 0x2000 + offset
			}
		}
	}

	return addr
}