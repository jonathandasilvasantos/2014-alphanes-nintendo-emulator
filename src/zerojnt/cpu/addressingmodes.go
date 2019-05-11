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
import "zerojnt/cartridge"

// Relative
func Rel(cpu *CPU, cart *cartridge.Cartridge) uint16 {
        reladdr := RM(cpu, cart, cpu.PC+1)
        addr := int8(reladdr)
        return uint16(addr) + uint16(cpu.PC+2)
}

// Immediate
func Imm(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	return uint16(RM(cpu, cart, cpu.PC+1))
}

// Absolute
func Abs(cpu *CPU, cart *cartridge.Cartridge) uint16 {
        var lo, hi byte
        lo = RM(cpu, cart, cpu.PC+1)
        hi = RM(cpu, cart, cpu.PC+2)
        return  (uint16(hi) << 8) | uint16(lo)
}

// Absolute-X
func AbsX(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	return uint16(Abs(cpu, cart)+ uint16(cpu.X))
}

// Absolute-Y
func AbsY(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	return uint16(Abs(cpu, cart)+ uint16(cpu.Y))
}

// Zero Page
func Zp(cpu *CPU, cart *cartridge.Cartridge) uint16 {
        return uint16(  RM(cpu, cart, cpu.PC+1))
}

// Zero Page-X
func ZpX(cpu *CPU, cart *cartridge.Cartridge) uint16 {
        return uint16(  RM(cpu, cart, cpu.PC+1) + cpu.X )
	//return LE(RM(cpu, cart, cpu.PC+1) + cpu.X, 0)
}

// Zero Page-Y
func ZpY(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    return uint16(  RM(cpu, cart, cpu.PC+1) + cpu.Y )
}



// Indirect - Just used in JMP.
func Ind(cpu *CPU, cart *cartridge.Cartridge) uint16 {

	var a uint16 = uint16 ( LE( RM(cpu, cart, cpu.PC+1), RM(cpu, cart, cpu.PC+2)))
	var l byte = RM(cpu, cart, a)
	var h byte = RM(cpu, cart, a+1)
	if L(a) == 0xFF {
		h = RM(cpu, cart, a-0xFF)
	}
	return LE(l, h)
}

// Indirect Indexed (Pos-indexed)
func IndX(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	var res uint16 = uint16 ( LE( RM(cpu, cart, cpu.PC+1), 0)) + uint16(cpu.X)
	
	var l byte = RM(cpu, cart, res & 0xFF   )
	var h byte = RM(cpu, cart, (res+1) & 0xFF  )
	var target uint16 = LE(l,h)

	return target
}

// Indexed Indirect (Pre-indexed)
func IndY(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	var res uint16 = uint16 ( LE( RM(cpu, cart, cpu.PC+1), 0)) 
	
	var l byte = RM(cpu, cart, res & 0xFF   )
	var h byte = RM(cpu, cart, (res+1) & 0xFF  )
	var target uint16 = LE(l,h)
	

	
	var query uint16 = target 
	var indexed uint16 = query + uint16(cpu.Y)
	cpu.PageCrossed = 0
	if H(query) !=  H(indexed) {
		cpu.PageCrossed = 1
	}

	return indexed
}
