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
package ioports

import "zerojnt/cartridge"
import "zerojnt/mapper"


func WRITE_PPUCTRL(IO *IOPorts, value byte) {

	if Bit0(value) == 0 && Bit1(value) == 0 {
		IO.PPUCTRL.BASE_NAMETABLE_ADDR = 0x2000
	}
	if Bit0(value) == 1 && Bit1(value) == 0 {
		IO.PPUCTRL.BASE_NAMETABLE_ADDR = 0x2400
	}
	if Bit0(value) == 0 && Bit1(value) == 1 {	
		IO.PPUCTRL.BASE_NAMETABLE_ADDR = 0x2800
	}
	if Bit0(value) == 1 && Bit1(value) == 1 {
		IO.PPUCTRL.BASE_NAMETABLE_ADDR = 0x2C00
	}
	
	if Bit2(value) == 0 {
		IO.PPUCTRL.VRAM_INCREMENT = 1
	} else {
		IO.PPUCTRL.VRAM_INCREMENT = 32
	}
	
	if Bit3(value) == 0 {
		IO.PPUCTRL.SPRITE_8_ADDR = 0x0000
	} else {
		IO.PPUCTRL.SPRITE_8_ADDR = 0x1000
	}
	
	if Bit4(value) == 0 {
		IO.PPUCTRL.BACKGROUND_ADDR = 0x0000
	} else {
		IO.PPUCTRL.BACKGROUND_ADDR = 0x1000
	}
	
	if Bit5(value) == 0 {
		IO.PPUCTRL.SPRITE_SIZE = 8
	} else {
		IO.PPUCTRL.SPRITE_SIZE = 16
	}
	
	if Bit6(value) == 0 {
		IO.PPUCTRL.MASTER_SLAVE_SWITCH = 0
	} else {
		IO.PPUCTRL.MASTER_SLAVE_SWITCH = 1
	}
	
	if Bit7(value) == 0 {
		IO.PPUCTRL.GEN_NMI = false
	} else {
	IO.PPUCTRL.GEN_NMI = true
	}
}

func WRITE_PPUMASK(IO *IOPorts, value byte) {

	if Bit0(value) == 0 {
		IO.PPUMASK.GREYSCALE = false
	} else {
		IO.PPUMASK.GREYSCALE = true
	}
	
	if Bit1(value) == 0 {
		IO.PPUMASK.SHOW_LEFTMOST_8_BACKGROUND = false
	} else {
		IO.PPUMASK.SHOW_LEFTMOST_8_BACKGROUND = true
	}
	
	if Bit2(value) == 0 {
		IO.PPUMASK.SHOW_LEFTMOST_8_SPRITE = false
	} else {
		IO.PPUMASK.SHOW_LEFTMOST_8_SPRITE = true
	}
	
	if Bit3(value) == 0 {
		IO.PPUMASK.SHOW_BACKGROUND = false
	} else {
		IO.PPUMASK.SHOW_BACKGROUND = true
	}
	
	if Bit4(value) == 0 {
		IO.PPUMASK.SHOW_SPRITE = false
	} else {
		IO.PPUMASK.SHOW_SPRITE = true
	}
	
	if Bit5(value) == 0 {
		IO.PPUMASK.RED_BOOST = false
	} else {
		IO.PPUMASK.RED_BOOST = true
	}
	
	if Bit6(value) == 0 {
		IO.PPUMASK.GREEN_BOOST = false
	} else {
		IO.PPUMASK.GREEN_BOOST = true
	}
	
	if Bit7(value) == 0 {
		IO.PPUMASK.BLUE_BOOST = false
	} else {
		IO.PPUMASK.BLUE_BOOST = true
	}
}

func WRITE_OAMADDR(IO *IOPorts, value byte) {
	IO.PPU_OAM_ADDRESS = value
}

func WRITE_OAMDATA(IO *IOPorts, value byte) {
		IO.PPU_OAM[IO.PPU_OAM_ADDRESS] = value
		IO.PPU_OAM_ADDRESS++
}

func WRITE_PPUSCROLL(IO *IOPorts, value byte) {

	if IO.PPUSCROLL.NEXT_WRITES_Y == true {
		IO.PPUSCROLL.Y = value		
	} else {
		IO.PPUSCROLL.X = value		
	}
	IO.PPUSCROLL.NEXT_WRITES_Y = !IO.PPUSCROLL.NEXT_WRITES_Y
}

func WRITE_PPUADDR(IO *IOPorts, value byte) {

	if IO.PPU_MEMORY_STEP == 0 {
		// Records the lower byte
		IO.PPU_MEMORY_HIGHER = value
		IO.PPU_MEMORY_STEP = 1
	} else {
		// Record the Higher Byte
		IO.PPU_MEMORY_LOWER = (value << 2) >> 2
		IO.PPU_MEMORY_STEP = 0
		IO.VRAM_ADDRESS = LE(IO.PPU_MEMORY_LOWER, IO.PPU_MEMORY_HIGHER)
	}
}

func WRITE_PPUDATA(IO *IOPorts, cart *cartridge.Cartridge, value byte) {
	
	
	IO.PPU_RAM[ mapper.PPU(IO.VRAM_ADDRESS) ] = value
	IO.VRAM_ADDRESS += IO.PPUCTRL.VRAM_INCREMENT
}

func WRITE_OAMDMA(IO *IOPorts, cart *cartridge.Cartridge, value byte) {
	
	var cpuaddr uint16 = uint16(value) << 8
	for i:=0; i<256; i++ {
		prgrom, finaladdr := mapper.MemoryMapper(cart, cpuaddr)
		var data byte
		if prgrom == true {
			data = cart.PRG[finaladdr]
		} else {
			data = IO.CPU_RAM[finaladdr]
		}
		
		IO.PPU_OAM[IO.PPU_OAM_ADDRESS] = data
		IO.PPU_OAM_ADDRESS++
		cpuaddr++
	}
}