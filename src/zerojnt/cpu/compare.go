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

func DebugCompare(cpu *CPU, cart *cartridge.Cartridge) {
	
	A, errA := strconv.ParseUint( debug.GetA(cpu.D.Lines[cpu.SwitchTimes]), 0, 64 )
	X, errX := strconv.ParseUint( debug.GetX(cpu.D.Lines[cpu.SwitchTimes]), 0, 64 )
	Y, errY := strconv.ParseUint( debug.GetY(cpu.D.Lines[cpu.SwitchTimes]), 0, 64 )
	P, errP := strconv.ParseUint( debug.GetP(cpu.D.Lines[cpu.SwitchTimes]), 0, 64 )
	SP, errSP := strconv.ParseUint( debug.GetSP(cpu.D.Lines[cpu.SwitchTimes]), 0, 64 )
	OP, errOP := strconv.ParseUint( debug.GetOp(cpu.D.Lines[cpu.SwitchTimes]), 0, 64 )
	
	if errA != nil { log.Fatal(errA) }
	if errX != nil { log.Fatal(errX) }
	if errY != nil { log.Fatal(errY) }
	if errP != nil { log.Fatal(errP) }
	if errSP != nil { log.Fatal(errSP) }
	if errOP != nil { log.Fatal(errOP) }
	
	
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
	
	if OP != uint64(cpu.PC) {
		fmt.Printf("Error: PC:%X Debug PC:%X\n", uint64(cpu.PC), OP)
		err = true
	}

	if err {
		fmt.Printf("Error at line: %d -- %X %X %X\n", cpu.SwitchTimes, RM(cpu, cart, cpu.PC), RM(cpu, cart, cpu.PC+1), RM(cpu, cart, cpu.PC+2) )






		for i := 3; i>0; i-- {
			fmt.Printf("%s\n", cpu.D.Lines[cpu.SwitchTimes-i ])
		}

		fmt.Printf("%s\n", cpu.D.Lines[cpu.SwitchTimes])
		cpu.Running = false
	}



	
}