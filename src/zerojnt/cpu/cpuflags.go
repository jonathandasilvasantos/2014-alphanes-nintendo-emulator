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
package cpu

func UpdateStatus(cpu *CPU) {
	
	var s byte = cpu.P
	cpu.Flags.C = Bit0(s) 
	cpu.Flags.Z = Bit1(s) 
	cpu.Flags.I = Bit2(s) 
	cpu.Flags.D = Bit3(s) 
	cpu.Flags.B = Bit4(s) 
	// bit-5 Aways set
	cpu.Flags.V = Bit6(s) 
	cpu.Flags.N = Bit7(s)
}

func SetC(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.Flags.C = value
	cpu.P = SetBit(cpu.P, 0, value)
}

func SetZ(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.Flags.Z = value
	cpu.P = SetBit(cpu.P, 1, value)
}

func SetI(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.Flags.I = value
	cpu.P = SetBit(cpu.P, 2, value)
}

func SetD(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.Flags.D = value
	cpu.P = SetBit(cpu.P, 3, value)
}

func SetB(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.Flags.B = value
	cpu.P = SetBit(cpu.P, 4, value)
}

func SetV(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.Flags.V = value
	cpu.P = SetBit(cpu.P, 6, value)
}

func SetN(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.Flags.N = value
	cpu.P = SetBit(cpu.P, 7, value)
} 
