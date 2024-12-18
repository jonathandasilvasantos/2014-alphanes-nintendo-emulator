/*
Copyright 2014, 2015 Jonathan da Silva SAntos

This file is part of Alphanes.

    Alphanes is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    Alphanes is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with Alphanes. If not, see <http://www.gnu.org/licenses/>.
*/
package ioports

import "zerojnt/cartridge"

func READ_PPUSTATUS(IO *IOPorts) byte {
    var result byte = 0
    
    // Preserve lower 5 bits from previous write
    result = IO.PPUSTATUS.WRITTEN & 0x1F
    
    // Set upper 3 status bits
    if IO.PPUSTATUS.SPRITE_OVERFLOW {
        result |= 0x20
    }
    if IO.PPUSTATUS.SPRITE_0_BIT {
        result |= 0x40
    }
    if IO.PPUSTATUS.VBLANK {
        result |= 0x80
    }
    
    // Clear both VBlank and NMI occurred flags
    IO.PPUSTATUS.VBLANK = false
    IO.PPUSTATUS.NMI_OCCURRED = false
    
    // Reset address latch
    IO.PPU_MEMORY_STEP = 0
    
    return result
}

func READ_OAMDATA(IO *IOPorts) byte {
	var result byte = IO.PPU_OAM[IO.PPU_OAM_ADDRESS]
	return result
}

func READ_PPUDATA(IO *IOPorts, cart *cartridge.Cartridge, newaddr uint16) byte {
	var result byte

	if newaddr >= 0x3F00 && newaddr <= 0x3FFF {
		// Palette RAM reads are not buffered
		result = IO.PPU_RAM[newaddr]
	} else {
		// For other addresses, return buffered value and update buffer
		result = IO.PREVIOUS_READ
		IO.PREVIOUS_READ = IO.PPU_RAM[newaddr]
	}

	// Increment VRAM address after read
	IO.VRAM_ADDRESS += IO.PPUCTRL.VRAM_INCREMENT

	return result
}