/*
Copyright 2014, 2015 Jonathan da Silva SAntos

This file is part of Alphanes.

    Alphanes is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    Alphanes is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with Alphanes.  If not, see <http://www.gnu.org/licenses/>.
*/
package mapper

import (
    "zerojnt/cartridge"
    "log"
)

// Zero handles NROM mapper (mapper 0)
// Returns (isPRGROM, mappedAddress)
func Zero(addr uint16, prgsize byte) (bool, uint16) {
    // Handle PRG ROM mapping
    if addr >= 0x8000 {
        isPRGROM := true
        
        // For 16KB PRG ROM games (prgsize=1), mirror 0x8000-0xBFFF to 0xC000-0xFFFF
        if prgsize == 1 && addr >= 0xC000 {
            return isPRGROM, addr - 0xC000
        }
        
        // For 32KB PRG ROM games (prgsize=2), use full range
        if prgsize == 2 {
            return isPRGROM, addr - 0x8000
        }
        
        // Handle individual banks
        if addr < 0xC000 {
            return isPRGROM, addr - 0x8000
        } else {
            return isPRGROM, addr - 0xC000
        }
    }
    
    // Handle RAM mirroring
    if addr < 0x2000 {
        // Mirror RAM every 2KB (0x0800)
        return false, addr % 0x0800
    }
    
    // Handle PPU register mirroring
    if addr >= 0x2000 && addr <= 0x3FFF {
        // Mirror every 8 bytes in the range
        return false, 0x2000 + (addr % 8)
    }
    
    // All other addresses pass through unchanged
    return false, addr
}

// MemoryMapper handles dispatching to the appropriate mapper based on the cartridge type
func MemoryMapper(cart *cartridge.Cartridge, addr uint16) (bool, uint16) {
    switch cart.Header.RomType.Mapper {
    case 0:
        return Zero(addr, cart.Header.ROM_SIZE)
    default:
        log.Fatalf("Unsupported mapper: %d", cart.Header.RomType.Mapper)
        return false, 0
    }
}

// PPU maps CPU addresses to PPU address space, handling nametable and palette mirroring
func PPU(cart *cartridge.Cartridge, addr uint16) uint16 {
    // First wrap any address above 0x3FFF
    addr = addr % 0x4000
    
    // Pattern Tables: 0x0000-0x1FFF
    if addr < 0x2000 {
        return addr
    }
    
    // Nametables: 0x2000-0x2FFF
    if addr < 0x3000 {
        // Remove the base nametable address to get offset
        offset := addr - 0x2000
        
        // Calculate which nametable we're targeting (0-3)
        nt := (offset / 0x400) % 4
        
        // Get the offset within the nametable
        ntOffset := offset % 0x400
        
        var targetNt uint16
        if cart.Header.RomType.VerticalMirroring {
            // In vertical mirroring:
            // NT 0 mirrors NT 2
            // NT 1 mirrors NT 3
            targetNt = nt % 2
        } else {
            // In horizontal mirroring:
            // NT 0 mirrors NT 1
            // NT 2 mirrors NT 3
            targetNt = (nt / 2) * 2
        }
        
        return 0x2000 + (targetNt * 0x400) + ntOffset
    }
    
    // Mirror 0x3000-0x3EFF to 0x2000-0x2EFF
    if addr < 0x3F00 {
        return addr - 0x1000
    }
    
    // Palette RAM: 0x3F00-0x3FFF
    if addr >= 0x3F00 {
        // Mirror every 32 bytes
        addr = 0x3F00 + (addr % 0x20)
        
        // Handle palette mirroring
        switch addr {
        case 0x3F10, 0x3F14, 0x3F18, 0x3F1C:
            addr -= 0x10
        }
        
        return addr
    }
    
    // Should never reach here
    log.Printf("Invalid PPU address: %04X", addr)
    return 0x3F00 // Return background color address as fallback
}