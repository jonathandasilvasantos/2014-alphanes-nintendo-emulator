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

import (
    "fmt"
    "log"
    "zerojnt/apu"
    "zerojnt/cartridge"
    "zerojnt/debug"
    "zerojnt/ioports"
)

type CPU struct {
    Name string
    
    A byte // Accumulator
    X byte // X Index
    Y byte // Y Index
    P byte // Status
    PC uint16 // Program Count 16bits
    lastPC uint16
    SP byte // Stack Pointer
    CYC uint16
    CYCSpecial uint16 // For cases when we need to add more cycles for an operation
    PageCrossed byte // Only the addressing methods change this property
    Running bool
    Start int
    End int
    SwitchTimes int
    D debug.Debug
    IO ioports.IOPorts
    APU *apu.APU // APU instance
}

func StartCPU() CPU {
    var cpu CPU
    cpu.Name = "Ricoh 2A03"
    
    // Initialize APU
    var err error
    cpu.APU, err = apu.NewAPU()
    if err != nil {
        log.Fatalf("Failed to initialize APU: %v", err)
    }
    
    ResetCPU(&cpu)
    
    cpu.Start = 0
    cpu.End = 0xFFFF    
    cpu.SwitchTimes = -1
    fmt.Println(cpu.Name)
    return cpu
}

func ResetCPU(cpu *CPU) {
    cpu.A = 0
    cpu.X = 0
    cpu.Y = 0
    cpu.P = 0x24 // 00100000 = 32
    cpu.PC = 0xC000
    cpu.SP = 0xFD
    cpu.CYCSpecial = 0
    cpu.Running = true
    cpu.SwitchTimes = -1
}

func SetResetVector(cpu *CPU, cart *cartridge.Cartridge) {
    cpu.PC = LE(RM(cpu, cart, 0xFFFC), RM(cpu, cart, 0xFFFD))
}

func Process(cpu *CPU, cart *cartridge.Cartridge) {
    if cpu.Running {
        emulate(cpu, cart)
        cpu.APU.Clock() // Clock the APU each CPU cycle
    }
}

// WriteMemory handles memory writes including APU registers
func WriteMemory(cpu *CPU, cart *cartridge.Cartridge, addr uint16, value byte) {
    // Check if this is an APU register write
    if addr >= 0x4000 && addr <= 0x4017 {
        cpu.APU.WriteRegister(addr, value)
        return
    }
    
    // Original memory write logic
    WM(cpu, cart, addr, value)
}

// ReadMemory handles memory reads including APU registers
func ReadMemory(cpu *CPU, cart *cartridge.Cartridge, addr uint16) byte {
    // Check if this is an APU register read
    if addr == 0x4015 {
        // TODO: Implement APU status register read
        return 0
    }
    
    // Original memory read logic
    return RM(cpu, cart, addr)
}

func ZeroFlag(cpu *CPU, value uint16) {
    if byte(value) == 0 {
        SetZ(cpu, 1)
    } else {
        SetZ(cpu, 0)
    }
}

func NegativeFlag(cpu *CPU, value uint16) {
    var c byte = byte(value) >> 7
    if c == 1 {
        SetN(cpu, 1)
    } else {
        SetN(cpu, 0)
    }
}

func CarryFlag(cpu *CPU, value uint16) {
    if value > 0xFF || value < 0 {
        SetC(cpu, 1)
    } else {
        SetC(cpu, 0)
    }
}

// Cleanup function to properly shutdown the APU
func Cleanup(cpu *CPU) {
    if cpu.APU != nil {
        cpu.APU.Shutdown()
    }
}