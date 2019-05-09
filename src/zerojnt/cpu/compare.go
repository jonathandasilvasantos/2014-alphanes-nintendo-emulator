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
package cpu

import "fmt"
import "strconv"
import "zerojnt/debug"
import "zerojnt/cartridge"
import "log"

func DebugA(cpu *CPU, cart *cartridge.Cartridge) byte {
        debugLine := cpu.D.Lines[cpu.SwitchTimes+1]
	A, errA := strconv.ParseUint( debug.GetA(debugLine), 0, 64 )
        if errA != nil { log.Fatal(errA) }
        return byte(A)
}

func DebugX(cpu *CPU, cart *cartridge.Cartridge) byte {
        debugLine := cpu.D.Lines[cpu.SwitchTimes+1]
	X, errX := strconv.ParseUint( debug.GetX(debugLine), 0, 64 )
        if errX != nil { log.Fatal(errX) }
        return byte(X)
}


func DebugY(cpu *CPU, cart *cartridge.Cartridge) byte {
        debugLine := cpu.D.Lines[cpu.SwitchTimes+1]
	Y, errY := strconv.ParseUint( debug.GetY(debugLine), 0, 64 )
        if errY != nil { log.Fatal(errY) }
        return byte(Y)
}

func DebugP(cpu *CPU, cart *cartridge.Cartridge) byte {
        debugLine := cpu.D.Lines[cpu.SwitchTimes+1]
	P, errP := strconv.ParseUint( debug.GetP(debugLine), 0, 64 )
        if errP != nil { log.Fatal(errP) }
        return byte(P)
}

func DebugOp(cpu *CPU, cart *cartridge.Cartridge) byte {
        debugLine := cpu.D.Lines[cpu.SwitchTimes+1]
	OP, errPC := strconv.ParseUint( debug.GetOpcode(debugLine), 0, 64 )
        if errPC != nil { log.Fatal(errPC) }
        return byte(OP)
}


func DebugCompare(cpu *CPU, cart *cartridge.Cartridge) {
	
        debugLine := cpu.D.Lines[cpu.SwitchTimes]

	A, errA := strconv.ParseUint( debug.GetA(debugLine), 0, 64 )
	X, errX := strconv.ParseUint( debug.GetX(debugLine), 0, 64 )
	Y, errY := strconv.ParseUint( debug.GetY(debugLine), 0, 64 )
	P, errP := strconv.ParseUint( debug.GetP(debugLine), 0, 64 )
	SP, errSP := strconv.ParseUint( debug.GetSP(debugLine), 0, 64 )
	PC, errPC := strconv.ParseUint( debug.GetPC(debugLine), 0, 64 )

	
	if errA != nil { log.Fatal(errA) }
	if errX != nil { log.Fatal(errX) }
	if errY != nil { log.Fatal(errY) }
	if errP != nil { log.Fatal(errP) }
	if errSP != nil { log.Fatal(errSP) }
	if errPC != nil { log.Fatal(errPC) }


	
	var err bool = false
	
	if A != uint64(cpu.A) {
		fmt.Printf("Error: A:%X Debug A:%X\n", uint64(cpu.A), A)
		err = true
	}
	
	if X != uint64(cpu.X) {
		fmt.Printf("Error: X:%X Debug X:%X\n", uint64(cpu.X), X)
		err = true
	}
	
	if Y != uint64(cpu.Y) {
		fmt.Printf("Error: Y:%X Debug Y:%X\n", uint64(cpu.Y), Y)
		err = true
	}
	
	if P != uint64(cpu.P) {
		fmt.Printf("Error: P:%X Debug P:%X\n", uint64(cpu.P), P)
		err = true
	}
	
	if SP != uint64(cpu.SP) {
		fmt.Printf("Error: SP:%X Debug SP:%X\n", uint64(cpu.SP), SP)
		err = true
	}
	
	if PC != uint64(cpu.PC) {
		fmt.Printf("Error: PC:%X Debug PC:%X\n", uint64(cpu.PC), PC)
		err = true
	}

	if err {
            fmt.Printf("Error at line: %d -- %X %X %X - SwitchTime: %d\n", cpu.SwitchTimes, RM(cpu, cart, cpu.PC), RM(cpu, cart, cpu.PC+1), RM(cpu, cart, cpu.PC+2), cpu.SwitchTimes )






		for i := 3; i>0; i-- {
			fmt.Printf("%s\n", cpu.D.Lines[cpu.SwitchTimes-i ])
		}

		fmt.Printf("%s\n", debugLine)
		cpu.Running = false
	}



	
}
