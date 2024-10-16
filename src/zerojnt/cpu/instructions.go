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


// A logical AND is performed, bit by bit, on the accumulator contents using the contents of a byte of memory.
func AND (cpu *CPU, value uint16) {
	cpu.A = cpu.A & byte(value)
	ZeroFlag(cpu, uint16 (cpu.A))
	SetN(cpu, ((cpu.A >> 7) & 1))
}

// This operation shifts all the bits of the accumulator or memory contents one bit left. Bit 0 is set to 0 and bit 7 is placed in the carry flag. The effect of this operation is to multiply the memory contents by 2 (ignoring 2's complement considerations), setting the carry if the result will not fit in 8 bits.
// ASL on Accumulator
func ASL_A(cpu *CPU) {
    SetC(cpu, Bit7(cpu.A))          // Set Carry Flag to bit 7 of A
    cpu.A = cpu.A << 1               // Shift A left by 1
    ZeroFlag(cpu, uint16(cpu.A))     // Set Zero Flag
    SetN(cpu, (cpu.A>>7)&1)          // Set Negative Flag based on new bit 7
}

// ASL on Memory
func ASL_M(cpu *CPU, cart *cartridge.Cartridge, address uint16) {
    value := RM(cpu, cart, address)   // Read memory value
    SetC(cpu, Bit7(value))           // Set Carry Flag to bit 7 of value
    shifted := value << 1             // Shift left by 1
    WM(cpu, cart, address, shifted)   // Write back to memory
    ZeroFlag(cpu, uint16(shifted))    // Set Zero Flag
    SetN(cpu, (shifted>>7)&1)         // Set Negative Flag based on new bit 7
}

// ADC (Add with Carry) instruction
func ADC(cpu *CPU, value byte) {
    // Convert registers and value to uint16 for addition to handle overflow
    a := uint16(cpu.A)
    m := uint16(value)
    c := uint16( FlagC(cpu)   ) // Assume GetCarryFlag returns 0 or 1

    // Perform the addition with carry
    sum := a + m + c
    result := byte(sum & 0xFF) // Keep only the lower 8 bits

    // Set Carry flag if sum exceeds 255
    if sum > 0xFF {
        SetC(cpu, 1)
    } else {
        SetC(cpu, 0)
    }

    // Set Zero flag if result is zero
    if result == 0 {
        SetZ(cpu, 1)
    } else {
        SetZ(cpu, 0)
    }

    // Set Negative flag based on bit 7 of the result
    SetN(cpu, (result>>7)&1)

    // Set Overflow flag
    // Overflow occurs if the sign bits of A and M are the same,
    // but the sign bit of the result is different
    if (((cpu.A ^ value) & 0x80) == 0) && (((cpu.A ^ result) & 0x80) != 0) {
        SetV(cpu, 1)
    } else {
        SetV(cpu, 0)
    }

    // Update the Accumulator with the result
    cpu.A = result
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


// Loads a byte of memory into the accumulator setting the zero and negative flags as appropriate.
func LDA(cpu *CPU, value uint16) {
	cpu.A = byte(value)
	ZeroFlag(cpu, uint16(cpu.A))
	SetN(cpu, ((cpu.A >> 7) & 1) )
}

// Loads a byte of memory into the X register setting the zero and negative flags as appropriate.
func LDX(cpu *CPU, value uint16) {
	cpu.X = byte(value)
	ZeroFlag(cpu, uint16(cpu.X))
        SetN(cpu, ((cpu.X >> 7) & 1))
}

// Loads a byte of memory into the Y register setting the zero and negative flags as appropriate.
func LDY (cpu *CPU, value uint16) {
	cpu.Y = byte(value)
	ZeroFlag(cpu, uint16(cpu.Y))
        SetN(cpu, ((cpu.Y >> 7) & 1))
}


// LSR_A performs Logical Shift Right on the Accumulator.
// It shifts all bits in A one position to the right.
// Bit 0 is moved into the Carry flag, and bit 7 is set to 0.
// Flags affected: C, Z, N
func LSR_A(cpu *CPU) {
    // Set Carry flag to bit 0 before shifting
    SetC(cpu, cpu.A & 0x01)

    // Shift Accumulator right by 1
    cpu.A >>= 1

    // Set Zero flag if result is zero
    ZeroFlag(cpu, uint16(cpu.A))

    // Clear Negative flag since bit 7 is always 0 after LSR
    SetN(cpu, 0)
}

// LSR_M performs Logical Shift Right on a memory address.
// It shifts all bits in the memory location one position to the right.
// Bit 0 is moved into the Carry flag, and bit 7 is set to 0.
// Flags affected: C, Z, N
func LSR_M(cpu *CPU, cart *cartridge.Cartridge, address uint16) {
    // Read the current value from memory
    value := RM(cpu, cart, address)

    // Set Carry flag to bit 0 before shifting
    SetC(cpu, value & 0x01)

    // Shift right by 1
    shifted := value >> 1

    // Write the shifted value back to memory
    WM(cpu, cart, address, shifted)

    // Set Zero flag if result is zero
    ZeroFlag(cpu, uint16(shifted))

    // Clear Negative flag since bit 7 is always 0 after LSR
    SetN(cpu, 0)
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
// Pulls an 8-bit value from the stack and into the processor flags.
// Ensures that Bit 5 is always set and Bit 4 (Break) is cleared.
func PLP(cpu *CPU) {
    var all byte = PopMemory(cpu)
    // Clear Bit 4 (Break Flag) and ensure Bit 5 is set
    newP := (all & 0xEF) | 0x20 // 0xEF = 11101111, 0x20 = 00100000
    SetP(cpu, newP)
}

// Move each of the bits in either A or M one place to the right. Bit 7 is filled with the current value of the carry flag whilst the old bit 0 becomes the new carry flag value.
func ROR(cpu *CPU, cart *cartridge.Cartridge, value uint16, op byte) {
    var result byte
    var tmp byte

    switch op {
    case 0x66, 0x6E, 0x76, 0x7E: // Memory Addressing Modes
        result = RM(cpu, cart, value)
        tmp = result & 0x1
        result = (result >> 1) | (FlagC(cpu) << 7)
        WM(cpu, cart, value, result)
    case 0x6A: // Accumulator
        tmp = cpu.A & 0x1
        cpu.A = (cpu.A >> 1) | (FlagC(cpu) << 7)
        result = cpu.A
    default:
        return
    }

    // Set Flags
    SetC(cpu, tmp)
    ZeroFlag(cpu, uint16(result)) // Corrected line
    SetN(cpu, (result>>7)&1)
}

// Move each of the bits in either A or M one place to the left. Bit 0 is filled with the current value of the carry flag whilst the old bit 7 becomes the new carry flag value.
func ROL(cpu *CPU, cart *cartridge.Cartridge, value uint16, op byte) {
    var result byte
    var tmp byte

    switch op {
    case 0x26, 0x2E, 0x36, 0x3E: // Memory Addressing Modes
        result = RM(cpu, cart, value)
        tmp = (result >> 7) & 0x1
        result = (result << 1) | FlagC(cpu)
        WM(cpu, cart, value, result)
    case 0x2A: // Accumulator
        tmp = (cpu.A >> 7) & 0x1
        cpu.A = (cpu.A << 1) | FlagC(cpu)
        result = cpu.A
    default:
        // Handle unknown opcode or raise an error
        return
    }

    // Set Flags
    SetC(cpu, tmp)
    ZeroFlag(cpu, uint16(result)) // Corrected line
    SetN(cpu, (result>>7)&1)
}


// This instruction subtracts the contents of a memory location to the accumulator together with the not of the carry bit. If overflow occurs the carry bit is clear, this enables multiple byte subtraction to be performed.
// Obs: sbc(x) = adc(255-x)
func SBC(cpu *CPU, value uint16) {
    // Cast the value to byte to handle 8-bit operations
    m := byte(value & 0xFF)

    // Retrieve the current state of the carry flag (1 if set, 0 if clear)
    carry := FlagC(cpu)

    // Perform the subtraction: A - M - (1 - C)
    // This is equivalent to A + (~M) + C
    a := uint16(cpu.A)
    tmp := a + uint16(^m) + uint16(carry)

    // Set the Carry flag:
    // In subtraction, the Carry flag is set if no borrow occurred (A >= M + (1 - C))
    if tmp > 0xFF {
        SetC(cpu, 1)
    } else {
        SetC(cpu, 0)
    }

    // Compute the 8-bit result
    result := byte(tmp & 0xFF)

    // Set the Zero flag if the result is zero
    if result == 0 {
        SetZ(cpu, 1)
    } else {
        SetZ(cpu, 0)
    }

    // Set the Negative flag based on the most significant bit of the result
    SetN(cpu, (result>>7)&1)

    // Set the Overflow flag
    // Overflow occurs if the sign of A and M are different and the sign of A and result are different
    if (((cpu.A ^ m) & 0x80) != 0) && (((cpu.A ^ result) & 0x80) != 0) {
        SetV(cpu, 1)
    } else {
        SetV(cpu, 0)
    }

    // Update the accumulator with the result
    cpu.A = result
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
// The RTS instruction is used at the end of a subroutine to return to the calling routine.
// It pulls the program counter (minus one) from the stack and increments it to the return address.
func RTS(cpu *CPU) {
    returnAddress := PopWord(cpu)
    returnAddress = (returnAddress + 1) & 0xFFFF
    cpu.PC = returnAddress
}

// The JSR instruction pushes the address (minus one) of the return point onto the stack 
// and then sets the program counter to the target memory address.
func JSR(cpu *CPU, value uint16) {
    returnAddress := (cpu.PC + 2) & 0xFFFF
    PushWord(cpu, returnAddress)
    cpu.PC = value
}

// The BRK instruction forces the generation of an interrupt request. The program counter and processor status are pushed on the stack then the IRQ interrupt vector at $FFFE/F is loaded into the PC and the break flag in the status set to one.
func BRK(cpu *CPU, cart *cartridge.Cartridge) {
    
    returnAddress := (cpu.PC + 2) & 0xFFFF
    PushWord(cpu, returnAddress)
    statusWithBreak := cpu.P | 0x10
    PushMemory(cpu, statusWithBreak)
    SetI(cpu, 1)
    low := RM(cpu, cart, 0xFFFE)
    high := RM(cpu, cart, 0xFFFF)
    cpu.PC = LE(low, high)
}


// The RTI instruction is used at the end of an interrupt processing routine. It pulls the processor flags from the stack followed by the program counter.
// Pulls an 8-bit value from the stack and into the processor flags.
// Ensures that Bit 5 is always set and Bit 4 (Break) is cleared.
func RTI(cpu *CPU) {
    
    var all byte = PopMemory(cpu)
    // Correctly handle the processor status
    newP := (all & 0xEF) | 0x20
    SetP(cpu, newP)
    cpu.PC = PopWord(cpu)
}
