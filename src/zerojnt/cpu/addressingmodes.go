/*
Copyright 2014‑2025 Jonathan da Silva Santos

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

// -----------------------------------------
// Addressing‑mode helpers for the 6502 core
// -----------------------------------------
// Todas retornam o endereço *efetivo* que a UCP acessará.  Para modos que
// podem cruzar páginas (AbsX/Y, IndY) a flag cpu.PageCrossed é ajustada –
// isto deve ser verificado pelo chamador para cobrar o ciclo extra.
// -----------------------------------------

// Rel — deslocamento relativo signed (+127/‑128) usado pelas instruções de branch.
func Rel(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    off := int8(RM(cpu, cart, cpu.PC+1))                  // deslocamento assinado
    return uint16(int32(cpu.PC) + 2 + int32(off))         // PC já aponta para o opcode
}

// Imm — operando imediato (retorna como uint16 para reutilização).
func Imm(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    return uint16(RM(cpu, cart, cpu.PC+1))
}

// Abs — endereço de 16 bits na própria instrução.
func Abs(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    lo := RM(cpu, cart, cpu.PC+1)
    hi := RM(cpu, cart, cpu.PC+2)
    return (uint16(hi) << 8) | uint16(lo)
}

// AbsX — endereço absoluto indexado por X, com detecção de page‑cross.
func AbsX(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    base := Abs(cpu, cart)
    addr := base + uint16(cpu.X)
    cpu.PageCrossed = BoolToByte(H(base) != H(addr))
    return addr
}

// AbsY — endereço absoluto indexado por Y.
func AbsY(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    base := Abs(cpu, cart)
    addr := base + uint16(cpu.Y)
    cpu.PageCrossed = BoolToByte(H(base) != H(addr))
    return addr
}

// Zp — zero‑page direto (cara a cara com o operando).
func Zp(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    return uint16(RM(cpu, cart, cpu.PC+1))
}

// ZpX — zero‑page indexado por X (wrap 0x00‑0xFF).
func ZpX(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    return uint16((RM(cpu, cart, cpu.PC+1) + cpu.X) & 0xFF)
}

// ZpY — zero‑page indexado por Y.
func ZpY(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    return uint16((RM(cpu, cart, cpu.PC+1) + cpu.Y) & 0xFF)
}

// Ind — modo indireto (somente em JMP).  Implementa o bug de page‑wrap do 6502.
func Ind(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    ptr := Abs(cpu, cart)                                 // endereço do ponteiro HHLL
    lo := RM(cpu, cart, ptr)
    hi := RM(cpu, cart, (ptr&0xFF00)|uint16((ptr+1)&0x00FF)) // wrap em 0xXXFF→0xXX00
    return LE(lo, hi)
}

// IndX — (d,X)   pós‑indexado pelo registrador X, *antes* de formar o par de bytes.
func IndX(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    zp := (RM(cpu, cart, cpu.PC+1) + cpu.X) & 0xFF         // wrap na zero‑page
    lo := RM(cpu, cart, uint16(zp))
    hi := RM(cpu, cart, uint16((zp+1)&0xFF))
    return LE(lo, hi)
}

// IndY — (d),Y  pré‑indexado: ponteiro lido, soma Y, marca page‑cross.
func IndY(cpu *CPU, cart *cartridge.Cartridge) uint16 {
    zp := RM(cpu, cart, cpu.PC+1)
    lo := RM(cpu, cart, uint16(zp))
    hi := RM(cpu, cart, uint16((zp+1)&0xFF))
    base := LE(lo, hi)
    addr := base + uint16(cpu.Y)
    cpu.PageCrossed = BoolToByte(H(base) != H(addr))
    return addr
}
