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

	ppu_handle := addr >= 0x2000 && addr <= 0x3FFF
	prgrom, newaddr := mapper.MemoryMapper(cart, addr)
	if ((newaddr >= 0x2000 && newaddr < 0x2008) || (newaddr == 0x4014) && ppu_handle) {
		ioports.WMPPU(&cpu.IO, cart, newaddr, value)
		return
	}
	
	if prgrom {
		log.Fatal("Error: The program is trying to write in the PRG-ROM!")
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