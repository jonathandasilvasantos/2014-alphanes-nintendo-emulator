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
//import "zerojnt/mapper"
import "zerojnt/ioports"
import "os"
import "math/rand"

import "github.com/veandco/go-sdl2/sdl"

var tx uint16 = 0
var ty uint16 = 0


type PPU struct {

	SCREEN_DATA []int
	
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
var xx int

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



	checkKeyboard()
	ShowScreen(ppu)
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
func ShowScreen(ppu *PPU) {

			renderer.SetDrawColor(0,0,0,255)
			renderer.Clear()

	for x:=0; x<256; x++ {
		for y:=0; y<240; y++ {
			c := rand.Intn(4)
			
			if c == 0 { renderer.SetDrawColor(0, 0, 0, 255) }
			if c == 1 { renderer.SetDrawColor(128, 128, 128, 255) }
			if c == 2 { renderer.SetDrawColor(190, 190, 190, 255) }
			if c == 3 { renderer.SetDrawColor(255, 255, 255, 255) }
			var ox int32 = int32(x)
			var oy int32 = int32(y)
			renderer.DrawPoint(ox, oy)
			
		}
	}
	renderer.Present()
}
