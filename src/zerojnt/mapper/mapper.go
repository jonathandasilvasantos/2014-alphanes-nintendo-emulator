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
import "zerojnt/cartridge"
import "log"

func Zero (addr uint16, prgsize byte) (bool, uint16) {


	
	var prgrom bool = false

	// Check if the memory address targets to PRG-ROM Lower Bank
	if (addr >= 0x8000) && prgsize == 2 {
		addr = addr - 0x8000
		prgrom = true
		return prgrom, addr
	}
	
	if (addr >= 0x8000) && (addr < 0xC000) {
		addr = addr - 0x8000
		prgrom = true
	}

	
	if (addr >= 0xC000) {
		addr = addr - 0xC000
		prgrom = true
	}
	
	if prgrom == true {
		return prgrom, addr
	}
	
	// Check the three mirrors of (0x0000-0x07FF) at (0x0800 - 0x2000)

	
		if addr >= 0x0000 && addr < 0x2000 {
			addr = addr % 0x0800
		}
			
		
	
		// Check the mirrors of (02007-0x2007) to (0x2008 - 0x3FFF)
		if addr >= 0x2008 && addr <= 0x3FFF {
			addr = (addr % 8) + 0x2000
		}
	
	return prgrom, addr
}

func MemoryMapper(cart *cartridge.Cartridge, addr uint16) (bool, uint16) {
	
	if cart.Header.RomType.Mapper == 0 {
		prgrom, newaddr := Zero(addr, cart.Header.ROM_SIZE)
		return prgrom, newaddr
	} else { 
		
		log.Fatal("Memory mapper not supported: ", cart.Header.RomType.Mapper)
	}
	return false, 0
}

// PPU maps the CPU address to the appropriate PPU address, handling nametable mirroring
// and palette mirroring based on the cartridge's configuration.
func PPU(cart *cartridge.Cartridge, addr uint16) uint16 {
    // Handle palette mirroring: $3F10/$3F14/$3F18/$3F1C mirror $3F00/$3F04/$3F08/$3F0C respectively
    switch addr {
    case 0x3F10, 0x3F14, 0x3F18, 0x3F1C:
        addr = 0x3F00 + (addr % 0x20)
    }

    // Apply nametable mirroring for addresses $2000 - $2FFF
    if addr >= 0x2000 && addr < 0x3000 {
        if cart.Header.RomType.VerticalMirroring {
            // Vertical Mirroring: $2000 mirrors $2800, $2400 mirrors $2C00
            if addr >= 0x2800 && addr < 0x2C00 {
                addr -= 0x800 // Mirror $2800-$2BFF to $2000-$23FF
            }
            if addr >= 0x2C00 && addr < 0x3000 {
                addr -= 0x800 // Mirror $2C00-$2FFF to $2400-$27FF
            }
        } else {
            // Horizontal Mirroring: $2000 mirrors $2400, $2800 mirrors $2C00
            if addr >= 0x2400 && addr < 0x2800 {
                addr -= 0x400 // Mirror $2400-$27FF to $2000-$23FF
            }
            if addr >= 0x2C00 && addr < 0x3000 {
                addr -= 0x400 // Mirror $2C00-$2FFF to $2800-$2BFF
            }
        }
    }

    // Handle addresses $3000 - $3EFF by mirroring them to $2000 - $2EFF
    if addr >= 0x3000 && addr < 0x4000 {
        addr -= 0x1000 // Mirror $3000-$3EFF to $2000-$2EFF
    }

    // Handle palette mirroring again for safety (optional, as handled above)
    if addr >= 0x3F00 && addr <= 0x3FFF {
        addr = 0x3F00 + (addr % 0x20)
    }

    // Recursively handle addresses beyond $3FFF by wrapping around
    if addr >= 0x4000 {
        return PPU(cart, addr%0x4000)
    }

    // Log an error if the address is out of range (optional)
    if addr > 0x3FFF {
        log.Printf("PPU Address out of range: %X\n", addr)
        return 0x3F00 // Default to background palette
    }

    return addr
}
