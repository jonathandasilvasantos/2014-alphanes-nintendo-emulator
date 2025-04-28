package cpu

import "zerojnt/cartridge"

// Addressing mode helpers for the 6502 core
// All functions return the effective address to be accessed by the CPU
// For modes that can cross pages (AbsX, AbsY, IndY), the cpu.PageCrossed flag is set
// Zero-page and pointers always wrap around within the 8 least significant bits

// Rel - 8-bit signed relative offset used by branch instructions
// Target = (PC + 2) + offset, where PC points to opcode
func Rel(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	off := int8(RM(cpu, cart, cpu.PC+1)) // -128 to +127
	base := cpu.PC + 2                   // next instruction
	return base + uint16(int16(off))     // add with sign-extend and 16-bit wrap
}

// Imm - immediate mode (returns as uint16 for generic reuse)
func Imm(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	return uint16(RM(cpu, cart, cpu.PC+1))
}

// Abs - 16-bit address embedded in instruction
func Abs(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	lo := RM(cpu, cart, cpu.PC+1)
	hi := RM(cpu, cart, cpu.PC+2)
	return (uint16(hi) << 8) | uint16(lo)
}

// AbsX - absolute indexed by X; marks page-cross if crossing boundary
func AbsX(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	base := Abs(cpu, cart)
	addr := base + uint16(cpu.X)
	cpu.PageCrossed = BoolToByte(H(base) != H(addr))
	return addr
}

// AbsY - absolute indexed by Y
func AbsY(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	base := Abs(cpu, cart)
	addr := base + uint16(cpu.Y)
	cpu.PageCrossed = BoolToByte(H(base) != H(addr))
	return addr
}

// Zp - direct zero-page
func Zp(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	return uint16(RM(cpu, cart, cpu.PC+1))
}

// ZpX - zero-page indexed by X (wrap 0x00-0xFF)
func ZpX(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	return uint16((RM(cpu, cart, cpu.PC+1) + cpu.X) & 0xFF)
}

// ZpY - zero-page indexed by Y
func ZpY(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	return uint16((RM(cpu, cart, cpu.PC+1) + cpu.Y) & 0xFF)
}

// Ind - indirect mode used only by JMP
// Implements the page-wrap "bug": if pointer is at 0xXXFF,
// hi-byte comes from 0xXX00 (not 0x(X+1)00)
func Ind(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	ptr := Abs(cpu, cart)
	lo := RM(cpu, cart, ptr)
	hi := RM(cpu, cart, (ptr&0xFF00)|uint16((ptr+1)&0x00FF))
	return LE(lo, hi)
}

// IndX - (d,X) - first adds X to operand, then reads pointer
func IndX(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	zp := (RM(cpu, cart, cpu.PC+1) + cpu.X) & 0xFF
	lo := RM(cpu, cart, uint16(zp))
	hi := RM(cpu, cart, uint16((zp+1)&0xFF))
	return LE(lo, hi)
}

// IndY - (d),Y - reads pointer, then adds Y; marks page-cross
func IndY(cpu *CPU, cart *cartridge.Cartridge) uint16 {
	zp := RM(cpu, cart, cpu.PC+1)
	lo := RM(cpu, cart, uint16(zp))
	hi := RM(cpu, cart, uint16((zp+1)&0xFF))
	base := LE(lo, hi)
	addr := base + uint16(cpu.Y)
	cpu.PageCrossed = BoolToByte(H(base) != H(addr))
	return addr
}