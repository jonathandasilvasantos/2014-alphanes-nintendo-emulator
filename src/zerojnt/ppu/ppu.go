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

import "fmt"
import "zerojnt/cartridge"
import "zerojnt/mapper"
import "zerojnt/ioports"

import "github.com/veandco/go-sdl2/sdl"
import "os"

var tx uint16 = 0
var ty uint16 = 0

type PPU struct {
	Name string
	CYC int		
	SCANLINE int
	
	
	NMI_DELAY uint
	
	
	
	NAMETABLE byte
	ATTR byte
	HIGH_TILE byte
	LOW_TILE byte
	
	
	VISIBLE_SCANLINE bool
	
	
	IO *ioports.IOPorts
	
}

var window *sdl.Window
var renderer *sdl.Renderer
var event sdl.Event
var xx int

func StartPPU(IO *ioports.IOPorts) PPU {
	var ppu PPU
	ppu.Name = "RICOH RP-2C02\n"
	fmt.Printf("Started PPU")
	fmt.Printf(ppu.Name)
	initCanvas()
	ppu.CYC = 0
	ppu.SCANLINE = -1
	ppu.IO = IO
		
	return ppu
}

func checkVisibleScanline(ppu *PPU) {

	if ppu.SCANLINE >= 0 || ppu.SCANLINE < 240 {
		ppu.VISIBLE_SCANLINE = true
	} else {
		ppu.VISIBLE_SCANLINE = false
	}

}

func Process(ppu *PPU, cart *cartridge.Cartridge) {

for event = sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
	switch t := event.(type) {
		case *sdl.KeyDownEvent:
		if t.Keysym.Sym == 27 {
		fmt.Printf("[%d ms] Keyboard\ttype:%d\tsym:%d\tmodifiers:%d\tstate:%d\trepeat:%d\n",t.Timestamp, t.Type, t.Keysym.Sym, t.Keysym.Mod, t.State, t.Repeat)
			os.Exit(0)
		}			
		break
	}
}

	checkVisibleScanline(ppu)
	
	if (ppu.SCANLINE < 0) && (ppu.CYC <= 0) {
		ppu.SCANLINE = 0
		ppu.CYC = 0

		return
	}
	
	if ppu.CYC >= 0 && ppu.CYC < 256 && ppu.VISIBLE_SCANLINE {
		
		var x uint16 = uint16(ppu.CYC%256)
		var y uint16 = uint16(ppu.CYC%240)
		
		if (tx != x || ty != y) {
			fetchNametable(ppu, x, y)
			drawTile(ppu, x, y)
			tx = x
			ty = y
			
		}
		
		
		
		

	}
	
	
	


	
		if ppu.NMI_DELAY > 0 {
		
			ppu.NMI_DELAY = ppu.NMI_DELAY - 1
			if ppu.NMI_DELAY == 0 && ppu.IO.PPUSTATUS.NMI_OCCURRED == true && ppu.IO.PPUCTRL.GEN_NMI == true {
				ioports.SetNMI(ppu.IO)
			}		
		}
	
	ppu.CYC = ppu.CYC + 1
	if ppu.CYC > 341 {
		
		ppu.CYC = 0
		ppu.SCANLINE = ppu.SCANLINE + 1
		
		
		if ppu.SCANLINE == 241 && ppu.CYC == 0 {
			SetVBLANK(ppu)
			renderer.Present()
		}
		
		if ppu.SCANLINE == 261 {
			ClearVBLANK(ppu)
		}
		
		if ppu.SCANLINE > 261 {			
			
			renderer.SetDrawColor(0,0,0,255)
			renderer.Clear()
			ppu.SCANLINE = -1
		}
		
				
	}
}
	
	func SetVBLANK(ppu *PPU) {
		ppu.IO.PPUSTATUS.VBLANK = true
		ppu.IO.PPUSTATUS.NMI_OCCURRED = true
		nmiHasBeenChanged(ppu)	
	}
	
	func ClearVBLANK(ppu *PPU) {
		ppu.IO.PPUSTATUS.VBLANK = false
		ppu.IO.PPUSTATUS.NMI_OCCURRED = false
		nmiHasBeenChanged(ppu)	
	}
	
	func nmiHasBeenChanged(ppu *PPU) {
		if ppu.IO.PPUSTATUS.NMI_OCCURRED == true && ppu.IO.PPUCTRL.GEN_NMI == true {
			ppu.NMI_DELAY = 15

			}
	}



func initCanvas() {

	var winTitle string = "Alphanes"
	var winWidth, winHeight int = 320, 240

	

	window, err := sdl.CreateWindow(winTitle, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		winWidth, winHeight, sdl.WINDOW_SHOWN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create window: %s\n", err)
		return
	}
	renderer, err = sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create renderer: %s\n", err)
		return
	}
//	defer renderer.Destroy()
}

func fetchTile(ppu *PPU) [8][8]byte {


	var result [8][8]byte
	
	for y := 0; y < 16; y+=2 {
	
			var addr uint16 = ppu.IO.PPUCTRL.BACKGROUND_ADDR + uint16(ppu.NAMETABLE)
			
			var a byte = ppu.IO.PPU_RAM[ mapper.PPU( addr + uint16(y) ) ]
			var b byte = ppu.IO.PPU_RAM[ mapper.PPU(addr+uint16(y)+1)  ]
			
			for x := 0; x < 8; x++ {
				xa := (a << byte(x)) >> 7
				xb := (b << byte(x)) >> 7
				
			
				
				
				if xa == 0 && xb == 0 { result[x][y/2] = 0 }			
				if xa == 1 && xb == 0 { result[x][y/2] = 1 }
				if xa == 0 && xb == 1 { result[x][y/2] = 2 }
				if xa == 1 && xb == 1 { result[x][y/2] = 3 }			
			}
	}
	
	return result
}

func fetchNametable(ppu *PPU, x uint16, y uint16) {
	ppu.NAMETABLE = ppu.IO.PPU_RAM[ mapper.PPU( ppu.IO.PPUCTRL.BASE_NAMETABLE_ADDR + (x*y) ) ]
}

func drawTile(ppu *PPU, x uint16, y uint16) {

	tile := fetchTile(ppu)
	
	for ky := 0; ky < 8; ky++ {
		for kx := 0; kx < 8; kx++ {
		
		
			if tile[kx][ky] == 0 { renderer.SetDrawColor(0, 0, 0, 255) }
			if tile[kx][ky] == 1 { renderer.SetDrawColor(128, 128, 128, 255) }
			if tile[kx][ky] == 2 { renderer.SetDrawColor(190, 190, 190, 255) }
			if tile[kx][ky] == 3 { renderer.SetDrawColor(255, 255, 255, 255) }
			
			
			renderer.DrawPoint(int(x*8) + kx, int(y*8) + ky)
		
		}
	}
	

}