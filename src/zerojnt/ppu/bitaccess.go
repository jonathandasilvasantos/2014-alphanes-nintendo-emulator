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
package ppu

func H(value uint16) byte {
	return byte( value >> 8)
}

func L(value uint16) byte {
	return byte( (value << 8) >> 8 )
}

func  ReadBit(input byte, pos byte) byte {
	return (input << pos) >> 7
}

func  Bit0(input byte) byte {
	return ReadBit(input, 7)
}

func Bit1(input byte) byte {
	return ReadBit(input, 6)
}

func Bit2(input byte) byte {
	return ReadBit(input, 5)
}

func Bit3(input byte) byte {
	return ReadBit(input, 4)
}

func Bit4(input byte) byte {
	return ReadBit(input, 3)
}

func Bit5(input byte) byte {
	return ReadBit(input, 2)
}

func Bit6(input byte) byte {
	return ReadBit(input, 1)
}

func Bit7(input byte) byte {
	return ReadBit(input, 0)
}

/*func SetBit(input *byte, pos byte, value byte ) {
	if value == 1 {
		*input |= 1 << pos
	} else {
		*input &= ^(1 << pos)
	}
}
*/
func SetBit(input byte, pos byte, value byte ) byte {
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
