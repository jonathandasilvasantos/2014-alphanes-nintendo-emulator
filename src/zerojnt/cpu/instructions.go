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

//This instruction adds the contents of a memory location to the accumulator together with the carry bit. If overflow occurs the carry bit is set, this enables multiple byte addition to be performed.
func iADC (cpu *CPU, value uint16) {
	var tmp uint16 = uint16(cpu.A) + value
	if(FlagC(cpu) == 1) {
		tmp++
	}
	
	CarryFlag(cpu, tmp)
	NegativeFlag(cpu, tmp)

	var M byte = byte(cpu.A)
	var N byte = byte(value)
	var result byte = byte(tmp)


	if (((M^N) & 0x80) == 0 ) && (((M^result) & 0x80) != 0)  {
		SetV(cpu, 1)
	} else {
		SetV(cpu, 0)
	}

	ZeroFlag(cpu, tmp)
	cpu.A = byte(tmp)
}


//This instruction adds the contents of a memory location to the accumulator together with the carry bit. If overflow occurs the carry bit is set, this enables multiple byte addition to be performed.
func ADC (cpu *CPU, value uint16) {

    var sum, j, k, c6, c7 byte
    //b := RM(cpu, cart value)
    b := byte(value)
    j = (b >> 7) & 0x1
    k = (cpu.A >> 7) & 0x1
    sum = cpu.A + b + FlagC(cpu)
    c6 = j ^ k ^ ((sum >> 7) & 0x1);
    c7 = (j & k) | (j & c6) | (k & c6);
    SetC(cpu, c7)
    SetV(cpu, c6 ^ c7)
    ZeroFlag(cpu, uint16(sum))
     SetN(cpu, (j ^ k ^ c6))
    cpu.A = sum


}




// A logical AND is performed, bit by bit, on the accumulator contents using the contents of a byte of memory.
func AND (cpu *CPU, value uint16) {
	cpu.A = cpu.A & byte(value)
	ZeroFlag(cpu, uint16 (cpu.A))
	SetN(cpu, ((cpu.A >> 7) & 1))
}

// This operation shifts all the bits of the accumulator or memory contents one bit left. Bit 0 is set to 0 and bit 7 is placed in the carry flag. The effect of this operation is to multiply the memory contents by 2 (ignoring 2's complement considerations), setting the carry if the result will not fit in 8 bits.
func ASL (cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	
	if RM(cpu, cart, cpu.PC) == 0x0A {
		SetC(cpu, Bit7(cpu.A))
		cpu.A = cpu.A << 1
		ZeroFlag(cpu, uint16(cpu.A))
	        SetN(cpu, ((cpu.A >> 7) & 1))
		return
	}

	var tmp uint16 = uint16(RM(cpu, cart, value))
	SetC(cpu, Bit7(byte(tmp)))
	tmp = tmp << 1
	ZeroFlag(cpu, tmp)
	SetN(cpu, ((byte(tmp) >> 7) & 1))
	WM(cpu, cart, value, byte(tmp))
}

// If the carry flag is clear then add the relative displacement to the program counter to cause a branch to a new location.
func BCC(cpu *CPU, value uint16) {
    cpu.CYCSpecial = 0;

    if (FlagC(cpu)) == 0 {
        Branch(cpu, value)
        return
    }
    cpu.PC += 2
}

// If the carry flag is set then add the relative displacement to the program counter to cause a branch to a new location.
func BCS(cpu *CPU, value uint16) {
    cpu.CYCSpecial = 0;

    if (FlagC(cpu)) == 1 {
        Branch(cpu, value)
        return
    }
    cpu.PC += 2
}

// If the zero flag is set then add the relative displacement to the program counter to cause a branch to a new location.
func BEQ(cpu *CPU, value uint16) {
	cpu.CYCSpecial = 0;

    if (FlagZ(cpu)) == 1 {
        Branch(cpu, value)
        return
    }
    cpu.PC += 2
}

// This instructions is used to test if one or more bits are set in a target memory location. The mask pattern in A is ANDed with the value in memory to set or clear the zero flag, but the result is not kept. Bits 7 and 6 of the value from memory are copied into the N and V flags.
func BIT(cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	var v byte = RM(cpu, cart, value)
	var b byte = cpu.A & v
	ZeroFlag(cpu, uint16(b))
	SetN(cpu, Bit7(v))
	SetV(cpu, Bit6(v))
}

// If the negative flag is set then add the relative displacement to the program counter to cause a branch to a new location.
func BMI(cpu *CPU, value uint16) {
    cpu.CYCSpecial = 0;

    if (FlagN(cpu)) == 1 {
        Branch(cpu, value)
        return
    }
    cpu.PC += 2
}

// If the zero flag is clear then add the relative displacement to the program counter to cause a branch to a new location.
func BNE(cpu *CPU, value uint16) {
    cpu.CYCSpecial = 0;

    if (FlagZ(cpu)) == 0 {
        Branch(cpu, value)
        return
    }
    cpu.PC += 2
}



// If the negative flag is clear then add the relative displacement to the program counter to cause a branch to a new location.
func BPL(cpu *CPU, value uint16) {
    cpu.CYCSpecial = 0;

    if (FlagN(cpu)) == 0 {
        Branch(cpu, value)
        return
    }
    cpu.PC += 2
}


// The BRK instruction forces the generation of an interrupt request. The program counter and processor status are pushed on the stack then the IRQ interrupt vector at $FFFE/F is loaded into the PC and the break flag in the status set to one.
func BRK(cpu *CPU, cart *cartridge.Cartridge) {
        PushWord(cpu, cpu.PC)
	PushMemory (cpu, cpu.P)
	cpu.PC = LE( RM(cpu, cart, 0xFFFE), RM(cpu, cart, 0xFFFF))
	SetB(cpu, 1)
}


// If the overflow flag is clear then add the relative displacement to the program counter to cause a branch to a new location.
func BVC(cpu *CPU, value uint16) {
    cpu.CYCSpecial = 0;

    if (FlagV(cpu)) == 0 {
        Branch(cpu, value)
        return
    }
    cpu.PC += 2
}


// If the overflow flag is set then add the relative displacement to the program counter to cause a branch to a new location.
func BVS(cpu *CPU, value uint16) {
    cpu.CYCSpecial = 0;

    if (FlagV(cpu)) == 1 {
        Branch(cpu, value)
        return
    }
    cpu.PC += 2
}

// This instruction compares the contents of the accumulator with another memory held value and sets the zero and carry flags as appropriate.
func CMP(cpu *CPU, value uint16) {
	var tmp uint16 = uint16(cpu.A) - value

	SetN(cpu, ((byte(tmp) >> 7) & 1))
	SetC(cpu, 0)
	SetZ(cpu, 0)
	if uint16(cpu.A) >= value {
		SetC(cpu, 1)
	}
	if uint16(cpu.A) == value {
		SetZ(cpu, 1)
	}
}


// Set the carry flag to zero
func CLC(cpu *CPU) {
	SetC(cpu, 0)
}

// Sets the decimal mode flag to zero.
func CLD(cpu *CPU) {
	SetD(cpu, 0)
}

// Clears the interrupt disable flag allowing normal interrupt requests to be serviced.
func CLI(cpu *CPU) {
	SetI(cpu, 0)
}

// Clears the overflow flag.
func CLV(cpu *CPU) {
	SetV(cpu, 0)
}

// This instruction compares the contents of the X register with another memory held value and sets the zero and carry flags as appropriate.
func CPX (cpu *CPU, value uint16) {
	var tmp byte = cpu.X - byte(value)
	
	if cpu.X >= byte(value) {
		SetC(cpu, 1)
	} else {
		SetC(cpu, 0)
	}
	if cpu.X == byte(value) {
		SetZ(cpu, 1)
	} else {
		SetZ(cpu, 0)
	}
	SetN(cpu, ((byte(tmp) >> 7) & 1))
}

// This instruction compares the contents of the Y register with another memory held value and sets the zero and carry flags as appropriate.
func CPY (cpu *CPU, value uint16) {
	var tmp byte = cpu.Y - byte(value)
	if cpu.Y >= byte(value) {
		SetC(cpu, 1)
	} else {
		SetC(cpu, 0)
	}
	if cpu.Y == byte(value) {
		SetZ(cpu, 1)
	} else {
		SetZ(cpu, 0)
	}
	SetN(cpu, ((byte(tmp) >> 7) & 1))
}

// Subtracts one from the value held at a specified memory location setting the zero and negative flags as appropriate.
func DEC (cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	var tmp byte = RM(cpu, cart, value)
	tmp--
	WM(cpu, cart, value, tmp)
	
	ZeroFlag(cpu, uint16(tmp))
	SetN(cpu, ((byte(tmp) >> 7) & 1))
}

// Subtracts one from the X register setting the zero and negative flags as appropriate.
func DEX(cpu *CPU) {
	cpu.X--
	ZeroFlag(cpu, uint16(cpu.X))
        SetN(cpu, ((cpu.X >> 7) & 1))
}

// Subtracts one from the Y register setting the zero and negative flags as appropriate.
func DEY(cpu *CPU) {
	cpu.Y--
	ZeroFlag(cpu, uint16(cpu.Y))
        SetN(cpu, ((cpu.Y >> 7) & 1))
}

// An exclusive OR is performed, bit by bit, on the accumulator contents using the contents of a byte of memory.
func EOR (cpu *CPU, value uint16) {
	cpu.A = cpu.A ^ byte(value)
	ZeroFlag(cpu, uint16(cpu.A))
        SetN(cpu, ((cpu.A >> 7) & 1))
}

// Adds one to the value held at a specified memory location setting the zero and negative flags as appropriate.
func INC (cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	
	var tmp byte = RM(cpu, cart, value)
	tmp++
	WM(cpu, cart, value, tmp)
	
	ZeroFlag(cpu, uint16(tmp))
	SetN(cpu, ((byte(tmp) >> 7) & 1))
}

// Adds one to the X register setting the zero and negative flags as appropriate.
func INX (cpu *CPU) {
	cpu.X++
	ZeroFlag(cpu, uint16(cpu.X))
	SetN(cpu, ((byte(cpu.X) >> 7) & 1))
}


// Adds one to the Y register setting the zero and negative flags as appropriate.
func INY (cpu *CPU) {
	cpu.Y++
	ZeroFlag(cpu, uint16(cpu.Y))
	SetN(cpu, ((byte(cpu.Y) >> 7) & 1))
}


// Sets the program counter to the address specified by the operand.
func JMP(cpu *CPU, value uint16) {
	cpu.PC = value
}

// The JSR instruction pushes the address (minus one) of the return point on to the stack and then sets the program counter to the target memory address.
func JSR(cpu *CPU, value uint16) {
        PushWord(cpu, cpu.PC+3)
	cpu.PC = value
}

// Loads a byte of memory into the accumulator setting the zero and negative flags as appropriate.
func LDA(cpu *CPU, value uint16) {
	cpu.A = byte(value)
	ZeroFlag(cpu, value)
	SetN(cpu, ((cpu.A >> 7) & 1) )
}

// Loads a byte of memory into the X register setting the zero and negative flags as appropriate.
func LDX(cpu *CPU, value uint16) {
	cpu.X = byte(value)
	ZeroFlag(cpu, value)
        SetN(cpu, ((cpu.X >> 7) & 1))
}

// Loads a byte of memory into the Y register setting the zero and negative flags as appropriate.
func LDY (cpu *CPU, value uint16) {
	cpu.Y = byte(value)
	ZeroFlag(cpu, value)
        SetN(cpu, ((cpu.Y >> 7) & 1))
}

// Each of the bits in A or M is shift one place to the right. The bit that was in bit 0 is shifted into the carry flag. Bit 7 is set to zero.
func LSR (cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	
	if RM(cpu, cart, cpu.PC) == 0x4A {
		SetC(cpu, Bit0(cpu.A))
		cpu.A = cpu.A >> 1
		ZeroFlag(cpu, uint16(cpu.A))
	        SetN(cpu, ((byte(cpu.A) >> 7) & 1))
		return
	}
	
	var tmp byte = RM(cpu, cart, value)
	SetC(cpu, Bit0(tmp))
	tmp = tmp >> 1
	ZeroFlag(cpu, uint16(tmp))
	SetN(cpu, ((byte(tmp) >> 7) & 1))
	WM(cpu, cart, value, tmp)
}


// The NOP instruction causes no changes to the processor other than the normal incrementing of the program counter to the next instruction.
func NOP() {
}

// An inclusive OR is performed, bit by bit, on the accumulator contents using the contents of a byte of memory.
func ORA (cpu *CPU, value uint16) {
	cpu.A = cpu.A | byte(value)
	ZeroFlag(cpu, uint16(cpu.A))
        SetN(cpu, ((cpu.A >> 7) & 1))
}

// Pulls an 8 bit value from the stack and into the accumulator. The zero and negative flags are set as appropriate.
func PLA (cpu *CPU) {
	cpu.A = PopMemory(cpu)
	ZeroFlag(cpu, uint16(cpu.A))
        SetN(cpu, ((cpu.A >> 7) & 1))
	
}

// Pushes a copy of the accumulator on to the stack.
func PHA (cpu *CPU) {
	PushMemory(cpu, cpu.A)
}


// C824  48        PHA                             A:FF X:00 Y:00 P:AD SP:FB CYC: 86 SL:243
// C825  28        PLP                             A:FF X:00 Y:00 P:AD SP:FA CYC: 95 SL:243
// C826  D0 09     BNE $C831                       A:FF X:00 Y:00 P:EF SP:FB CYC:107 SL:243

// Pushes a copy of the status flags on to the stack.
func PHP (cpu *CPU) {
	PushMemory(cpu, SetBit(SetBit(cpu.P, 4, 1), 5, 1) )
}

// Pulls an 8 bit value from the stack and into the processor flags. The flags will take on new states as determined by the value pulled.
func PLP(cpu *CPU) {
	var all byte = PopMemory(cpu)
	var b4 = Bit4(cpu.P)
	var b5 = Bit5(cpu.P)
        newP := all
	newP = SetBit(newP, 4, b4)
	newP = SetBit(newP, 5, b5)
        SetP(cpu, newP)
}

// Move each of the bits in either A or M one place to the left. Bit 0 is filled with the current value of the carry flag whilst the old bit 7 becomes the new carry flag value.
func ROL (cpu *CPU, cart *cartridge.Cartridge, value uint16, op byte) {


    switch(op) {

    case 0x26:  // Zp
        var result uint16 = uint16(RM(cpu, cart, value))
        var tmp = (result >> 7) & 0x1
        result = (result << 1) | uint16(FlagC(cpu))
        WM(cpu, cart, value, byte(result))
        ZeroFlag(cpu, result)
	SetN(cpu, (( byte(result)  >> 7) & 1))
        SetC(cpu, byte(tmp))
        break

    case 0x2A:  // Acc
        
        var tmp byte = (cpu.A >> 7) & 0x1
        cpu.A = (cpu.A << 1) | FlagC(cpu)
        SetC(cpu, tmp)
        ZeroFlag(cpu, uint16(cpu.A))
	SetN(cpu, (( byte(cpu.A)  >> 7) & 1))
        break
    }
}

// Move each of the bits in either A or M one place to the right. Bit 7 is filled with the current value of the carry flag whilst the old bit 0 becomes the new carry flag value.
func ROR (cpu *CPU, cart *cartridge.Cartridge, value uint16, op byte) {

    switch(op) {

    case 0x66:

        var result uint16 = uint16(RM(cpu, cart, value))
        tmp := (result & 0x1)
        result = (result >> 1) | (uint16(FlagC(cpu)) << 7)
        SetC(cpu, byte(tmp))
        ZeroFlag(cpu, result)
	SetN(cpu, (( byte(result)  >> 7) & 1))
        WM(cpu, cart, value, byte(result))
        break

    case 0x6A:  // Acc
        var tmp byte =  cpu.A & 0x1
        cpu.A = (cpu.A >> 1) | (FlagC(cpu) << 7)
        SetC(cpu, tmp)
        ZeroFlag(cpu, uint16(cpu.A))
	SetN(cpu, (( byte(cpu.A)  >> 7) & 1))
        break
    }
}


// The RTI instruction is used at the end of an interrupt processing routine. It pulls the processor flags from the stack followed by the program counter.
func RTI(cpu *CPU) {

        SetP(cpu, PopMemory(cpu))
	cpu.PC = PopWord(cpu)
}

// The RTS instruction is used at the end of a subroutine to return to the calling routine. It pulls the program counter (minus one) from the stack.
func RTS (cpu *CPU) {
	cpu.PC = PopWord(cpu)
}

// This instruction subtracts the contents of a memory location to the accumulator together with the not of the carry bit. If overflow occurs the carry bit is clear, this enables multiple byte subtraction to be performed.

// Obs: sbc(x) = adc(255-x)
func SBC (cpu *CPU, value uint16) {
	var tmp uint16 = uint16(cpu.A) + (255 - value)
	if(FlagC(cpu) == 1) {
		tmp++
	}
	
	CarryFlag(cpu, tmp)
        SetN(cpu, ((byte(tmp) >> 7) & 1))

	var M byte = byte(cpu.A)
	var N byte = byte(255-value)
	var result byte = byte(tmp)


	if (((M^N) & 0x80) == 0 ) && (((M^result) & 0x80) != 0)  {
		SetV(cpu, 1)
	} else {
		SetV(cpu, 0)
	}

	ZeroFlag(cpu, tmp)
	cpu.A = byte(tmp)
}


// Set the carry flag to one.
func SEC(cpu *CPU) {
	SetC(cpu, 1)
}

// Set the decimal mode flag to one.
func SED(cpu *CPU) {
	SetD(cpu, 1)
}

// Set the interrupt disable flag to one.
func SEI(cpu *CPU) {
	SetI(cpu, 1)
}

// Stores the contents of the A register into memory
func STA (cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	WM(cpu, cart, value, cpu.A)
}

// Stores the contents of the X register into memory
func STX (cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	WM(cpu, cart, value, cpu.X)
}

// Stores the contents of the Y register into memory
func STY (cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	WM(cpu, cart, value, cpu.Y)
}


//Copies the current contents of the accumulator into the X register and sets the zero and negative flags as appropriate.
func TAX (cpu *CPU) {
	cpu.X = cpu.A
	ZeroFlag(cpu, uint16(cpu.X))
        SetN(cpu, ((cpu.X >> 7) & 1))
}

// Copies the current contents of the accumulator into the Y register and sets the zero and negative flags as appropriate.
func TAY (cpu *CPU) {
	cpu.Y = cpu.A
	ZeroFlag(cpu, uint16(cpu.Y))
        SetN(cpu, ((cpu.Y >> 7) & 1))
}

// Copies the current contents of the stack register into the X register and sets the zero and negative flags as appropriate.
func TSX (cpu *CPU) {
	cpu.X = cpu.SP
	ZeroFlag(cpu, uint16(cpu.X))
        SetN(cpu, ((cpu.X >> 7) & 1))
}

// Copies the current contents of the X register into the accumulator and sets the zero and negative flags as appropriate.
func TXA (cpu *CPU) {
	cpu.A = cpu.X
	ZeroFlag(cpu, uint16(cpu.A))
        SetN(cpu, ((cpu.A >> 7) & 1))
}

// Copies the current contents of the X register into the stack register.
func TXS (cpu *CPU) {
	cpu.SP = cpu.X
}

// Copies the current contents of the Y register into the accumulator and sets the zero and negative flags as appropriate.
func TYA (cpu *CPU) {
	cpu.A = cpu.Y
	ZeroFlag(cpu, uint16(cpu.A))
        SetN(cpu, ((cpu.A >> 7) & 1))
}
