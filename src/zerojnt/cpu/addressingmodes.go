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
        reladdr := uint16(RM(cpu, cart, cpu.PC+1))
	if (reladdr & 0x80) != 0{ reladdr |= 0xFF00 }
        return reladdr
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
}

// Zero Page-Y
func ZpY(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    return uint16(  RM(cpu, cart, cpu.PC+1) + cpu.Y )
}


// Indirect - Just used in JMP.
func Ind(cpu *CPU, cart *cartridge.Cartridge) uint16 {

    var lo, hi, lo2, hi2 byte
    var a, a1 uint16

    lo = RM(cpu, cart, cpu.PC+1)
    hi = RM(cpu, cart, cpu.PC+2)
    a =   ( uint16(hi) << 8) | uint16(lo)
  a1 = ( uint16(hi) << 8) | uint16((lo + 1))
  lo2 = RM(cpu, cart, uint16(a))
  hi2 = RM(cpu, cart, a1)
  return (uint16(hi2) << 8) | uint16(lo2)
}

// Indirect Indexed (Pos-indexed)
func IndX(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    var base, lo, hi byte
  base = RM(cpu, cart, cpu.PC+1)
  lo = base + cpu.X
  hi = base + cpu.X + 1
  return   ( uint16(RM(cpu,cart, uint16(hi)))  << 8) | uint16(RM(cpu, cart, uint16(lo)))
}

// Indexed Indirect (Pre-indexed)
func IndY(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    var z, lo, hi byte
    z = RM(cpu, cart, cpu.PC+1)
    lo = z;
    hi = z + 1;
    var plow uint16 = uint16(RM(cpu, cart, uint16(lo)))
    var phigh = uint16(RM(cpu, cart, uint16(hi))) << 8
    return (  phigh | plow ) + uint16(cpu.Y)
}









