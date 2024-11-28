// Modified memory.go
package cpu

import "zerojnt/cartridge"
import "zerojnt/mapper"
import "zerojnt/ioports"
import "log"

func RM(cpu *CPU, cart *cartridge.Cartridge, addr uint16) byte {
    ppu_handle := addr >= 0x2000 && addr <= 0x3FFF 
    prgrom, newaddr := mapper.MemoryMapper(cart, addr)
    
    if newaddr >= 0x2000 && newaddr < 0x2008 && ppu_handle {
        return ioports.RMPPU(&cpu.IO, cart, newaddr)
    }

    if prgrom {
        return cart.PRG[newaddr]
    } else {
        return cpu.IO.CPU_RAM[newaddr]
    }
}

func WM(cpu *CPU, cart *cartridge.Cartridge, addr uint16, value byte) {

    if addr >= 0x4000 && addr <= 0x4017 {
        cpu.APU.WriteRegister(addr, value) // Changed from cpu.apu to cpu.APU
        return
    }

    // Handle PPU registers first
    ppu_handle := (addr >= 0x2000 && addr <= 0x3FFF) || (addr == 0x4014)
    prgrom, newaddr := mapper.MemoryMapper(cart, addr)
    
    
    if ((newaddr >= 0x2000 && newaddr < 0x2008) || (newaddr == 0x4014) && ppu_handle) {
        ioports.WMPPU(&cpu.IO, cart, newaddr, value)
        return
    }

    // Check if this is an MMC1 write
    if cart.Header.RomType.Mapper == 1 && addr >= 0x8000 {
        // This is an MMC1 register write
        mapper.MMC1Write(cart, addr, value)
        return
    }
    
    // Prevent direct writes to PRG-ROM for non-mapper writes
    if prgrom {
        log.Fatal("Error: The program is trying to write directly to PRG-ROM!")
    }
    
    cpu.IO.CPU_RAM[newaddr] = value
}

func PushMemory(cpu *CPU, v byte) {
    cpu.IO.CPU_RAM[0x0100 + int(cpu.SP)] = v
    cpu.SP--
}

func PopMemory(cpu *CPU) byte {
    cpu.SP++
    var result byte = cpu.IO.CPU_RAM[0x0100 + uint(cpu.SP)]
    return result
}

func PushWord(cpu *CPU, v uint16) {
    PushMemory(cpu, byte(v >> 8))       // Push high byte first
    PushMemory(cpu, byte(v & 0xFF))     // Push low byte next
}

func PopWord(cpu *CPU) uint16 {
    var lo, hi byte
    lo = PopMemory(cpu)                 // Pop low byte first
    hi = PopMemory(cpu)                 // Pop high byte next
    return uint16(hi)<<8 | uint16(lo)   // Combine bytes correctly
}