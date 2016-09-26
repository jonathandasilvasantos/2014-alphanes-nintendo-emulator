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
//import "fmt"

type PPU_STATUS struct {
	WRITTEN byte // Least significant bits previously written into a PPU register
	SPRITE_OVERFLOW bool // More then 8 sprites in a scanline SPRITE_0_BIT bool // Set when a nonzero pixel of sprite 0 overlaps
         				// a nonzero background pixel; cleared at dot 1 of the pre-render line.  Used for raster timing.
	SPRITE_0_BIT bool // Sprite colides.
	VBLANK bool // Vertical Blank
	NMI_OCCURRED bool

	
}

type PPU_SCROLL struct {
	X byte
	Y byte
	NEXT_WRITES_Y bool
}

type PPU_MASK struct {

	GREYSCALE bool // 0: false (normal color) 1: true (grayscale)
	SHOW_LEFTMOST_8_BACKGROUND bool // Show background in leftmost 8 pixels of screen, 0: Hide
	SHOW_LEFTMOST_8_SPRITE bool // 1: Show sprites in leftmost 8 pixels of screen, 0: Hide
	SHOW_BACKGROUND bool // 0: false 1: true
	SHOW_SPRITE bool // 0: false 1: true
	RED_BOOST bool
	GREEN_BOOST bool
	BLUE_BOOST bool
}

type PPU_CTRL struct {

	BASE_NAMETABLE_ADDR uint16 // Determined by Bit 1 ** 0 = 0x2000 1 = 0x2400
	VRAM_INCREMENT uint16 // if BIT 2 of 0x2000 is 0; increment = 1 if it's 1 so incrment = 32
	SPRITE_8_ADDR uint16 // 0: 0x0000 1: 0x1000 (ignored if sprite mode 16 is true)
	BACKGROUND_ADDR uint16 // 0: 0x0000 1: 0x1000
	SPRITE_SIZE uint16 // 0: 8x8 1: 8x16
	MASTER_SLAVE_SWITCH uint16 // (0: read backdrop from EXT pins; 1: output color on EXT pins)
	GEN_NMI bool // Generate an NMI at the start of the vertical blanking interval (0: off; 1: on)
}

type IOPorts struct {
	CPU_RAM []byte
	PPU_RAM []byte

	PPU_MEMORY_STEP byte // Used in 0x2006 to specify if it's need to record the lower or higher byte.
	PPU_MEMORY_LOWER byte
	PPU_MEMORY_HIGHER byte
	VRAM_ADDRESS uint16
	
	PPU_OAM []byte
	PPU_OAM_ADDRESS byte
	PPUCTRL PPU_CTRL
	PPUMASK PPU_MASK
	PPUSTATUS PPU_STATUS
	PPUSCROLL PPU_SCROLL
	NMI bool
}

func StartIOPorts(cart *cartridge.Cartridge) IOPorts {
	var io IOPorts
	io.CPU_RAM = make([]byte, 0xFFFF)
	io.PPU_RAM = make([]byte, 0x4000)
	io.NMI = false
	
	io.PPUSTATUS.NMI_OCCURRED = true
	io.PPUSTATUS.SPRITE_0_BIT = false
	io.PPUSTATUS.SPRITE_OVERFLOW = true
	

	
	
	
	var page8bits = 8192
	var size int = int(cart.Header.VROM_SIZE)*page8bits

	for i := 0; i < size; i++ {
		io.PPU_RAM[i] = cart.CHR[i]
	}

	
	io.PPU_OAM = make([]byte, 256)
	return io
}

func RMPPU(IO *IOPorts, cart *cartridge.Cartridge, addr uint16) byte {

	//fmt.Printf("rd %x\n", addr)

	switch(addr) {
	
		case 0x2002:
			return READ_PPUSTATUS(IO)
		break
		
		case 0x2004:
			return READ_OAMDATA(IO)
		break
		
		case 0x2007:
			return READ_PPUDATA(IO, cart)
		break
			
	
	}
	return 0
}


func WMPPU(IO *IOPorts, cart *cartridge.Cartridge, addr uint16, value byte) {

	//fmt.Printf("wrt %x : %x\n", addr, value)

	// Last bytes written
	IO.PPUSTATUS.WRITTEN = value

	switch(addr) {
	
		case 0x4014:
			WRITE_OAMDMA(IO, cart, value)
		break
	
		case 0x2000:
			WRITE_PPUCTRL(IO, value)
		break
		
		case 0x2001:
			WRITE_PPUMASK(IO, value)
		break
		
		case 0x2003:
			WRITE_OAMADDR(IO, value)
		break
		
		case 0x2004:
			WRITE_OAMDATA(IO, value)
		break
		
		case 0x2005:
			WRITE_PPUSCROLL(IO, value)
		break
		
		case 0x2006:
			WRITE_PPUADDR(IO, value)
		break
		
		case 0x2007:
			WRITE_PPUDATA(IO, cart, value)
		break
		
	
	}
}

func SetNMI(IO *IOPorts) {
	IO.NMI = true
}