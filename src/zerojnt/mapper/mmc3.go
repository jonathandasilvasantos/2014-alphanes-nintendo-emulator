// File: ./mapper/mmc3.go
package mapper

import (
	"log"
	"sync"
)

// MMC3 Mirroring Modes
const (
	MMC3_MIRROR_VERTICAL   = 0
	MMC3_MIRROR_HORIZONTAL = 1
)

// MMC3State holds the internal state of the MMC3 mapper
type MMC3State struct {
	// Banking Control ($8000)
	bankSelect byte // Lower 3 bits select register R0-R7, bit 6 PRG mode, bit 7 CHR mode

	// Bank Register Values (R0-R7)
	bankRegisters [8]byte

	// Mirroring ($A000)
	mirroringMode byte // 0=Vertical, 1=Horizontal (or Four Screen if cart allows)

	// PRG RAM Protect ($A001)
	prgRAMEnabled      bool // Bit 7: 0=Enabled, 1=Disabled
	prgRAMWriteProtect bool // Bit 6: 0=Write Enabled, 1=Write Disabled (ignored if RAM disabled)

	// IRQ Control
	irqCounter byte   // Scanline counter
	irqLatch   byte   // Value to reload counter with ($C000)
	irqReload  bool   // Flag to reload counter on next eligible clock ($C001 write)
	irqEnabled bool   // Master IRQ enable flag ($E001 write)
	irqPending bool   // Flag indicating IRQ has been triggered ($C001 clears, $E001 enables/ack)

	// Derived/Cached State
	prgOffsets [4]uint32 // Effective offsets in OriginalPRG for CPU banks $8000, $A000, $C000, $E000
	chrOffsets [8]uint32 // Effective offsets in OriginalCHR for PPU banks $0000(x2), $0800(x2), $1000, $1400, $1800, $1C00

	// Cartridge Info Cache
	prgSize       uint32 // Size of OriginalPRG in bytes
	chrSize       uint32 // Size of OriginalCHR in bytes (0 if RAM)
	numPrgBanks8k uint32 // Total number of 8KB banks in OriginalPRG
	numChrBanks1k uint32 // Total number of 1KB banks in OriginalCHR/RAM
	hasSRAM       bool
	hasChrRAM     bool
	hasFourScreen bool
}

// MMC3 represents the MMC3 mapper (Mapper 4).
type MMC3 struct {
	state MMC3State
	cart  MapperAccessor // Interface for accessing cartridge data
	mutex sync.RWMutex   // Use RWMutex for separate read/write locking
}

// Compile-time check to ensure MMC3 implements the Mapper interface
var _ Mapper = (*MMC3)(nil)

// Initialize initializes the MMC3 mapper state based on the cartridge.
func (m *MMC3) Initialize(cart MapperAccessor) {
	m.cart = cart

	m.state.prgSize = cart.GetPRGSize()
	m.state.chrSize = cart.GetCHRSize() // 0 if CHR RAM
	m.state.hasSRAM = cart.HasSRAM()
	m.state.hasChrRAM = (m.state.chrSize == 0)
	m.state.hasFourScreen = cart.HasFourScreenVRAM()

	// Calculate bank counts
	if m.state.prgSize > 0 {
		m.state.numPrgBanks8k = m.state.prgSize / PRG_BANK_SIZE_8K
	} else {
		m.state.numPrgBanks8k = 0
	}

	if m.state.hasChrRAM {
		// Assume 8KB CHR RAM for banking purposes
		effectiveChrSize := cart.GetCHRRAMSize()
		if effectiveChrSize == 0 {
			effectiveChrSize = CHR_BANK_SIZE // Default to 8KB if not specified
		}
		m.state.chrSize = effectiveChrSize // Use effective size for banking
		m.state.numChrBanks1k = m.state.chrSize / CHR_BANK_SIZE_1K
	} else if m.state.chrSize > 0 {
		m.state.numChrBanks1k = m.state.chrSize / CHR_BANK_SIZE_1K
	} else {
		m.state.numChrBanks1k = 0
	}

	log.Printf("MMC3 Initializing: PRG:%dKB(%d banks) CHR:%dKB(%d banks, RAM:%v) SRAM:%v 4SCR:%v",
		m.state.prgSize/1024, m.state.numPrgBanks8k*2, // Report in 16KB equivalent
		cart.GetCHRSize()/1024,                         // Log original CHR size
		m.state.numChrBanks1k, m.state.hasChrRAM,
		m.state.hasSRAM, m.state.hasFourScreen)
}

// Reset resets the MMC3 mapper to its power-on/reset state.
func (m *MMC3) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Reset control registers
	m.state.bankSelect = 0
	for i := range m.state.bankRegisters {
		m.state.bankRegisters[i] = 0
	}
	m.state.mirroringMode = MMC3_MIRROR_VERTICAL // Default
	m.state.prgRAMEnabled = true                 // Typically enabled by default
	m.state.prgRAMWriteProtect = false           // Typically writable by default

	// Reset IRQ state
	m.state.irqCounter = 0
	m.state.irqLatch = 0
	m.state.irqReload = false
	m.state.irqEnabled = false
	m.state.irqPending = false

	// Update banks and mirroring based on reset state
	m.updateBanks()     
	m.updateMirroring() 
	m.copyBanks()       
	log.Println("MMC3 Reset complete.")
}

// updateBanks calculates the effective bank offsets based on current registers. Called with lock held.
func (m *MMC3) updateBanks() {
	// PRG ROM Banking
	prgMode0 := (m.state.bankSelect & 0x40) == 0 // R6/R7 control $8000/$A000

	// Get selected bank numbers from registers R6 and R7
	// Mask bank numbers to be within the available range
	prgBankMask8k := uint32(0)
	if m.state.numPrgBanks8k > 0 {
		prgBankMask8k = m.state.numPrgBanks8k - 1
	}
	r6 := uint32(m.state.bankRegisters[6]) & prgBankMask8k
	r7 := uint32(m.state.bankRegisters[7]) & prgBankMask8k

	// Fixed banks are always the last two 8KB banks
	fixedBankC000 := uint32(0) // Second-to-last bank
	if m.state.numPrgBanks8k > 1 {
		fixedBankC000 = (m.state.numPrgBanks8k - 2)
	}
	fixedBankE000 := uint32(0) // Last bank
	if m.state.numPrgBanks8k > 0 {
		fixedBankE000 = (m.state.numPrgBanks8k - 1)
	}

	if prgMode0 {
		m.state.prgOffsets[0] = r6 * PRG_BANK_SIZE_8K            // $8000 swappable -> R6
		m.state.prgOffsets[1] = r7 * PRG_BANK_SIZE_8K            // $A000 swappable -> R7
		m.state.prgOffsets[2] = fixedBankC000 * PRG_BANK_SIZE_8K // $C000 fixed -> second-to-last
		m.state.prgOffsets[3] = fixedBankE000 * PRG_BANK_SIZE_8K // $E000 fixed -> last
	} else { // prgMode1 implied by else
		m.state.prgOffsets[0] = fixedBankC000 * PRG_BANK_SIZE_8K // $8000 fixed -> second-to-last
		m.state.prgOffsets[1] = r7 * PRG_BANK_SIZE_8K            // $A000 swappable -> R7
		m.state.prgOffsets[2] = r6 * PRG_BANK_SIZE_8K            // $C000 swappable -> R6
		m.state.prgOffsets[3] = fixedBankE000 * PRG_BANK_SIZE_8K // $E000 fixed -> last
	}

	// CHR ROM/RAM Banking
	chrMode0 := (m.state.bankSelect & 0x80) == 0 // R0/R1 are 2KB, R2-R5 are 1KB

	// Get selected bank numbers from registers R0-R5
	// Mask bank numbers to be within the available range
	chrBankMask1k := uint32(0)
	if m.state.numChrBanks1k > 0 {
		chrBankMask1k = m.state.numChrBanks1k - 1
	}

	// R0 and R1 need special handling for 2KB banks
	r0_2k := uint32(m.state.bankRegisters[0] & 0xFE) // Ignore bit 0 for 2KB banks
	r1_2k := uint32(m.state.bankRegisters[1] & 0xFE) // Ignore bit 0 for 2KB banks

	// Apply mask for 2KB banks
	r0_2k &= chrBankMask1k & ^uint32(1) // Mask ensuring it's an even boundary and within range
	r1_2k &= chrBankMask1k & ^uint32(1) // Mask ensuring it's an even boundary and within range

	// 1KB banks use the full register value masked by 1k range
	r2 := uint32(m.state.bankRegisters[2]) & chrBankMask1k
	r3 := uint32(m.state.bankRegisters[3]) & chrBankMask1k
	r4 := uint32(m.state.bankRegisters[4]) & chrBankMask1k
	r5 := uint32(m.state.bankRegisters[5]) & chrBankMask1k

	// CHR mapping logic based on target PPU address ranges
	if chrMode0 { 
		m.state.chrOffsets[0] = r0_2k * CHR_BANK_SIZE_1K // $0000-$07FF
		m.state.chrOffsets[1] = r0_2k*CHR_BANK_SIZE_1K + CHR_BANK_SIZE_1K
		m.state.chrOffsets[2] = r1_2k * CHR_BANK_SIZE_1K // $0800-$0FFF
		m.state.chrOffsets[3] = r1_2k*CHR_BANK_SIZE_1K + CHR_BANK_SIZE_1K
		m.state.chrOffsets[4] = r2 * CHR_BANK_SIZE_1K    // $1000-$13FF
		m.state.chrOffsets[5] = r3 * CHR_BANK_SIZE_1K    // $1400-$17FF
		m.state.chrOffsets[6] = r4 * CHR_BANK_SIZE_1K    // $1800-$1BFF
		m.state.chrOffsets[7] = r5 * CHR_BANK_SIZE_1K    // $1C00-$1FFF
	} else { 
		m.state.chrOffsets[0] = r2 * CHR_BANK_SIZE_1K    // $0000-$03FF
		m.state.chrOffsets[1] = r3 * CHR_BANK_SIZE_1K    // $0400-$07FF
		m.state.chrOffsets[2] = r4 * CHR_BANK_SIZE_1K    // $0800-$0BFF
		m.state.chrOffsets[3] = r5 * CHR_BANK_SIZE_1K    // $0C00-$0FFF
		m.state.chrOffsets[4] = r0_2k * CHR_BANK_SIZE_1K // $1000-$17FF
		m.state.chrOffsets[5] = r0_2k*CHR_BANK_SIZE_1K + CHR_BANK_SIZE_1K
		m.state.chrOffsets[6] = r1_2k * CHR_BANK_SIZE_1K // $1800-$1FFF
		m.state.chrOffsets[7] = r1_2k*CHR_BANK_SIZE_1K + CHR_BANK_SIZE_1K
	}
}

// copyBanks performs the actual copy to mapped windows. Called with lock held.
func (m *MMC3) copyBanks() {
	// Copy PRG Banks (4 * 8KB chunks)
	if m.cart.GetPRGSize() > 0 {
		m.cart.CopyPRGData(0*PRG_BANK_SIZE_8K, m.state.prgOffsets[0], PRG_BANK_SIZE_8K)
		m.cart.CopyPRGData(1*PRG_BANK_SIZE_8K, m.state.prgOffsets[1], PRG_BANK_SIZE_8K)
		m.cart.CopyPRGData(2*PRG_BANK_SIZE_8K, m.state.prgOffsets[2], PRG_BANK_SIZE_8K)
		m.cart.CopyPRGData(3*PRG_BANK_SIZE_8K, m.state.prgOffsets[3], PRG_BANK_SIZE_8K)
	}

	// Copy CHR Banks (8 * 1KB chunks)
	if !m.state.hasChrRAM && m.cart.GetCHRSize() > 0 {
		m.cart.CopyCHRData(0*CHR_BANK_SIZE_1K, m.state.chrOffsets[0], CHR_BANK_SIZE_1K)
		m.cart.CopyCHRData(1*CHR_BANK_SIZE_1K, m.state.chrOffsets[1], CHR_BANK_SIZE_1K)
		m.cart.CopyCHRData(2*CHR_BANK_SIZE_1K, m.state.chrOffsets[2], CHR_BANK_SIZE_1K)
		m.cart.CopyCHRData(3*CHR_BANK_SIZE_1K, m.state.chrOffsets[3], CHR_BANK_SIZE_1K)
		m.cart.CopyCHRData(4*CHR_BANK_SIZE_1K, m.state.chrOffsets[4], CHR_BANK_SIZE_1K)
		m.cart.CopyCHRData(5*CHR_BANK_SIZE_1K, m.state.chrOffsets[5], CHR_BANK_SIZE_1K)
		m.cart.CopyCHRData(6*CHR_BANK_SIZE_1K, m.state.chrOffsets[6], CHR_BANK_SIZE_1K)
		m.cart.CopyCHRData(7*CHR_BANK_SIZE_1K, m.state.chrOffsets[7], CHR_BANK_SIZE_1K)
	}
}

// updateMirroring sets the mirroring mode in the cartridge. Called with lock held.
func (m *MMC3) updateMirroring() {
	if m.state.hasFourScreen {
		m.cart.SetMirroringMode(false, false, true, 0) // Force four screen
	} else if m.state.mirroringMode == MMC3_MIRROR_HORIZONTAL {
		m.cart.SetMirroringMode(false, true, false, 0) // Horizontal
	} else {
		m.cart.SetMirroringMode(true, false, false, 0) // Vertical
	}
}

// MapCPU maps a CPU address ($6000-$FFFF) to a PRG ROM/RAM offset.
func (m *MMC3) MapCPU(addr uint16) (isROM bool, mappedAddr uint16) {
	m.mutex.RLock() 
	defer m.mutex.RUnlock()

	if addr >= 0x6000 && addr <= 0x7FFF {
		if m.state.hasSRAM && m.state.prgRAMEnabled {
			// MMC3 typically has 8KB SRAM mapped directly here when enabled.
			return false, addr & 0x1FFF // Offset within 8KB window
		}
		return false, 0xFFFF // Unmapped/disabled SRAM
	}

	if addr >= 0x8000 {
		// Address relative to the 32KB window ($8000-$FFFF)
		relativeAddr := addr - 0x8000
		return true, relativeAddr
	}

	log.Printf("Warning: MMC3 MapCPU called with unexpected address %04X", addr)
	return false, 0xFFFF // Unmapped
}

// MapPPU maps a PPU address ($0000-$1FFF) to a CHR ROM/RAM offset.
func (m *MMC3) MapPPU(addr uint16) uint16 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if addr < 0x2000 {
		// Address relative to the 8KB window ($0000-$1FFF)
		return addr & 0x1FFF
	}

	log.Printf("Warning: MMC3 MapPPU called with non-CHR address %04X", addr)
	return 0xFFFF // Indicate invalid address for CHR mapping
}

// Write handles CPU writes to mapper registers ($8000-$FFFF) or potentially PRG RAM ($6000-$7FFF).
func (m *MMC3) Write(addr uint16, value byte) {
	// Handle PRG RAM Writes ($6000-$7FFF)
	if addr >= 0x6000 && addr <= 0x7FFF {
		m.mutex.RLock() 
		canWrite := m.state.hasSRAM && m.state.prgRAMEnabled && !m.state.prgRAMWriteProtect
		m.mutex.RUnlock()

		if canWrite {
			m.cart.WriteSRAM(addr&0x1FFF, value)
		}
		return
	}

	// Ignore writes below mapper register range
	if addr < 0x8000 {
		return
	}

	// Mapper register writes require exclusive lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	needsBankUpdate := false
	needsMirrorUpdate := false

	switch {
	case addr >= 0x8000 && addr <= 0x9FFF: // Bank Select / Bank Data
		if addr%2 == 0 { // Bank Select ($8000, $8002, ...)
			if m.state.bankSelect != value {
				m.state.bankSelect = value
				needsBankUpdate = true
			}
		} else { // Bank Data ($8001, $8003, ...)
			regIndex := m.state.bankSelect & 0x07
			maskedValue := value
			
			// PRG banks R6/R7 have fewer bits
			if regIndex == 6 || regIndex == 7 {
				maskedValue &= 0x3F // Mask PRG banks to 6 bits (supports up to 512KB)
			}

			if m.state.bankRegisters[regIndex] != maskedValue {
				m.state.bankRegisters[regIndex] = maskedValue
				needsBankUpdate = true 
			}
		}

	case addr >= 0xA000 && addr <= 0xBFFF: // Mirroring / PRG RAM Protect
		if addr%2 == 0 { // Mirroring ($A000, $A002, ...)
			newMirrorMode := value & 0x01
			if m.state.mirroringMode != newMirrorMode {
				m.state.mirroringMode = newMirrorMode
				needsMirrorUpdate = true
			}
		} else { // PRG RAM Protect ($A001, $A003, ...)
			newEnable := (value & 0x80) == 0
			newWriteProtect := (value & 0x40) != 0
			m.state.prgRAMEnabled = newEnable
			m.state.prgRAMWriteProtect = newWriteProtect
		}

	case addr >= 0xC000 && addr <= 0xDFFF: // IRQ Latch / IRQ Reload
		if addr%2 == 0 { // IRQ Latch ($C000, $C002, ...)
			m.state.irqLatch = value
		} else { // IRQ Reload ($C001, $C003, ...)
			m.state.irqCounter = 0 
			m.state.irqReload = true
		}

	case addr >= 0xE000 && addr <= 0xFFFF: // IRQ Disable / IRQ Enable
		if addr%2 == 0 { // IRQ Disable ($E000, $E002, ...)
			m.state.irqEnabled = false
			m.state.irqPending = false 
		} else { // IRQ Enable ($E001, $E003, ...)
			m.state.irqEnabled = true
		}
	}

	// Apply updates if needed
	if needsBankUpdate {
		m.updateBanks()
		m.copyBanks()
	}
	if needsMirrorUpdate {
		m.updateMirroring()
	}
}

// ClockIRQCounter simulates the MMC3 IRQ counter clocking mechanism.
// Called by the PPU, typically when A12 rises during rendering.
func (m *MMC3) ClockIRQCounter() {
	m.mutex.Lock() 
	defer m.mutex.Unlock()

	if m.state.irqReload {
		m.state.irqCounter = m.state.irqLatch 
		m.state.irqReload = false            
	} else if m.state.irqCounter > 0 {
		m.state.irqCounter-- 
	}

	// If counter is zero after decrement/reload, and IRQs are enabled, trigger pending flag
	if m.state.irqCounter == 0 && m.state.irqEnabled {
		m.state.irqPending = true
	}
}

// IRQState returns true if the mapper is currently asserting the IRQ line.
func (m *MMC3) IRQState() bool {
	m.mutex.RLock() 
	defer m.mutex.RUnlock()
	return m.state.irqPending
}