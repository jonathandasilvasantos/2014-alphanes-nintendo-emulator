/*
Copyright 2014, 2015 Jonathan da Silva SAntos

This file is part of Alphanes.

    Alphanes is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    Alphanes is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with Alphanes. If not, see <http://www.gnu.org/licenses/>.
*/
package cpu

import "zerojnt/cartridge"

// AND (Logical AND)
// A logical AND is performed, bit by bit, on the accumulator contents using the contents of a byte of memory.
func AND(cpu *CPU, value uint16) {
	cpu.A &= byte(value)       // Perform bitwise AND with the value
	ZeroFlag(cpu, uint16(cpu.A)) // Update Zero Flag based on the result
	SetN(cpu, cpu.A>>7)         // Update Negative Flag based on bit 7 of the result
}

// ASL (Arithmetic Shift Left - Accumulator)
// This operation shifts all the bits of the accumulator one bit left.
// Bit 0 is set to 0, and bit 7 is placed in the carry flag.
func ASL_A(cpu *CPU) {
	SetC(cpu, (cpu.A>>7)&1)     // Set Carry Flag to the original bit 7 of A
	cpu.A <<= 1                // Shift A left by 1 bit
	ZeroFlag(cpu, uint16(cpu.A)) // Update Zero Flag based on the result
	SetN(cpu, cpu.A>>7)         // Update Negative Flag based on the new bit 7
}

// ASL (Arithmetic Shift Left - Memory)
// This operation shifts all the bits of the memory contents one bit left.
// Bit 0 is set to 0, and bit 7 is placed in the carry flag.
func ASL_M(cpu *CPU, cart *cartridge.Cartridge, address uint16) {
	value := RM(cpu, cart, address) // Read the value from memory
	SetC(cpu, (value>>7)&1)     // Set Carry Flag to the original bit 7
	value <<= 1                // Shift the value left by 1 bit
	WM(cpu, cart, address, value)   // Write the shifted value back to memory
	ZeroFlag(cpu, uint16(value)) // Update Zero Flag based on the result
	SetN(cpu, value>>7)         // Update Negative Flag based on the new bit 7
}

// ADC (Add with Carry)
// Adds the contents of a memory location to the accumulator together with the carry bit.
func ADC(cpu *CPU, value byte) {
	a := uint16(cpu.A)
	c := uint16(FlagC(cpu))
	sum := a + uint16(value) + c

	// Set Carry if there's an overflow beyond 8 bits
	SetC(cpu, BoolToByte(sum > 255))

	// Set Overflow if the sign of both inputs is different from the sign of the result
	SetV(cpu, BoolToByte(((cpu.A^value)&0x80 == 0) && ((cpu.A^byte(sum))&0x80 != 0)))

	cpu.A = byte(sum)          // Update accumulator with the result
	ZeroFlag(cpu, uint16(cpu.A)) // Update Zero Flag
	SetN(cpu, cpu.A>>7)         // Update Negative Flag
}

// BCC (Branch if Carry Clear)
// If the carry flag is clear, then add the relative displacement to the program counter.
func BCC(cpu *CPU, value uint16) {
	cpu.CYCSpecial = 0
	if FlagC(cpu) == 0 {
		Branch(cpu, value)
	} else {
		cpu.PC += 2
	}
}

// BCS (Branch if Carry Set)
// If the carry flag is set, then add the relative displacement to the program counter.
func BCS(cpu *CPU, value uint16) {
	cpu.CYCSpecial = 0
	if FlagC(cpu) == 1 {
		Branch(cpu, value)
	} else {
		cpu.PC += 2
	}
}

// BEQ (Branch if Equal)
// If the zero flag is set, then add the relative displacement to the program counter.
func BEQ(cpu *CPU, value uint16) {
	cpu.CYCSpecial = 0
	if FlagZ(cpu) == 1 {
		Branch(cpu, value)
	} else {
		cpu.PC += 2
	}
}

// BIT (Bit Test)
// Tests if one or more bits are set in a target memory location.
func BIT(cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	memValue := RM(cpu, cart, value)
	result := cpu.A & memValue
	ZeroFlag(cpu, uint16(result)) // Update Zero Flag based on the AND result
	SetN(cpu, memValue>>7)         // Update Negative Flag based on bit 7 of memory value
	SetV(cpu, (memValue>>6)&1)     // Update Overflow Flag based on bit 6 of memory value
}

// BMI (Branch if Minus)
// If the negative flag is set, then add the relative displacement to the program counter.
func BMI(cpu *CPU, value uint16) {
	cpu.CYCSpecial = 0
	if FlagN(cpu) == 1 {
		Branch(cpu, value)
	} else {
		cpu.PC += 2
	}
}

// BNE (Branch if Not Equal)
// If the zero flag is clear, then add the relative displacement to the program counter.
func BNE(cpu *CPU, value uint16) {
	cpu.CYCSpecial = 0
	if FlagZ(cpu) == 0 {
		Branch(cpu, value)
	} else {
		cpu.PC += 2
	}
}

// BPL (Branch if Positive)
// If the negative flag is clear, then add the relative displacement to the program counter.
func BPL(cpu *CPU, value uint16) {
	cpu.CYCSpecial = 0
	if FlagN(cpu) == 0 {
		Branch(cpu, value)
	} else {
		cpu.PC += 2
	}
}

// BVC (Branch if Overflow Clear)
// If the overflow flag is clear, then add the relative displacement to the program counter.
func BVC(cpu *CPU, value uint16) {
	cpu.CYCSpecial = 0
	if FlagV(cpu) == 0 {
		Branch(cpu, value)
	} else {
		cpu.PC += 2
	}
}

// BVS (Branch if Overflow Set)
// If the overflow flag is set, then add the relative displacement to the program counter.
func BVS(cpu *CPU, value uint16) {
	cpu.CYCSpecial = 0
	if FlagV(cpu) == 1 {
		Branch(cpu, value)
	} else {
		cpu.PC += 2
	}
}

// CMP (Compare)
// Compares the contents of the accumulator with another memory held value.
func CMP(cpu *CPU, value uint16) {
	result := uint16(cpu.A) - value
	SetC(cpu, BoolToByte(cpu.A >= byte(value))) // Set Carry if A >= value
	ZeroFlag(cpu, result)                     // Update Zero Flag
	SetN(cpu, byte(result>>7))                // Update Negative Flag
}

// CLC (Clear Carry Flag)
// Sets the carry flag to zero.
func CLC(cpu *CPU) {
	SetC(cpu, 0)
}

// CLD (Clear Decimal Mode)
// Sets the decimal mode flag to zero.
func CLD(cpu *CPU) {
	SetD(cpu, 0)
}

// CLI (Clear Interrupt Disable)
// Clears the interrupt disable flag, allowing normal interrupt requests to be serviced.
func CLI(cpu *CPU) {
	SetI(cpu, 0)
}

// CLV (Clear Overflow Flag)
// Clears the overflow flag.
func CLV(cpu *CPU) {
	SetV(cpu, 0)
}

// CPX (Compare X Register)
// Compares the contents of the X register with another memory held value.
func CPX(cpu *CPU, value uint16) {
	result := uint16(cpu.X) - value
	SetC(cpu, BoolToByte(cpu.X >= byte(value))) // Set Carry if X >= value
	ZeroFlag(cpu, result)                     // Update Zero Flag
	SetN(cpu, byte(result>>7))                // Update Negative Flag
}

// CPY (Compare Y Register)
// Compares the contents of the Y register with another memory held value.
func CPY(cpu *CPU, value uint16) {
	result := uint16(cpu.Y) - value
	SetC(cpu, BoolToByte(cpu.Y >= byte(value))) // Set Carry if Y >= value
	ZeroFlag(cpu, result)                     // Update Zero Flag
	SetN(cpu, byte(result>>7))                // Update Negative Flag
}

// DEC (Decrement Memory)
// Subtracts one from the value held at a specified memory location.
func DEC(cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	memValue := RM(cpu, cart, value)
	memValue--
	WM(cpu, cart, value, memValue)
	ZeroFlag(cpu, uint16(memValue)) // Update Zero Flag
	SetN(cpu, memValue>>7)         // Update Negative Flag
}

// DEX (Decrement X Register)
// Subtracts one from the X register.
func DEX(cpu *CPU) {
	cpu.X--
	ZeroFlag(cpu, uint16(cpu.X)) // Update Zero Flag
	SetN(cpu, cpu.X>>7)         // Update Negative Flag
}

// DEY (Decrement Y Register)
// Subtracts one from the Y register.
func DEY(cpu *CPU) {
	cpu.Y--
	ZeroFlag(cpu, uint16(cpu.Y)) // Update Zero Flag
	SetN(cpu, cpu.Y>>7)         // Update Negative Flag
}

// EOR (Exclusive OR)
// Performs an exclusive OR operation, bit by bit, on the accumulator contents using the contents of a byte of memory.
func EOR(cpu *CPU, value uint16) {
	cpu.A ^= byte(value)        // Perform bitwise XOR
	ZeroFlag(cpu, uint16(cpu.A)) // Update Zero Flag
	SetN(cpu, cpu.A>>7)         // Update Negative Flag
}

// INC (Increment Memory)
// Adds one to the value held at a specified memory location.
func INC(cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	memValue := RM(cpu, cart, value)
	memValue++
	WM(cpu, cart, value, memValue)
	ZeroFlag(cpu, uint16(memValue)) // Update Zero Flag
	SetN(cpu, memValue>>7)         // Update Negative Flag
}

// INX (Increment X Register)
// Adds one to the X register.
func INX(cpu *CPU) {
	cpu.X++
	ZeroFlag(cpu, uint16(cpu.X)) // Update Zero Flag
	SetN(cpu, cpu.X>>7)         // Update Negative Flag
}

// INY (Increment Y Register)
// Adds one to the Y register.
func INY(cpu *CPU) {
	cpu.Y++
	ZeroFlag(cpu, uint16(cpu.Y)) // Update Zero Flag
	SetN(cpu, cpu.Y>>7)         // Update Negative Flag
}

// JMP (Jump)
// Sets the program counter to the address specified by the operand.
func JMP(cpu *CPU, value uint16) {
	cpu.PC = value
}

// LDA (Load Accumulator)
// Loads a byte of memory into the accumulator.
func LDA(cpu *CPU, value uint16) {
	cpu.A = byte(value)
	ZeroFlag(cpu, uint16(cpu.A)) // Update Zero Flag
	SetN(cpu, cpu.A>>7)         // Update Negative Flag
}

// LDX (Load X Register)
// Loads a byte of memory into the X register.
func LDX(cpu *CPU, value uint16) {
	cpu.X = byte(value)
	ZeroFlag(cpu, uint16(cpu.X)) // Update Zero Flag
	SetN(cpu, cpu.X>>7)         // Update Negative Flag
}

// LDY (Load Y Register)
// Loads a byte of memory into the Y register.
func LDY(cpu *CPU, value uint16) {
	cpu.Y = byte(value)
	ZeroFlag(cpu, uint16(cpu.Y)) // Update Zero Flag
	SetN(cpu, cpu.Y>>7)         // Update Negative Flag
}

// LSR_A (Logical Shift Right - Accumulator)
// Shifts all bits in the accumulator one position to the right.
func LSR_A(cpu *CPU) {
	SetC(cpu, cpu.A&1)          // Set Carry Flag to the original bit 0 of A
	cpu.A >>= 1                // Shift A right by 1 bit
	ZeroFlag(cpu, uint16(cpu.A)) // Update Zero Flag
	SetN(cpu, 0)                // Clear Negative Flag (bit 7 is always 0 after LSR)
}

// LSR_M (Logical Shift Right - Memory)
// Shifts all bits in the memory location one position to the right.
func LSR_M(cpu *CPU, cart *cartridge.Cartridge, address uint16) {
	value := RM(cpu, cart, address) // Read the value from memory
	SetC(cpu, value&1)          // Set Carry Flag to the original bit 0
	value >>= 1                // Shift the value right by 1 bit
	WM(cpu, cart, address, value)   // Write the shifted value back to memory
	ZeroFlag(cpu, uint16(value)) // Update Zero Flag
	SetN(cpu, 0)                // Clear Negative Flag (bit 7 is always 0 after LSR)
}

// NOP (No Operation)
// The NOP instruction causes no changes to the processor other than the normal incrementing of the program counter.
func NOP() {
	// Do nothing
}

// ORA (Logical Inclusive OR)
// Performs an inclusive OR operation, bit by bit, on the accumulator contents using the contents of a byte of memory.
func ORA(cpu *CPU, value uint16) {
	cpu.A |= byte(value)        // Perform bitwise OR
	ZeroFlag(cpu, uint16(cpu.A)) // Update Zero Flag
	SetN(cpu, cpu.A>>7)         // Update Negative Flag
}

// PLA (Pull Accumulator)
// Pulls an 8-bit value from the stack and into the accumulator.
func PLA(cpu *CPU) {
	cpu.A = PopMemory(cpu)
	ZeroFlag(cpu, uint16(cpu.A)) // Update Zero Flag
	SetN(cpu, cpu.A>>7)         // Update Negative Flag
}

// PHA (Push Accumulator)
// Pushes a copy of the accumulator onto the stack.
func PHA(cpu *CPU) {
	PushMemory(cpu, cpu.A)
}

// PHP (Push Processor Status)
// Pushes a copy of the status flags onto the stack.
func PHP(cpu *CPU) {
	// Bit 4 (B flag) and Bit 5 (always 1) are set to 1 when pushing to the stack.
	PushMemory(cpu, SetBit(SetBit(cpu.P, 4, 1), 5, 1))
}

// PLP (Pull Processor Status)
// Pulls an 8-bit value from the stack and into the processor flags.
func PLP(cpu *CPU) {
	status := PopMemory(cpu)
	// Bit 4 (B flag) is cleared, and Bit 5 (always 1) is set when pulling from the stack.
	SetP(cpu, (status&0xEF)|0x20)
}

// ROR (Rotate Right)
// Moves each of the bits in either A or M one place to the right.
func ROR(cpu *CPU, cart *cartridge.Cartridge, value uint16, op byte) {
	var result byte
	var tmp byte

	switch op {
	case 0x66, 0x6E, 0x76, 0x7E: // Memory Addressing Modes
		result = RM(cpu, cart, value)
		tmp = result & 0x1                                 // Store original bit 0
		result = (result >> 1) | (FlagC(cpu) << 7)         // Rotate right, inserting carry into bit 7
		WM(cpu, cart, value, result)                      // Write the result back to memory
	case 0x6A: // Accumulator
		tmp = cpu.A & 0x1                                  // Store original bit 0
		cpu.A = (cpu.A >> 1) | (FlagC(cpu) << 7)           // Rotate right, inserting carry into bit 7
		result = cpu.A
	default:
		return
	}

	SetC(cpu, tmp)              // Set Carry to the original bit 0
	ZeroFlag(cpu, uint16(result)) // Update Zero Flag
	SetN(cpu, result>>7)        // Update Negative Flag
}

// ROL (Rotate Left)
// Moves each of the bits in either A or M one place to the left.
func ROL(cpu *CPU, cart *cartridge.Cartridge, value uint16, op byte) {
	var result byte
	var tmp byte

	switch op {
	case 0x26, 0x2E, 0x36, 0x3E: // Memory Addressing Modes
		result = RM(cpu, cart, value)
		tmp = (result >> 7) & 0x1                         // Store original bit 7
		result = (result << 1) | FlagC(cpu)               // Rotate left, inserting carry into bit 0
		WM(cpu, cart, value, result)                      // Write the result back to memory
	case 0x2A: // Accumulator
		tmp = (cpu.A >> 7) & 0x1                          // Store original bit 7
		cpu.A = (cpu.A << 1) | FlagC(cpu)                 // Rotate left, inserting carry into bit 0
		result = cpu.A
	default:
		return
	}

	SetC(cpu, tmp)              // Set Carry to the original bit 7
	ZeroFlag(cpu, uint16(result)) // Update Zero Flag
	SetN(cpu, result>>7)        // Update Negative Flag
}

// SBC (Subtract with Carry)
// Subtracts the contents of a memory location from the accumulator together with the NOT of the carry bit.
func SBC(cpu *CPU, value uint16) {
	// SBC is equivalent to ADC of the two's complement of the value
	val := byte(value)
	complement := ^val
	ADC(cpu, complement)
}

// SEC (Set Carry Flag)
// Sets the carry flag to one.
func SEC(cpu *CPU) {
	SetC(cpu, 1)
}

// SED (Set Decimal Flag)
// Sets the decimal mode flag to one.
func SED(cpu *CPU) {
	SetD(cpu, 1)
}

// SEI (Set Interrupt Disable)
// Sets the interrupt disable flag to one.
func SEI(cpu *CPU) {
	SetI(cpu, 1)
}

// STA (Store Accumulator)
// Stores the contents of the accumulator into memory.
func STA(cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	WM(cpu, cart, value, cpu.A)
}

// STX (Store X Register)
// Stores the contents of the X register into memory.
func STX(cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	WM(cpu, cart, value, cpu.X)
}

// STY (Store Y Register)
// Stores the contents of the Y register into memory.
func STY(cpu *CPU, cart *cartridge.Cartridge, value uint16) {
	WM(cpu, cart, value, cpu.Y)
}

// TAX (Transfer Accumulator to X)
// Copies the current contents of the accumulator into the X register.
func TAX(cpu *CPU) {
	cpu.X = cpu.A
	ZeroFlag(cpu, uint16(cpu.X)) // Update Zero Flag
	SetN(cpu, cpu.X>>7)         // Update Negative Flag
}

// TAY (Transfer Accumulator to Y)
// Copies the current contents of the accumulator into the Y register.
func TAY(cpu *CPU) {
	cpu.Y = cpu.A
	ZeroFlag(cpu, uint16(cpu.Y)) // Update Zero Flag
	SetN(cpu, cpu.Y>>7)         // Update Negative Flag
}

// TSX (Transfer Stack Pointer to X)
// Copies the current contents of the stack pointer into the X register.
func TSX(cpu *CPU) {
	cpu.X = cpu.SP
	ZeroFlag(cpu, uint16(cpu.X)) // Update Zero Flag
	SetN(cpu, cpu.X>>7)         // Update Negative Flag
}

// TXA (Transfer X to Accumulator)
// Copies the current contents of the X register into the accumulator.
func TXA(cpu *CPU) {
	cpu.A = cpu.X
	ZeroFlag(cpu, uint16(cpu.A)) // Update Zero Flag
	SetN(cpu, cpu.A>>7)         // Update Negative Flag
}

// TXS (Transfer X to Stack Pointer)
// Copies the current contents of the X register into the stack pointer.
func TXS(cpu *CPU) {
	cpu.SP = cpu.X
}

// TYA (Transfer Y to Accumulator)
// Copies the current contents of the Y register into the accumulator.
func TYA(cpu *CPU) {
	cpu.A = cpu.Y
	ZeroFlag(cpu, uint16(cpu.A)) // Update Zero Flag
	SetN(cpu, cpu.A>>7)         // Update Negative Flag
}

// RTS (Return from Subroutine)
// Returns from a subroutine by pulling the program counter from the stack.
func RTS(cpu *CPU) {
	cpu.PC = PopWord(cpu) + 1 // Pull PC from stack and increment by 1
}

// JSR (Jump to Subroutine)
func JSR(cpu *CPU, value uint16) {
    PushWord(cpu, cpu.PC + 2) // Push the address of the next instruction (which is current PC + 2 since we are at the last byte of the JSR instruction)
    cpu.PC = value           // Set PC to the subroutine address
}


// BRK (Force Interrupt)
// Forces the generation of an interrupt request.
func BRK(cpu *CPU, cart *cartridge.Cartridge) {
	PushWord(cpu, cpu.PC+1)        // Push PC + 1 (return address after BRK) onto the stack
	PHP(cpu)                       // Push processor status with B flag set to 1
	SEI(cpu)                       // Set Interrupt Disable to prevent further interrupts
	cpu.PC = LE(RM(cpu, cart, 0xFFFE), RM(cpu, cart, 0xFFFF)) // Load PC from interrupt vector
}

// RTI (Return from Interrupt)
// Returns from an interrupt by pulling the processor status and program counter from the stack.
func RTI(cpu *CPU) {
	PLP(cpu)           // Pull processor status from the stack
	cpu.PC = PopWord(cpu) // Pull PC from the stack
}


