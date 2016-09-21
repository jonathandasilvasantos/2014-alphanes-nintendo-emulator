/*
Copyright 2014, 2015 Jonathan da Silva SAntos

This file is part of Alphanes.

    Alphanes is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    Foobar is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with Foobar.  If not, see <http://www.gnu.org/licenses/>.
*/
package ppu

import "zerojnt/ioports"
import "zerojnt/mapper"
import "fmt"

func WMPPU(IO *ioports.IOPorts, addr uint16, value byte) {
	fmt.Printf("ppu wm: %x value:%x\n", mapper.PPU(addr), value )
	IO.PPU_RAM[mapper.PPU(addr)] = value
}

func RMPPU(IO *ioports.IOPorts, addr uint16) byte {
	fmt.Printf("ppu rm: %x\n", mapper.PPU(addr) )
	return IO.PPU_RAM[mapper.PPU(addr)]
}