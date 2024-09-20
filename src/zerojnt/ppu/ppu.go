/*
Copyright 2014, 2014 Jonathan da Silva Santos

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

import (
    "fmt"
    "os"
    "zerojnt/cartridge"
    "zerojnt/debug"
    "zerojnt/ioports"

    "github.com/veandco/go-sdl2/sdl"
)

var tx uint16 = 0
var ty uint16 = 0

type PPU struct {
    SCREEN_DATA []int
    Name string
    CYC  int
    SCANLINE int
    D    *debug.PPUDebug
    texture *sdl.Texture
    ATTR      byte
    HIGH_TILE byte
    LOW_TILE  byte
    VISIBLE_SCANLINE bool
    IO *ioports.IOPorts
}

var window *sdl.Window
var renderer *sdl.Renderer
var colors = rgb() // Assume rgb() is defined elsewhere

func StartPPU(IO *ioports.IOPorts) PPU {
    var ppu PPU
    ppu.Name = "RICOH RP-2C02\n"
    fmt.Printf("Started PPU")
    fmt.Printf(ppu.Name)
    initCanvas( &ppu)

    ppu.CYC = 0
    ppu.SCANLINE = -1 // Corrected initialization
    ppu.IO = IO

    ppu.SCREEN_DATA = make([]int, 256*240) // Corrected size

    return ppu
}

func checkVisibleScanline(ppu *PPU) {
    if ppu.SCANLINE >= 0 && ppu.SCANLINE < 240 {
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
        }
    }
}

func Process(ppu *PPU, cart *cartridge.Cartridge) {
    checkVisibleScanline(ppu)

    if ppu.VISIBLE_SCANLINE {
        var x uint16 = uint16(ppu.CYC % 256)
        var y uint16 = uint16(ppu.SCANLINE % 240)
        checkSprite0Bit(ppu, x, y)
    }

    if (ppu.SCANLINE < 0) && (ppu.CYC <= 0) {
        ppu.SCANLINE = 0
        ppu.CYC = 0
        return
    }

    if ppu.CYC >= 0 && ppu.CYC < 256 && ppu.VISIBLE_SCANLINE {
        // Rendering code can be added here if needed
    }

    if ppu.IO.PPUSTATUS.NMI_OCCURRED == true && ppu.IO.PPUCTRL.GEN_NMI == true {
        ioports.SetNMI(ppu.IO)
        ppu.IO.PPUSTATUS.NMI_OCCURRED = false
    }

    ppu.CYC = ppu.CYC + 1
    if ppu.CYC > 340 { // Corrected cycle count
        ppu.CYC = 0
        ppu.SCANLINE = ppu.SCANLINE + 1

        if ppu.SCANLINE == 241 && ppu.CYC == 0 {
            SetVBLANK(ppu)
            checkKeyboard()
            handleBackground(ppu)
            handleSprite(ppu)
            ShowScreen(ppu)
        }

        if ppu.SCANLINE == 261 && ppu.CYC == 0 {
            ClearVBLANK(ppu)
        }

        if ppu.SCANLINE >= 262 {
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
    ppu.IO.PPUSTATUS.SPRITE_0_BIT = false
}

func initCanvas(ppu *PPU) {
    var winTitle string = "Alphanes"

    if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to initialize SDL: %s\n", err)
        os.Exit(1)
    }


    window, err := sdl.CreateWindow(winTitle, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
        0, 0, sdl.WINDOW_FULLSCREEN_DESKTOP)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to create window: %s\n", err)
        os.Exit(1)
    }

    renderer, err = sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to create renderer: %s\n", err)
        os.Exit(1)
    }


    ppu.texture, err = renderer.CreateTexture(sdl.PIXELFORMAT_ARGB8888, sdl.TEXTUREACCESS_STREAMING, 256, 240)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to create texture: %s\n", err)
        os.Exit(1)
    }

    renderer.SetLogicalSize(256, 240)
}

func attrTable(ppu *PPU) [8][8]byte {
    var result [8][8]byte
    addr := ppu.IO.PPUCTRL.BASE_NAMETABLE_ADDR + 0x3C0

    for y := 0; y < 8; y++ {
        for x := 0; x < 8; x++ {
            absolute_addr := addr + uint16(y*8+x)
            result[y][x] = ReadPPURam(ppu, absolute_addr)
        }
    }
    return result
}

func palForBackground(attr [8][8]byte, x int, y int) byte {
    attr_x := ((x / 4) % 8 + 8) % 8
    attr_y := ((y / 4) % 8 + 8) % 8

    grid := attr[attr_y][attr_x]

    sub_x := ((x / 2) % 2 + 2) % 2
    sub_y := ((y / 2) % 2 + 2) % 2

    shift := (sub_y*2 + sub_x) * 2
    pal := (grid >> shift) & 0x03

    return pal
}

func fetchTile(ppu *PPU, index byte, base_addr uint16) [8][8]byte {
    var result [8][8]byte

    addr := base_addr + uint16(index)*16

    for y := 0; y < 8; y++ {
        tile_addr := addr + uint16(y)
        tile_addr_b := tile_addr + 8

        var a byte = ReadPPURam(ppu, tile_addr)
        var b byte = ReadPPURam(ppu, tile_addr_b)

        for x := 0; x < 8; x++ {
            bit0 := (a >> (7 - x)) & 1
            bit1 := (b >> (7 - x)) & 1
            result[y][x] = (bit1 << 1) | bit0
        }
    }

    return result
}

func fetchNametable(ppu *PPU, x uint16, y uint16) byte {
    x = x % 32
    y = y % 30
    absolute_addr := ppu.IO.PPUCTRL.BASE_NAMETABLE_ADDR + (y * 32) + x
    return ReadPPURam(ppu, absolute_addr)
}

func drawBGTile(ppu *PPU, x int, y int, index byte, base_addr uint16, flipX bool, flipY bool, ignoreZero bool) {
    tile := fetchTile(ppu, index, base_addr)

    wx := ((x / 8) % 32 + 32) % 32
    wy := ((y / 8) % 30 + 30) % 30

    attrpal := attrTable(ppu)
    pal := palForBackground(attrpal, wx*8, wy*8)

    for ky := 0; ky < 8; ky++ {
        for kx := 0; kx < 8; kx++ {
            var ox int = x + kx
            var oy int = y + ky

            if flipX {
                ox = x + (7 - kx)
            }
            if flipY {
                oy = y + (7 - ky)
            }

            if oy < 240 && ox < 256 && oy >= 0 && ox >= 0 {
                colorIndex := tile[ky][kx]
                if ignoreZero && colorIndex == 0 {
                    continue
                }

                paletteIndex := uint16(colorIndex) | uint16(pal)<<2
                colorAddr := uint16(0x3F00) + paletteIndex
                color := ReadPPURam(ppu, colorAddr)

                if colorIndex == 0 {
                    color = ReadPPURam(ppu, 0x3F00)
                }

                WRITE_SCREEN(ppu, ox, oy, int(color))
            }
        }
    }
}

func drawTile(ppu *PPU, x uint16, y uint16, index byte, base_addr uint16, flipX bool, flipY bool, attr byte) {
    tile := fetchTile(ppu, index, base_addr)
    pal := attr & 0x03

    for ky := 0; ky < 8; ky++ {
        for kx := 0; kx < 8; kx++ {
            var ox int = int(x) + kx
            var oy int = int(y) + ky

            if flipX {
                ox = int(x) + (7 - kx)
            }
            if flipY {
                oy = int(y) + (7 - ky)
            }

            if oy < 240 && ox < 256 {
                colorIndex := tile[ky][kx]
                if colorIndex == 0 {
                    continue
                }

                colorAddr := uint16(0x3F10) + uint16(pal)*4 + uint16(colorIndex)
                color := ReadPPURam(ppu, colorAddr)

                WRITE_SCREEN(ppu, ox, oy, int(color))
            }
        }
    }
}

func ShowScreen(ppu *PPU) {
    // Lock the texture to get access to its pixel buffer
    pixels, pitch, err := ppu.texture.Lock(nil)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to lock texture: %s\n", err)
        os.Exit(1)
    }
    defer ppu.texture.Unlock()

    // Convert pixels to a slice for easy access
    pixelData := pixels[:(pitch * 240)] // 240 rows

    // Write pixel data into the texture
    for y := 0; y < 240; y++ {
        for x := 0; x < 256; x++ {
            c := READ_SCREEN(ppu, x, y)
            color := colors[c] // colors is [][]byte

            offset := y*pitch + x*4
            pixelData[offset+0] = color[2] // Blue
            pixelData[offset+1] = color[1] // Green
            pixelData[offset+2] = color[0] // Red
            pixelData[offset+3] = 255      // Alpha
        }
    }

    // Clear the renderer
    renderer.SetDrawColor(0, 0, 0, 255)
    renderer.Clear()

    // Copy the texture to the renderer
    renderer.Copy(ppu.texture, nil, nil)

    // Present the renderer
    renderer.Present()
}


func READ_SCREEN(ppu *PPU, x int, y int) int {
    if x >= 256 || y >= 240 || x < 0 || y < 0 {
        return 0
    }
    return ppu.SCREEN_DATA[x+(y*256)]
}

func WRITE_SCREEN(ppu *PPU, x int, y int, k int) {
    if x >= 256 || y >= 240 || x < 0 || y < 0 {
        return
    }
    ppu.SCREEN_DATA[x+(y*256)] = k
}

func handleBackground(ppu *PPU) {
    if !ppu.IO.PPUMASK.SHOW_BACKGROUND {
        return
    }

    scrollX := int(ppu.IO.PPUSCROLL.X)
    scrollY := int(ppu.IO.PPUSCROLL.Y)

    for tileY := 0; tileY < 30; tileY++ {
        for tileX := 0; tileX <= 32; tileX++ {
            //scrolledX := (tileX*8 - (scrollX % 256)) % 512
            //scrolledY := (tileY*8 - (scrollY % 240)) % 480

            nametableTileX := (scrollX/8 + tileX) % 32
            nametableTileY := (scrollY/8 + tileY) % 30

            tileid := fetchNametable(ppu, uint16(nametableTileX), uint16(nametableTileY))

            screenX := (tileX*8 - (scrollX % 8))
            screenY := (tileY*8 - (scrollY % 8))

            drawBGTile(ppu,
                screenX,
                screenY,
                tileid,
                ppu.IO.PPUCTRL.BACKGROUND_ADDR,
                false,
                false,
                false)
        }
    }
}

func handleSprite(ppu *PPU) {
    if !ppu.IO.PPUMASK.SHOW_SPRITE {
        return
    }

    for s := 0; s < 256; s += 4 {
        pos_y := uint16(ppu.IO.PPU_OAM[s]) + 1 // Sprites are delayed by one scanline
        ind := ppu.IO.PPU_OAM[s+1]
        attr := ppu.IO.PPU_OAM[s+2]
        pos_x := uint16(ppu.IO.PPU_OAM[s+3])

        flipX := attr&0x40 != 0
        flipY := attr&0x80 != 0

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
    if ppu.IO.PPUSTATUS.SPRITE_0_BIT {
        return
    }

    pos_y := uint16(ppu.IO.PPU_OAM[0]) + 1 // Sprites are delayed by one scanline
    ind := ppu.IO.PPU_OAM[1]
    attr := ppu.IO.PPU_OAM[2]
    pos_x := uint16(ppu.IO.PPU_OAM[3])

    flipX := attr&0x40 != 0
    flipY := attr&0x80 != 0

    if y < pos_y || y >= pos_y+8 || x < pos_x || x >= pos_x+8 {
        return
    }

    deltaX := x - pos_x
    deltaY := y - pos_y

    if flipX {
        deltaX = 7 - deltaX
    }
    if flipY {
        deltaY = 7 - deltaY
    }

    sprite_tile := fetchTile(ppu, ind, ppu.IO.PPUCTRL.SPRITE_8_ADDR)
    bg_tile_index := fetchNametable(ppu, x/8, y/8)
    bg_tile := fetchTile(ppu, bg_tile_index, ppu.IO.PPUCTRL.BACKGROUND_ADDR)

    sprite_pixel := sprite_tile[deltaY][deltaX]
    bg_pixel := bg_tile[y%8][x%8]

    if sprite_pixel != 0 && bg_pixel != 0 {
        ppu.IO.PPUSTATUS.SPRITE_0_BIT = true
    }
}
