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

func H(value uint16) byte {
	return byte(value >> 8)
}

func L(value uint16) byte {
	return byte((value << 8) >> 8)
}

// ReadBit remains as is for compatibility if used elsewhere,
// but its internal usage by BitN can be optimized away.
func ReadBit(input byte, pos byte) byte {
	// Original implementation kept for direct callers.
	return (input << pos) >> 7
}

// Optimized BitN functions using direct shift and mask.
// Assumes N in BitN refers to the bit index (0=LSB, 7=MSB).

// Bit0 returns the value of bit 0 (Least Significant Bit).
func Bit0(input byte) byte {
	return input & 1
}

// Bit1 returns the value of bit 1.
func Bit1(input byte) byte {
	return (input >> 1) & 1
}

// Bit2 returns the value of bit 2.
func Bit2(input byte) byte {
	return (input >> 2) & 1
}

// Bit3 returns the value of bit 3.
func Bit3(input byte) byte {
	return (input >> 3) & 1
}

// Bit4 returns the value of bit 4.
func Bit4(input byte) byte {
	return (input >> 4) & 1
}

// Bit5 returns the value of bit 5.
func Bit5(input byte) byte {
	return (input >> 5) & 1
}

// Bit6 returns the value of bit 6.
func Bit6(input byte) byte {
	return (input >> 6) & 1
}

// Bit7 returns the value of bit 7 (Most Significant Bit).
func Bit7(input byte) byte {
	return (input >> 7) & 1
}

func SetBit(input byte, pos byte, value byte) byte {
	var b byte = input
	if value == 1 {
		b |= 1 << pos
	} else {
		b &= ^(1 << pos)
	}
	return b
}

func LE(a byte, b byte) uint16 {
    var x uint16 = uint16(a)
    var y uint16 = uint16(b)
    return (y << 8) | x
}