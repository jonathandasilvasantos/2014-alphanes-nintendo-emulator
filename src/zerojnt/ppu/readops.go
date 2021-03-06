/*
Copyright 2014, 2014 Jonathan da Silva SAntos

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
package ppu

//import "zerojnt/cartridge"
import "zerojnt/mapper"
//import "zerojnt/ioports"

func ReadPPURam(ppu *PPU, addr uint16) byte {


    newaddr := mapper.PPU(ppu.IO.CART, addr)

    if ppu.D.Enable {
        if newaddr < uint16(len(ppu.D.DUMP)) { return ppu.D.DUMP[addr] }
    }


    var page8bits uint16 = 8192
    var size uint16 = uint16(ppu.IO.CART.Header.VROM_SIZE)*page8bits
	    
    if newaddr < size {
        return ppu.IO.CART.CHR[newaddr]
    }


    return ppu.IO.PPU_RAM[newaddr]

    
}
