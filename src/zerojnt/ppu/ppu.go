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

import "fmt"
import "zerojnt/cartridge"
import "zerojnt/ioports"
import "zerojnt/debug"
import "os"
import "os/exec"

import "github.com/veandco/go-sdl2/sdl"

var tx uint16 = 0
var ty uint16 = 0


type PPU struct {

	SCREEN_DATA []int
	
	Name string
	CYC int		
	SCANLINE int
        D *debug.PPUDebug
	
	
	
	
	
	ATTR byte
	HIGH_TILE byte
	LOW_TILE byte
	
	
	VISIBLE_SCANLINE bool
	
	
	IO *ioports.IOPorts
	
	
}

var window *sdl.Window
var renderer *sdl.Renderer
var colors = rgb()

func StartPPU(IO *ioports.IOPorts) PPU {
	var ppu PPU
	ppu.Name = "RICOH RP-2C02\n"
	fmt.Printf("Started PPU")
	fmt.Printf(ppu.Name)
	initCanvas()
	
	


	



	
	
	ppu.CYC = 0
	ppu.SCANLINE = 241
	ppu.IO = IO
	
	ppu.SCREEN_DATA = make([]int, 61441)
		
	return ppu
}

func checkVisibleScanline(ppu *PPU) {

	if ppu.SCANLINE >= 0 || ppu.SCANLINE < 240 {
		ppu.VISIBLE_SCANLINE = true
	} else {
		ppu.VISIBLE_SCANLINE = false
	}

}

func checkKeyboard() {
for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event.(type) {
			case *sdl.QuitEvent:
				println("Quit")
				os.Exit(0)
				break
			}
		}
}


func Process(ppu *PPU, cart *cartridge.Cartridge) {



	checkVisibleScanline(ppu)
	
	if (ppu.VISIBLE_SCANLINE) {
	
		var x uint16 = uint16(ppu.CYC%256)
		var y uint16 = uint16(ppu.SCANLINE%240)	
		checkSprite0Bit(ppu, x, y)
	}
	

	
	if (ppu.SCANLINE < 0) && (ppu.CYC <= 0) {
		ppu.SCANLINE = 0
		ppu.CYC = 0
		return
	}
	
	if ppu.CYC >= 0 && ppu.CYC < 256 && ppu.VISIBLE_SCANLINE {
	}
	
	
	


	
		if ppu.IO.PPUSTATUS.NMI_OCCURRED == true && ppu.IO.PPUCTRL.GEN_NMI == true {
		    ioports.SetNMI(ppu.IO)
                    ppu.IO.PPUSTATUS.NMI_OCCURRED = false
		}
					
	ppu.CYC = ppu.CYC + 1
	if ppu.CYC > 341 {
		
		ppu.CYC = 0
		ppu.SCANLINE = ppu.SCANLINE + 1
		
		
		if ppu.SCANLINE == 241 && ppu.CYC == 0 {
			SetVBLANK(ppu)

	checkKeyboard()
		        handleBackground(ppu)
		        handleSprite(ppu)
			ShowScreen(ppu)
		}
		
		if ppu.SCANLINE == 261 {
			ClearVBLANK(ppu)
		}
		
		if ppu.SCANLINE > 261 {			
			ppu.SCANLINE = -1
		}
		
				
	}
}
	
	func SetVBLANK(ppu *PPU) {
		ppu.IO.PPUSTATUS.VBLANK = true
		ppu.IO.PPUSTATUS.NMI_OCCURRED = true
	}

	
func ClearVBLANK(ppu *PPU) {
		ppu.IO.PPUSTATUS.VBLANK = false
		ppu.IO.PPUSTATUS.NMI_OCCURRED = false
	}
	



func initCanvas() {

	var winTitle string = "Alphanes"
	var winWidth, winHeight int32 = 256, 240

	

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

func attrTable(ppu *PPU) [8][8]byte {
    var result [8][8]byte
    
    for x := 0; x < 8; x++ {
        for y := 0; y < 8; y++ {
	    var addr = ppu.IO.PPUCTRL.BASE_NAMETABLE_ADDR + 0x3C0
            addr = addr + uint16(x + (y*8))
        result[x][y] = ReadPPURam(ppu, addr)
        }
    }
    return result
}

func palForBackground(attr [8][8]byte, x uint16, y uint16) byte {

    grid := attr[x/8][y/8]

    bottomright := (grid >> 6)
    bottomleft := (grid << 2) >> 6
    topright := (grid << 4) >> 6
    topleft := (grid << 6) >> 6

    if (x%8 >= 4) && (y%8 < 4) { return topright }
    if (x%8 < 4) && (y%8 >= 4) { return bottomleft }
    if (x%8 < 4) && (y%8 < 4) { return topleft }
    return bottomright
}



func fetchTile(ppu *PPU, index byte, base_addr uint16) [8][8]byte {


	var result [8][8]byte
	
		

	
	for y := 0; y < 8; y++ {
	
	var addr uint16 = base_addr + uint16( uint16(index) * 16)

	tile_addr := addr + uint16(y)
	tile_addr_b :=  tile_addr+8
			
			
			
			
			var a byte = ReadPPURam(ppu,  tile_addr)
			var b byte = ReadPPURam(ppu, tile_addr_b )
			
			for x := 0; x < 8; x++ {
				xa := ReadBit(a, byte(x))
				xb := ReadBit(b, byte(x))
                                result[x][y] = xa+xb
		    }
	}
	
	return result
}

func fetchNametable(ppu *PPU, x uint16, y uint16) byte {

 
	absolute_addr := ppu.IO.PPUCTRL.BASE_NAMETABLE_ADDR + (x+ (y*32)  )
	return  ReadPPURam(ppu, absolute_addr)
	
}



func drawBGTile(ppu *PPU, x uint16, y uint16, index byte, base_addr uint16, flipX bool, flipY bool, ignoreZero bool) {


	tile := fetchTile(ppu, index, base_addr)

        // Getting palette values
        wx := uint16(x/16)
        wy := uint16(y/16)
        attrpal := attrTable(ppu)
        pal := palForBackground(attrpal, wx, wy)

        //var ca uint16 = 0
        //var cb uint16 = 1
        //var cc uint16 = 2
        //var cd uint16 = 3

        



	
	for ky := 0; ky < 8; ky++ {
		for kx := 0; kx < 8; kx++ {
		
			
			var ox int = int(x) + kx
			
			if (flipX == true) {
				ox = (int(x) + 8) - kx
			}
			
			var oy int = int(y) + ky
			


                            if oy < 240 {
                                
                color := uint16(tile[kx][ky] + (pal*4) + 1)
                var coloraddr = uint16(0x3F00+color)
                color = uint16(ReadPPURam(ppu, coloraddr))
                    if tile[kx][ky] == 0 { color = uint16(ppu.IO.PPU_RAM[0x3F00]) }
                    

			        WRITE_SCREEN(ppu, ox, oy, int(color) )
                            }
			
		
		}
	}
	

}


func drawTile(ppu *PPU, x uint16, y uint16, index byte, base_addr uint16, flipX bool, flipY bool, attr byte) {


	        tile := fetchTile(ppu, index, base_addr)
	
	for ky := 0; ky < 8; ky++ {
		for kx := 0; kx < 8; kx++ {
		
			
			var ox int = int(x) + kx
			
			if (flipX == true) {
				ox = (int(x) + 8) - kx
			}
			
			var oy int = int(y) + ky
			


                            if oy < 240 {
                            pal := uint16(((attr << 6) >> 6))
                            coloraddr := uint16( 0x3F10 + (pal*4 + 1) )
                color := ReadPPURam(ppu, coloraddr + uint16(tile[kx][ky]) )
                if tile[kx][ky] == 0 { color = 0 }


			        WRITE_SCREEN(ppu, ox, oy, int(color) )
                            }
			
		
		}
	}
	

}


func ShowScreen(ppu *PPU) {

			renderer.SetDrawColor(0,0,0,255)
			renderer.Clear()

	for x:=0; x<256; x++ {
		for y:=0; y<240; y++ {
			c := READ_SCREEN(ppu, x, y)
			

	    renderer.SetDrawColor(colors[c][0], colors[c][1], colors[c][2], 255)
		    if c == 0 { renderer.SetDrawColor(0, 0, 0, 255) }

			var ox int32 = int32(x)
			var oy int32 = int32(y)
			renderer.DrawPoint(ox, oy)
			
		}
	}
	renderer.Present()
}

func READ_SCREEN(ppu *PPU, x int, y int) int {
	return ppu.SCREEN_DATA[x +(y*256) ]
}

func WRITE_SCREEN(ppu *PPU, x int, y int, k int) {
	if x >= 256 || y >= 240 {
		return
	}
	ppu.SCREEN_DATA[x + (y*256) ] = k
}

func printNametable(ppu *PPU) {

	c := exec.Command("clear")
	c.Stdout = os.Stdout
	c.Run()

	for x:= 0; x < 32; x++ {
		for y:= 0; y < 32; y++ {
		}
	}

}

func handleBackground(ppu *PPU) {

    if ppu.IO.PPUMASK.SHOW_BACKGROUND == false {
        return
    }

    for lx :=0; lx < 32; lx++ {
        for ly :=0; ly < 30; ly++ {
        y := uint16(ly)
        x := uint16(lx)

		tileid := fetchNametable(ppu, x, y)
	drawBGTile(ppu,
                    x*8,
                    y*8,
                    tileid,
                    ppu.IO.PPUCTRL.BACKGROUND_ADDR,
                    false,
                    false,
                    false)
    }
}
}

func handleSprite(ppu *PPU) {

    if ppu.IO.PPUMASK.SHOW_SPRITE == false {
        return
    }

				for s := 0; s<256; s+=4 {
					pos_y := uint16( ppu.IO.PPU_OAM[s] )
					attr := ppu.IO.PPU_OAM[s+2]
					pos_x := uint16( ppu.IO.PPU_OAM[s+3] )
					ind := ppu.IO.PPU_OAM[s+1]
					
					
					var flipX bool = false
					var flipY bool = false
					
					if (attr << 7) >> 7 == 1 {
						flipY = true
					}
					
					if (attr << 6) >> 7 == 1 {
						flipX = true
					}
					



					drawTile(ppu, 
                                            pos_x,
                                            pos_y,
                                            ind,
                                            ppu.IO.PPUCTRL.SPRITE_8_ADDR,
                                            flipX,
                                            flipY,
                                            attr)

					
				} 
}

func checkSprite0Bit(ppu *PPU, x uint16, y uint16) {

if(ppu.IO.PPUSTATUS.SPRITE_0_BIT == true) { return }

	pos_y := uint16( ppu.IO.PPU_OAM[0])
	pos_x := uint16( ppu.IO.PPU_OAM[3] )
	ind := ppu.IO.PPU_OAM[1]
	
	matchVertical := pos_y >= y && (pos_y+8) <= y
	matchHorizontal := pos_x >= y && (pos_x+8) <= x
	
	
	if matchVertical == false || matchHorizontal == false { return }
	fmt.Printf("match!\n")
	
	deltaX := pos_x - x
	deltaY := pos_y - y
	
	sprite_tile := fetchTile(ppu, ind,  ppu.IO.PPUCTRL.SPRITE_8_ADDR )
	fetchNametable(ppu, x/8, y/8)
	bg_tile := fetchTile(ppu, ind,  ppu.IO.PPUCTRL.BACKGROUND_ADDR )
	
	if sprite_tile[deltaX][deltaY] != 0 && bg_tile[x%8][y%8] != 0 {
		ppu.IO.PPUSTATUS.SPRITE_0_BIT = true
		fmt.Printf("Sprite zero!\n")
	}
	
	
	

}
