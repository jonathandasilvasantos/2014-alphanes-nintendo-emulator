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

func PPU(addr uint16) uint16 {

    // Addresses $3F10/$3F14/$3F18/$3F1C are mirrors of $3F00/$3F04/$3F08/$3F0C. 
        if (addr == 0x3F10) { return 0x3F00 }
        if (addr == 0x3F14) { return 0x3F04 }
        if (addr == 0x3F18) { return 0x3F08 }
        if (addr == 0x3F1C) { return 0x3F0C }

        // Just in case of horizontal mirroring
        // Vertical mirror: TODO
        if (addr >= 0x2400) && (addr < 0x2800) {
            return addr - 0x400
        }
        if (addr >= 0x2C00) && (addr < 0x3000) {
            return addr - 0x400
        }
	
	if (addr >= 0x3F00 && addr <= 0x3FFF) {
		return 0x3F00 + (addr%32)
	}
	
	if (addr >= 0x4000) {
		addr = addr % 0x4000
	}
	return addr
}
