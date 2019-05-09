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


func FlagC(cpu *CPU) byte { return (cpu.P << 7) >> 7}
func FlagZ(cpu *CPU) byte { return (cpu.P << 6) >> 7 }
func FlagI(cpu *CPU) byte { return (cpu.P << 5) >> 7 }
func FlagD(cpu *CPU) byte { return (cpu.P << 4) >> 7 }
func FlagV(cpu *CPU) byte { return (cpu.P << 1) >> 7 }
func FlagN(cpu *CPU) byte { return cpu.P  >> 7 }



func SetP(cpu *CPU, value byte) {
    SetC(cpu, ReadBit(value, 0)) // Carry
    SetZ(cpu, ReadBit(value, 1)) // Zero
    SetI(cpu, ReadBit(value, 2)) // Interrupt
    SetD(cpu, ReadBit(value, 3)) // Decimal
    // bit 4 and 5 have no effects on cpu
    SetV(cpu, ReadBit(value, 6)) // Overflow
    SetN(cpu, ReadBit(value, 7)) // Overflow
    cpu.P = value
}

func SetC(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.P = SetBit(cpu.P, 0, value)
}

func SetZ(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.P = SetBit(cpu.P, 1, value)
}

func SetI(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.P = SetBit(cpu.P, 2, value)
}

func SetD(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.P = SetBit(cpu.P, 3, value)
}

func SetB(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.P = SetBit(cpu.P, 4, value)
}

func SetV(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.P = SetBit(cpu.P, 6, value)
}

func SetN(cpu *CPU, value byte) {
        if (value != 0) { value = 1 }
	cpu.P = SetBit(cpu.P, 7, value)
} 
