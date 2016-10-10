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
package cartridge

import "fmt"
import "os"
import "log"
import "bufio"

type Header struct {
	
	ID [4]byte
	ROM_SIZE byte // PRG Size
	VROM_SIZE byte
	ROM_TYPE byte // Type 1
	ROM_TYPE2 byte // Type 2
	ROM_BLANK [8]byte
	RomType RomType
}

type Cartridge struct {
	Header Header
	Data []byte
	PRG []byte
	CHR []byte
}

type RomType struct {
	Mapper int
	HorizontalMirroring bool
	VerticalMirroring bool
	SRAM bool
	Trainer bool // 512-bytes trainer present
	FourScreenVRAM bool
}

func LoadRom(Filename string) Cartridge {
	
	fmt.Println("Loading rom...")
	
	var cart Cartridge
	
	file, err := os.Open(Filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	
	info, err := file.Stat()
	if err != nil {
		log.Fatal(err)
	}
	
	var size int64 = info.Size()
	cart.Data = make([]byte, size)
	
	buffer := bufio.NewReader(file)
	_, err = buffer.Read(cart.Data)
	
LoadHeader(&cart.Header, cart.Data)
LoadPRG(&cart)
LoadCHR(&cart)

return cart
}

func LoadHeader(h *Header, b []byte) {
	
	fmt.Println("Loading header...")
	
	// Dump the NES+1A
	var step int = 0;
	for i := 0; i < 4; i++ {
		h.ID[i] = b[i]
		step = i
	}

	// PGR Size (ROM)
	step++
	h.ROM_SIZE = b[step]
	fmt.Println("PRG size: ", h.ROM_SIZE, " x 16384 = ", int(h.ROM_SIZE)*16384, " bytes =", (int(h.ROM_SIZE)*16384)/1024, "kbs")
	
	
	// CHR Size (VROM)
	step++
	h.VROM_SIZE = b[step]
	fmt.Println("CHR size: ", h.VROM_SIZE, " x 8192 = ", int(h.VROM_SIZE)*8192, " bytes =", (int(h.VROM_SIZE)*8192)/1024, "kbs")
	
	
	// Rom Type Control First Byte
	step++
	h.ROM_TYPE = b[step]
	
	// Rom Type Control Second Byte
	step++
	h.ROM_TYPE2 = b[step]
	
	// Reserved space in the header
	step++
	for i := 0; i < 8; i++ {
		h.ROM_BLANK[i] = b[step]
		step++
	}
	
	TranslateRomType(h)
}

func TranslateRomType(h *Header) {
	var sixbyte byte = h.ROM_TYPE
	var sevenbyte byte = h.ROM_TYPE2
	fmt.Printf("Header %b | %b\n", sixbyte, sevenbyte)
	fmt.Printf("Header %x | %x\n", sixbyte, sevenbyte)
	
	// Here we apply the iNES Extended specification for mappers >= 16
	var m1 byte =  (sixbyte & 0xF0) >> 4
	var m2 byte =  (sevenbyte & 0xF0) >> 4
	var m3 byte = (m2 << 4) | m1
	h.RomType.Mapper = int(m3)
	
	// Print the mapper
	fmt.Println("Mapper ", h.RomType.Mapper)
	
	
	var mirroring byte = (sixbyte << 7) >> 7
	h.RomType.HorizontalMirroring = mirroring == 0
	h.RomType.VerticalMirroring = mirroring != 0

	// Print in console the mirroring option
	if h.RomType.HorizontalMirroring {
		fmt.Println("Horizontal Mirroring")
	} else {
		fmt.Println("Vertical Mirroring")
	}
	
	var sram byte = (sixbyte << 6) >> 7
	h.RomType.SRAM = sram != 0
	if h.RomType.SRAM {
		fmt.Println("SRAM enabled")
	}
	
	var trainer byte = (sixbyte << 5) >> 7
	h.RomType.Trainer = trainer != 0
	if h.RomType.Trainer {
		fmt.Println("512-bytes Trainer Present")
	}
	
	var fourscreenvram = (sixbyte << 4) >> 7
	h.RomType.FourScreenVRAM = fourscreenvram != 0
	if h.RomType.FourScreenVRAM {
		fmt.Println("Four Screen VRAM enabled")
	}
}

func LoadPRG(c *Cartridge) {

	var page16bits = 16384
	var size int = int(c.Header.ROM_SIZE)*page16bits

	c.PRG = make([]byte, size)	
	for i := 0; i < size; i++ {
		c.PRG[i] = c.Data[i+16]
	}
}

func LoadCHR(c *Cartridge) {

	var page8bits = 8192
	var page16bits = 16384
	var size int = int(c.Header.VROM_SIZE)*page8bits
	var prgsize int = int(c.Header.ROM_SIZE)*page16bits
	var offset int = 16 + prgsize
	fmt.Printf("CHR Size: %x\n",size)
	
	c.CHR = make([]byte, size)
	for i := 0; i < size; i++ {
		c.CHR[i] = c.Data[i+offset]
	}
}