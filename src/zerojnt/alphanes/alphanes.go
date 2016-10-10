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
package main

import "zerojnt/cartridge"
import "zerojnt/cpu"
import "zerojnt/ppu"
import "zerojnt/ioports"
import "zerojnt/debug"
import "fmt"
import "os"

	 
	 type Emulator struct {
	 	Running bool
	 }

	 var Cart cartridge.Cartridge
	 var Nescpu cpu.CPU
	 var Nesppu ppu.PPU
	 var Nesio ioports.IOPorts
	 var Debug debug.Debug
	 var Alphanes Emulator
    
    func main() {

	
		fmt.Println("Loading " + os.Args[1])
		Cart = cartridge.LoadRom(os.Args[1])
	
		if len(os.Args) >= 3 {
			fmt.Printf("Debug mode is on\n")
			Debug = debug.OpenDebugFile(os.Args[2])
		} else {
			Debug.Enable = false
			fmt.Printf("Debug mode is off\n")
		}


	
		Nescpu = cpu.StartCPU()
		Nescpu.IO = ioports.StartIOPorts(&Cart)
		Nescpu.D = Debug
		Nescpu.D.Verbose = true
		cpu.SetResetVector(&Nescpu, &Cart)

		Nesppu = ppu.StartPPU(&Nescpu.IO)
		Nescpu.D.CURRENT_PPU = &Nesppu
		
		
		Alphanes.Running = true		
		emulate()
	
		
		
}

func emulate() {

	var ppudelay = 0

	for Alphanes.Running == true && Nescpu.Running == true {
		
		cpu.Process(&Nescpu, &Cart)
				
		if ppudelay < 30000 {
			ppudelay = ppudelay + 1
		} else {
			for x := 0; x < 3; x++ {
				ppu.Process(&Nesppu, &Cart)
			}
		}
		
	}
}