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
package ioports

import "zerojnt/cartridge"
import "zerojnt/mapper"
//import "fmt"

func READ_PPUSTATUS(IO *IOPorts) byte {

	var result byte = 0
	result = SetBit(result, 0, Bit0(IO.PPUSTATUS.WRITTEN))
	result = SetBit(result, 1, Bit1(IO.PPUSTATUS.WRITTEN))
	result = SetBit(result, 2, Bit2(IO.PPUSTATUS.WRITTEN))
	result = SetBit(result, 3, Bit3(IO.PPUSTATUS.WRITTEN))
	result = SetBit(result, 4, Bit4(IO.PPUSTATUS.WRITTEN))
	
	if IO.PPUSTATUS.SPRITE_OVERFLOW == true {
		result = SetBit(result, 5,1)
	}
	
	if IO.PPUSTATUS.SPRITE_0_BIT == true {
		result = SetBit(result, 6,1)
	}
		
	if IO.PPUSTATUS.NMI_OCCURRED == true {
		result = SetBit(result, 7,1)
	}
	IO.PPUSTATUS.NMI_OCCURRED = false
	
	IO.PPUSCROLL.X = 0
	IO.PPUSTATUS.SPRITE_0_BIT = false
	IO.PPUSCROLL.Y = 0
	IO.PPU_MEMORY_STEP = 0
	IO.VRAM_ADDRESS = 0
	
	return result	
}

func READ_OAMDATA(IO *IOPorts) byte {

		var result byte = IO.PPU_OAM[IO.PPU_OAM_ADDRESS]
		return result
}

func READ_PPUDATA(IO *IOPorts, cart *cartridge.Cartridge) byte {

	

	var newaddr uint16 = mapper.PPU (cart, IO.VRAM_ADDRESS)


	var request byte = IO.PPU_RAM[ newaddr ]
	var result byte = IO.PREVIOUS_READ
	
	if (newaddr >= 0x3F00) && (newaddr <= 0x3F1F) {
            return request
	}  
	
	
	IO.PREVIOUS_READ = request
	return result
}
