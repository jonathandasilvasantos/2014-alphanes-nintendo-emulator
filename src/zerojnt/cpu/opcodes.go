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
import "fmt"

func nmi(cpu *CPU, cart *cartridge.Cartridge) {
    // 1. Push the current Program Counter (PC) onto the stack.
    //    The PC should point to the next instruction to execute after returning from the NMI.
    PushWord(cpu, cpu.PC)

    // 2. Push the Processor Status Register (P) onto the stack.
    //    This saves the current state of the processor flags.
    PushMemory(cpu, cpu.P)

    // 3. Set the Program Counter (PC) to the NMI vector located at $FFFA/B.
    //    This tells the CPU where to jump to handle the NMI.
    nmiVectorLow := RM(cpu, cart, 0xFFFA)
    nmiVectorHigh := RM(cpu, cart, 0xFFFB)
    cpu.PC = LE(nmiVectorLow, nmiVectorHigh)

    // 4. Set the Interrupt Disable Flag (I) to prevent further IRQs.
    //    NMIs are non-maskable and will still occur regardless of this flag.
    SetI(cpu, 1)

    // 5. Reset specific PPU status flags and variables as needed.
    //    Ensure these resets align with NES hardware behavior.
    cpu.IO.PPUSTATUS.WRITTEN = 0
    cpu.IO.PPU_MEMORY_STEP = 0
    cpu.IO.VRAM_ADDRESS = 0

    // 6. Set the CPU cycle count to reflect the time taken to handle the NMI.
    //    The NES CPU takes 7 cycles to process an NMI.
    cpu.CYC = 7

    // 7. (Optional) Log or debug information for verification.
    //    Uncomment the following line if you want to see when an NMI is triggered.
    // fmt.Println("NMI triggered and handled.")
}


func emulate (cpu *CPU, cart *cartridge.Cartridge) {

        // Handle IO operations that takes CPU cycles
        cpu.CYC = cpu.CYC + cpu.IO.CPU_CYC_INCREASE
        cpu.IO.CPU_CYC_INCREASE = 0

	
	if cpu.CYC != 0 {
		cpu.CYC--
		return
	}
	


		

        op := RM(cpu, cart, cpu.PC)



        if (cpu.D.Enable) && (cpu.SwitchTimes > 8000) {
            if op != DebugOp(cpu, cart) {
                if DebugOp(cpu, cart) == 0x48 {
                        fmt.Printf("%x %x \n", op, DebugOp(cpu, cart))
		        nmi(cpu, cart)
                        //return
                }
            }
        }




	if cpu.D.Verbose && cpu.D.Enable { 
		Verbose(cpu, cart)
	}
	
	cpu.SwitchTimes++
	
	if cpu.D.Enable {
		DebugCompare(cpu, cart)
	}
	
	
	// Check the limit of opcodes (Debug function)
	cpu.Start = int(cpu.PC)
	if cpu.Start >= cpu.End {
		cpu.Running = false
		return
	}
	
	// Handle NMI Interruption
	if cpu.IO.NMI && (cpu.D.Enable == false){
		nmi(cpu, cart)
		cpu.IO.NMI = false
		return	
	}

        op = RM(cpu, cart, cpu.PC)
        cpu.lastPC = cpu.PC

	
	switch(RM(cpu, cart, cpu.PC)) {
		
	case 0x00: // BRK Imp
		BRK(cpu, cart)
		cpu.CYC = 7
		break
		
	case 0x01: // ORA IndX
		ORA(cpu, uint16(RM(cpu, cart, IndX(cpu, cart))))
		cpu.CYC = 6
		cpu.PC = cpu.PC + 2
		break
		
		case 0x4: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+2
			cpu.CYC = 2
			break

		


		
	case 0x08: // PHP Imp
		PHP(cpu)
		cpu.CYC = 3
		cpu.PC = cpu.PC + 1
		break
		
	case 0x05: // ORA Zp
		ORA(cpu, uint16(RM(cpu, cart,Zp(cpu, cart))))
		cpu.CYC = 3
		cpu.PC = cpu.PC + 2
		break
		
	case 0x09: // ORA Imm
		ORA(cpu, Imm(cpu, cart))
		cpu.CYC = 2
		cpu.PC = cpu.PC + 2
		break
		

		
		case 0x0C: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+3
			cpu.CYC = 2
			break
		
	case 0x0D: // Bit Abs
		ORA(cpu, uint16(RM(cpu, cart, Abs(cpu, cart))))
		cpu.PC = cpu.PC + 3
		cpu.CYC = 4
		break
		
		



		
	case 0x10: // BPL Relative
		BPL(cpu, Rel(cpu, cart))
		cpu.CYC = 2 + cpu.CYCSpecial
		break

	case 0x11: // ORA IndY
		ORA(cpu, uint16(RM(cpu, cart, IndY(cpu, cart))))
		cpu.CYC = 5
		cpu.PC = cpu.PC + 2
		if cpu.PageCrossed == 1 {
			cpu.CYC ++
		}
		break
		
		case 0x14: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+2
			cpu.CYC = 2
			break
		
		
	case 0x15: // ORA ZpX
		ORA(cpu, uint16(RM(cpu, cart,ZpX(cpu, cart))))
		cpu.CYC = 4
		cpu.PC = cpu.PC + 2
		break
		



		
	case 0x18: // CLC
		CLC(cpu)
		cpu.PC++
		cpu.CYC = 2
		break
		
	case 0x19: // ORA AbsY
		ORA(cpu, uint16(RM(cpu, cart, AbsY(cpu, cart))) )
		cpu.CYC = 4
		cpu.PC = cpu.PC + 3
		if cpu.PageCrossed == 1 {
			cpu.CYC++
		}
		break
		
		case 0x1A: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+1
			cpu.CYC = 2
			break
			
		case 0x1C: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+3
			cpu.CYC = 2
			break
		
		
	case 0x1D: // ORA AbsX
		ORA(cpu, uint16(RM(cpu, cart,AbsX(cpu, cart))))
		cpu.CYC = 4
		if cpu.PageCrossed == 1 {
			cpu.CYC++
		}
		cpu.PC = cpu.PC + 3
		break



		
	case 0x24: // Bit Zp
		BIT(cpu, cart, Zp(cpu, cart))
		cpu.PC = cpu.PC + 2
		cpu.CYC = 3
		break
		
	case 0x21: // AND IndX
		AND(cpu, uint16(RM(cpu, cart, IndX(cpu, cart))))
		cpu.CYC = 6
		cpu.PC = cpu.PC + 2
		break
		
	case 0x25: // AND Zp
		AND(cpu, uint16(RM(cpu, cart, Zp(cpu, cart))))
		cpu.CYC = 3
		cpu.PC = cpu.PC + 2
		break
		


		
		
	case 0x20: // JSR
		JSR(cpu, Abs(cpu, cart))
		cpu.CYC = 6
		break
		
	case 0x28: // PLP Imp
		PLP(cpu)
		cpu.CYC = 4
		cpu.PC = cpu.PC + 1
		break
		
	case 0x29: // AND Imm
		AND(cpu, Imm(cpu, cart))
		cpu.CYC = 2
		cpu.PC = cpu.PC + 2
		break
		
		
	case 0x2C: // Bit Abs
		BIT(cpu, cart, Abs(cpu, cart))
                        if cpu.D.Enable {
                            if ((Abs(cpu, cart) >= 0x2000) && (Abs(cpu, cart) <= 0x2007)) || (Abs(cpu, cart) == 0x4016) {
                                SetP(cpu, DebugP(cpu, cart))
                            }
                        }
		cpu.PC = cpu.PC + 3
		cpu.CYC = 4
		break
		
	case 0x2D: // AND Abs
		AND(cpu, uint16(RM(cpu, cart, Abs(cpu, cart))))
		cpu.CYC = 4
		cpu.PC = cpu.PC + 3
		break

	

		
	case 0x30: // BMI Relative
		BMI(cpu, Rel(cpu, cart))
		cpu.CYC = 2 + cpu.CYCSpecial
		break
		
	case 0x31: // AND IndY
		AND(cpu, uint16(RM(cpu, cart, IndY(cpu, cart))))
		cpu.CYC = 5
		cpu.PC = cpu.PC + 2
		if cpu.PageCrossed == 1 {
			cpu.CYC ++
		}
		break
		
		case 0x34: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+2
			cpu.CYC = 2
			break

		
	case 0x35: // AND ZpX
		AND(cpu, uint16(RM(cpu, cart, ZpX(cpu, cart))))
		cpu.CYC = 4
		cpu.PC = cpu.PC + 2
		break
		

		
		case 0x3A: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+1
			cpu.CYC = 2
			break


		
	case 0x38: // SEC
		SEC(cpu)
		cpu.CYC = 2
		cpu.PC = cpu.PC + 1
		break

	case 0x39: // AND AbsY
		AND(cpu, uint16(RM(cpu, cart, AbsY(cpu, cart))))
		cpu.CYC = 4
		cpu.PC = cpu.PC + 3
		if cpu.PageCrossed == 1 {
			cpu.CYC++
		}
		break
		
		case 0x3C: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+3
			cpu.CYC = 2
			break

		
	case 0x3D: // AND AbsX
		AND(cpu, uint16(RM(cpu, cart, AbsX(cpu, cart))) )
		cpu.CYC = 4
		cpu.PC = cpu.PC + 3
		if cpu.PageCrossed == 1 {
			cpu.CYC++
		}
		break
		
		
	case 0x41: // EOR IndX
		EOR(cpu, uint16(RM(cpu, cart, IndX(cpu, cart))))
		cpu.CYC = 6
		cpu.PC = cpu.PC + 2
		break
		
		case 0x44: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+2
			cpu.CYC = 2
			break
		
		
	case 0x45: // EOR Zp
		EOR(cpu, uint16(RM(cpu, cart, Zp(cpu, cart))))
		cpu.CYC = 3
		cpu.PC = cpu.PC + 2
		break



		case 0x48: // PHA Imp
			PHA(cpu)
			cpu.CYC = 3
			cpu.PC = cpu.PC + 1
			break
			
		case 0x49: // EOR Imm
			EOR (cpu, Imm(cpu, cart))
			cpu.PC = cpu.PC + 2
			cpu.CYC = 2
			break
			
		case 0x40: // RTI Imp
			RTI(cpu)
			cpu.CYC = 6
			break
			

		
		case 0x4C: // JMP Abs
			JMP(cpu, Abs(cpu, cart))
			cpu.CYC = 3
			break
			
		case 0x4D: // EOR Abs
			EOR(cpu, uint16(RM(cpu, cart, Abs(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			break



			
		case 0x50: // BVC Relative
			BVC(cpu, Rel(cpu, cart))
			cpu.CYC = 2 + cpu.CYCSpecial
			break
			
		case 0x51: // EOR IndY
			EOR(cpu, uint16(RM(cpu, cart, IndY(cpu, cart))))
			cpu.CYC = 5
			cpu.PC = cpu.PC + 2
			if cpu.PageCrossed == 1 {
				cpu.CYC ++
			}
			break
			
		case 0x54: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+2
			cpu.CYC = 2
			break
			

		case 0x55: // EOR ZpX
			EOR(cpu, uint16(RM(cpu, cart,ZpX(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 2
			break
			
			
		case 0x58: // CLI Imp
			CLI(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break
			
		case 0x59: // EOR AbsY
			EOR(cpu, uint16(RM(cpu, cart, AbsY(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			if cpu.PageCrossed == 1 {
				cpu.CYC++
			}
			break
			
		case 0x5A: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+1
			cpu.CYC = 2
			break
			
		case 0x5C: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+3
			cpu.CYC = 2
			break
			

		case 0x5D: // EOR AbX
			EOR(cpu, uint16(RM(cpu, cart,AbsX(cpu, cart))))
			cpu.CYC = 4
			if cpu.PageCrossed == 1 {
				cpu.CYC++
			}
			cpu.PC = cpu.PC + 3
			break

			
		case 0x60: // RTS Imp
			RTS(cpu)
			cpu.CYC = 6
			break
			
		
			
		case 0x64: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+2
			cpu.CYC = 2
			break
			
			


			
		case 0x68: // PLA Imp
			PLA(cpu)
			cpu.CYC = 4
			cpu.PC = cpu.PC + 1
			break
			

			

			
		case 0x6C: // JMP Ind
			JMP(cpu, Ind(cpu, cart))
			cpu.CYC = 3
			break

			
			
		case 0x70: // BVS Relative
			BVS(cpu, Rel(cpu, cart))
			cpu.CYC = 2 + cpu.CYCSpecial
			break
			
		
			
		case 0x74: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+2
			cpu.CYC = 2
			break


			
		

			
		case 0x78: // SEI Imp
			SEI(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break

			
		case 0x7A: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+1
			cpu.CYC = 2
			break
			
		case 0x7C: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+3
			cpu.CYC = 2
			break
			
			
		case 0x80: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+2
			cpu.CYC = 2
			break


			
		case 0x81: // STA IndX
			STA(cpu, cart, IndX(cpu, cart))
			cpu.CYC = 6
			cpu.PC = cpu.PC + 2
			break
			
		case 0x84: // STY Zp
			STY(cpu, cart, Zp(cpu, cart))
			cpu.CYC = 3
			cpu.PC = cpu.PC + 2
			break
			
		case 0x85: // STA Zp
			STA(cpu, cart, Zp(cpu, cart))
			cpu.CYC = 3
			cpu.PC = cpu.PC + 2
			break
			
		case 0x86: // STX Zp
			STX(cpu, cart, Zp(cpu, cart))
			cpu.CYC = 3
			cpu.PC = cpu.PC + 2
			break
			
		case 0x88: // DEY Imp
			DEY(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break
			
		case 0x8A: // TXA Imp
			TXA(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC+1
			break
			
		case 0x8C: // STY Abs
			STY(cpu, cart, Abs(cpu, cart))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			break

			
		case 0x8D: // STA Abs
			STA(cpu, cart, Abs(cpu, cart))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			break
			
		case 0x8E: // STX Abs
			STX(cpu, cart, Abs(cpu, cart))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			break
			
		case 0x90: // BCC Relative
			BCC(cpu, Rel(cpu, cart))
			cpu.CYC = 2 + cpu.CYCSpecial
			break
			
		case 0x91: // STA IndY
			STA(cpu, cart, IndY(cpu, cart))
			cpu.CYC = 5
			cpu.PC = cpu.PC + 2
			if cpu.PageCrossed == 1 {
				cpu.CYC ++
			}
			break
			
		case 0x94: // STY ZpX
			STY(cpu, cart, ZpX(cpu, cart))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 2
			break

		case 0x95: // STA ZpX
			STA(cpu, cart, ZpX(cpu, cart))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 2
			break
			
		case 0x96: // STX ZpY
			STX(cpu, cart, ZpY(cpu, cart))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 2
			break



			
		case 0x98: // TYA Imp
			TYA(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break
			
		case 0x99: // STA AbsY
			STA(cpu, cart, AbsY(cpu, cart))
			cpu.CYC = 5
			cpu.PC = cpu.PC + 3
			break

			
		case 0x9A: // TXS Imp
			TXS(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break
			
		case 0x9D: // STA AbX
			STA(cpu, cart, AbsX(cpu, cart))
			cpu.CYC = 5
			cpu.PC = cpu.PC + 3
			break
			
		case 0xA0: // LDY Imm
			LDY(cpu, Imm(cpu, cart))
			cpu.CYC = 2
			cpu.PC = cpu.PC + 2
			break
			
		case 0xA1: // LDA IndX
			LDA(cpu, uint16( RM(cpu, cart, IndX(cpu, cart))))
			cpu.CYC = 6
			cpu.PC = cpu.PC + 2
			break
			
		case 0xA2: // LDX Imm
			LDX(cpu, Imm(cpu, cart))
			cpu.PC = cpu.PC + 2
			cpu.CYC = 2
			break
			
		case 0xA4: // LDY Zp
			LDY(cpu, uint16(RM(cpu, cart, Zp(cpu, cart))) )
			cpu.CYC = 3
			cpu.PC = cpu.PC + 2
			break
			
		case 0xA5: // LDA Zp
			LDA(cpu, uint16(RM(cpu, cart, Zp(cpu, cart))) )
			cpu.CYC = 3
			cpu.PC = cpu.PC + 2
			break
			
		case 0xA6: // LDX Zp
			LDX(cpu, uint16(RM(cpu, cart, Zp(cpu, cart))) )
			cpu.CYC = 3
			cpu.PC = cpu.PC + 2
			break

			
		case 0xA8: // TAY Imp
			TAY(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break
			
		case 0xA9: // LDA Imm
			LDA(cpu, Imm(cpu, cart))
			cpu.PC = cpu.PC + 2
			cpu.CYC = 2
			break
			
		case 0xAA: // TAX Imp
			TAX(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break
			
		case 0xAC: // LDY Abs
			LDY(cpu, uint16(RM(cpu, cart, Abs(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			break
			
		case 0xAD: // LDA Abs
			LDA(cpu, uint16(RM(cpu, cart, Abs(cpu, cart))))
                        if cpu.D.Enable {
                            if ((Abs(cpu, cart) >= 0x2000) && (Abs(cpu, cart) <= 0x2007)) ||
                            (Abs(cpu, cart) == 0x4016) || 
                            (Abs(cpu, cart) == 0x4015) || 
                            (Abs(cpu, cart) == 0x4017)  {
                                cpu.A = DebugA(cpu, cart)
                                SetP(cpu, DebugP(cpu, cart))
                            }
                        }
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			break
			
		case 0xAE: // LDX Abs
			LDX(cpu, uint16(RM(cpu, cart, Abs(cpu, cart))))
                        if cpu.D.Enable {
                            if (Abs(cpu, cart) >= 0x2000) && (Abs(cpu, cart) <= 0x2007) {
                                cpu.X = DebugX(cpu, cart)
                                SetP(cpu, DebugP(cpu, cart))
                            }
                        }
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			break
			
		case 0xB0: // BCS Relative
			BCS(cpu, Rel(cpu, cart))
			cpu.CYC = 2 + cpu.CYCSpecial
			break
			
		case 0xB1: // LDA IndY
		LDA(cpu, uint16(RM(cpu, cart, IndY(cpu, cart))))
		cpu.CYC = 5
		if cpu.PageCrossed == 1 {
			cpu.CYC++
		}
		cpu.PC = cpu.PC + 2
		break
		
	case 0xB4: // LDY ZpX
		LDY(cpu, uint16(RM(cpu, cart, ZpX(cpu, cart))) )
		cpu.CYC = 4
		cpu.PC = cpu.PC + 2
		break

	case 0xB5: // LDA ZpX
		LDA(cpu, uint16(RM(cpu, cart, ZpX(cpu, cart))) )
		cpu.CYC = 4
		cpu.PC = cpu.PC + 2
		break
		
	case 0xB6: // LDX ZpY
		LDX(cpu, uint16(RM(cpu, cart, ZpY(cpu, cart))) )
		cpu.CYC = 4
		cpu.PC = cpu.PC + 2
		break			
			
		case 0xBA: // TSX Imp
			TSX(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break
			
		case 0xB8: // CLV Imp
			CLV(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break
			
		case 0xB9: // LDA AbsY
			LDA(cpu, uint16(RM(cpu, cart, AbsY(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			if cpu.PageCrossed == 1 {
				cpu.CYC ++
			}
			break
			
		case 0xBC: // LDY AbsX
			LDY(cpu, uint16(RM(cpu, cart, AbsX(cpu, cart))) )
			cpu.CYC = 3
			if cpu.PageCrossed == 1 {
				cpu.CYC++
			}
			cpu.PC = cpu.PC + 3
			break
			
		case 0xBD: // LDA AbX
			LDA(cpu, uint16(RM(cpu, cart,AbsX(cpu, cart))))
                        if cpu.D.Enable {
                            if ((Abs(cpu, cart) >= 0x2000) && (Abs(cpu, cart) <= 0x2007)) || (Abs(cpu, cart) == 0x4016) {
                                cpu.A = DebugA(cpu, cart)
                                SetP(cpu, DebugP(cpu, cart))
                            }
                        }
			cpu.CYC = 4
			if cpu.PageCrossed == 1 {
				cpu.CYC++
			}
			cpu.PC = cpu.PC + 3
			break
			
		case 0xBE: // LDX AbsY
			LDX(cpu, uint16(RM(cpu, cart, AbsY(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			if cpu.PageCrossed == 1 {
				cpu.CYC ++
			}
			break
			
		case 0xC0: // CPY Imm
			CPY(cpu, Imm(cpu, cart))
			cpu.CYC = 2
			cpu.PC = cpu.PC + 2
			break
			
		case 0xC1: // EOR IndX
			CMP(cpu, uint16(RM(cpu, cart, IndX(cpu, cart))))
			cpu.CYC = 6
			cpu.PC = cpu.PC + 2
			break
			
		case 0xC4: // CPY Zp
			CPY(cpu, uint16(RM(cpu, cart, Zp(cpu, cart))))
			cpu.CYC = 3
			cpu.PC = cpu.PC + 2
			break


		case 0xC5: // CMP Zp
			CMP	(cpu, uint16(RM(cpu, cart, Zp(cpu, cart))))
			cpu.CYC = 3
			cpu.PC = cpu.PC + 2
			break
			
		case 0xC6: // DEC Zp
			DEC(cpu, cart, Zp(cpu, cart))
			cpu.CYC = 5
			cpu.PC = cpu.PC + 2
			break


			
		case 0xC8: // INY Imp
			INY(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break
			
		case 0xC9: // CMP Imm
			CMP(cpu, Imm(cpu, cart))
			cpu.CYC = 2
			cpu.PC = cpu.PC + 2
			break
			
		case 0xCA: // DEX Imp
			DEX(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break
			
		case 0xCC: // CPY Abs
			CPY(cpu, uint16(RM(cpu, cart, Abs(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			break

			
		case 0xCD: // CMP Abs
			CMP(cpu, uint16(RM(cpu, cart, Abs(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			break

			
		case 0xCE: // DEC Abs
			DEC(cpu, cart, Abs(cpu, cart))
			cpu.CYC = 6
			cpu.PC = cpu.PC + 3
			break

			
		case 0xD0: // BNE Relative
			BNE(cpu, Rel(cpu, cart))
			cpu.CYC = 2 + cpu.CYCSpecial
			break
			
		case 0xD1: // CMP IndY
			CMP(cpu, uint16(RM(cpu, cart, IndY(cpu, cart))))
			cpu.CYC = 5
			cpu.PC = cpu.PC + 2
			if cpu.PageCrossed == 1 {
				cpu.CYC ++
			}
			break
			
		case 0xD4: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+2
			cpu.CYC = 2
			break
			
			
		case 0xD5: // CMP ZpX
			CMP	(cpu, uint16(RM(cpu, cart, ZpX(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 2
			break
	

		case 0xD6: // DEC ZpX
			DEC(cpu, cart, ZpX(cpu, cart))
			cpu.CYC = 6
			cpu.PC = cpu.PC + 2
			break

			
		case 0xD8: // CLD Imp
			CLD(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break
			
		case 0xD9: // CMP AbsY
			CMP(cpu, uint16(RM(cpu, cart, AbsY(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			if cpu.PageCrossed == 1 {
				cpu.CYC++
			}
			break
			
		case 0xDA: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+1
			cpu.CYC = 2
			break
			
		case 0xDC: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+3
			cpu.CYC = 2
			break
			
		case 0xDD: // CMP AbX
			CMP(cpu, uint16(RM(cpu, cart,AbsX(cpu, cart))))
			cpu.CYC = 4
			if cpu.PageCrossed == 1 {
				cpu.CYC++
			}
			cpu.PC = cpu.PC + 3
			break

		case 0xDE: // DEC AbX
			DEC(cpu, cart, AbsX(cpu, cart))
			cpu.CYC = 7
			cpu.PC = cpu.PC + 3
			break

						
		case 0xEA: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+1
			cpu.CYC = 2
			break
			
		case 0xE0: // CPX Imm
			CPX(cpu, Imm(cpu, cart))
			cpu.CYC = 2
			cpu.PC = cpu.PC + 2
			break
			
		case 0xE1: // SBC IndX
			SBC(cpu, uint16(RM(cpu, cart, IndX(cpu, cart))))
			cpu.CYC = 6
			cpu.PC = cpu.PC + 2
			break
			
		case 0xE4: // CPX Zp
			CPX(cpu, uint16(RM(cpu, cart, Zp(cpu, cart))))
			cpu.CYC = 3
			cpu.PC = cpu.PC + 2
			break

			
		case 0xE5: // SBC Zp
			SBC(cpu, uint16(RM(cpu, cart, Zp(cpu, cart))))
			cpu.CYC = 3
			cpu.PC = cpu.PC + 2
			break

		case 0xE6: // INC Zp
			INC(cpu, cart, Zp(cpu, cart))
			cpu.CYC = 5
			cpu.PC = cpu.PC + 2
			break

			
		case 0xE8: // INX Imp
			INX(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC + 1
			break

			
		case 0xE9: // SBC Imm
			SBC(cpu, Imm(cpu, cart))
			cpu.CYC = 2
			cpu.PC = cpu.PC + 2
			break

		case 0xEC: // CPX Abs
			CPX(cpu, uint16(RM(cpu, cart, Abs(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			break

			
		case 0xED: // SBC Abs
			SBC(cpu, uint16(RM(cpu, cart, Abs(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			break
			
		case 0xEE: // INC Abs
			INC(cpu, cart, Abs(cpu, cart))
			cpu.CYC = 6
			cpu.PC = cpu.PC + 3
			break
						
		case 0xF0: // BEQ Relative
			BEQ(cpu, Rel(cpu, cart))
			cpu.CYC = 2 + cpu.CYCSpecial
			break
			
		case 0xF1: // SBC IndY
			SBC(cpu, uint16(RM(cpu, cart, IndY(cpu, cart))))
			cpu.CYC = 5
			cpu.PC = cpu.PC + 2
			if cpu.PageCrossed == 1 {
				cpu.CYC ++
			}
			break
			
		case 0xF4: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+2
			cpu.CYC = 2
			break
				

		case 0xF5: // SBC ZpX
			SBC(cpu, uint16(RM(cpu, cart, ZpX(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 2
			break
			
		case 0xF6: // INC ZpX
			INC(cpu, cart, ZpX(cpu, cart))
			cpu.CYC = 6
			cpu.PC = cpu.PC + 2
			break


			
		case 0xF8: // SED Imp
			SED(cpu)
			cpu.CYC = 2
			cpu.PC = cpu.PC+1
			break
			
		case 0xFC: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+3
			cpu.CYC = 2
			break
			
		case 0xF9: // SBC AbsY
			SBC(cpu, uint16(RM(cpu, cart, AbsY(cpu, cart))))
			cpu.CYC = 4
			cpu.PC = cpu.PC + 3
			if cpu.PageCrossed == 1 {
				cpu.CYC++
			}
			break
			
		case 0xFA: // Nop - No Operation
			NOP()
			cpu.PC = cpu.PC+1
			cpu.CYC = 2
			break
			
		case 0xFD: // SBC AbX
			SBC(cpu, uint16(RM(cpu, cart,AbsX(cpu, cart))))
			cpu.CYC = 4
			if cpu.PageCrossed == 1 {
				cpu.CYC++
			}
			cpu.PC = cpu.PC + 3
			break
			
		case 0xFE: // INC AbX
			INC(cpu, cart, AbsX(cpu, cart))
			cpu.CYC = 7
			cpu.PC = cpu.PC + 3
			break


    // ASL Zero Page
    case 0x06: // ASL Zp
        address := Zp(cpu, cart)          // Calculate Zero Page address
        ASL_M(cpu, cart, address)        // Perform ASL on memory
        cpu.CYC = 5                       // Set cycle count for ASL Zp
        cpu.PC += 2                        // Increment Program Counter by 2 bytes (opcode + operand)
        break

    // ASL Accumulator
    case 0x0A: // ASL Acc
        ASL_A(cpu)                        // Perform ASL on Accumulator
        cpu.CYC = 2                       // Set cycle count for ASL Acc
        cpu.PC += 1                        // Increment Program Counter by 1 byte (opcode only)
        break

    // ASL Absolute
    case 0x0E: // ASL Abs
        address := Abs(cpu, cart)         // Calculate Absolute address
        ASL_M(cpu, cart, address)        // Perform ASL on memory
        cpu.CYC = 6                       // Set cycle count for ASL Abs
        cpu.PC += 3                        // Increment Program Counter by 3 bytes (opcode + 2-byte address)
        break

    // ASL Zero Page,X
    case 0x16: // ASL ZpX
        address := ZpX(cpu, cart)         // Calculate Zero Page,X address
        ASL_M(cpu, cart, address)        // Perform ASL on memory
        cpu.CYC = 6                       // Set cycle count for ASL ZpX
        cpu.PC += 2                        // Increment Program Counter by 2 bytes (opcode + operand)
        break

    // ASL Absolute,X
    case 0x1E: // ASL AbsX
        address := AbsX(cpu, cart)         // Calculate Absolute,X address
        ASL_M(cpu, cart, address)        // Perform ASL on memory
        cpu.CYC = 7                       // Set cycle count for ASL AbsX
        cpu.PC += 3                        // Increment Program Counter by 3 bytes (opcode + 2-byte address)
        break



    

		case 0x46: // LSR Zero Page
        LSR_M(cpu, cart, Zp(cpu, cart))
        cpu.CYC = 5
        cpu.PC += 2
        break

    case 0x4A: // LSR Accumulator
        LSR_A(cpu)
        cpu.CYC = 2
        cpu.PC += 1
        break

    case 0x4E: // LSR Absolute
        LSR_M(cpu, cart, Abs(cpu, cart))
        cpu.CYC = 6
        cpu.PC += 3
        break

    case 0x56: // LSR Zero Page,X
        LSR_M(cpu, cart, ZpX(cpu, cart))
        cpu.CYC = 6
        cpu.PC += 2
        break

    
		case 0x66: // ROR Zp
        ROR(cpu, cart, Zp(cpu, cart), 0x66)
        cpu.CYC = 5
        cpu.PC += 2
		break

    case 0x6A: // ROR Acc
        ROR(cpu, cart, 0, 0x6A)
        cpu.CYC = 2
        cpu.PC += 1
		break
    
	case 0x6E: // ROR Abs
        ROR(cpu, cart, Abs(cpu, cart), 0x6E) // Changed from 0x66 to 0x6E
        cpu.CYC = 6
        cpu.PC += 3
		break

    case 0x76: // ROR ZpX
        ROR(cpu, cart, ZpX(cpu, cart), 0x76) // Changed from 0x6A to 0x76
        cpu.CYC = 6
        cpu.PC += 2
		break

    case 0x7E: // ROR AbX
        ROR(cpu, cart, AbsX(cpu, cart), 0x7E) // Changed from 0x7E
        cpu.CYC = 7
        cpu.PC += 3
		break


    case 0x26: // ROL Zp
        ROL(cpu, cart, Zp(cpu, cart), 0x26)
        cpu.CYC = 5
        cpu.PC += 2
		break

    case 0x2A: // ROL Acc
        ROL(cpu, cart, 0, 0x2A)
        cpu.CYC = 2
        cpu.PC += 1
		break
    
	case 0x2E: // ROL Abs
        ROL(cpu, cart, Abs(cpu, cart), 0x2E) // Changed from 0x26 to 0x2E
        cpu.CYC = 6
        cpu.PC += 3
		break

    case 0x36: // ROL ZpX
        ROL(cpu, cart, ZpX(cpu, cart), 0x36) // Changed from 0x2A to 0x36
        cpu.CYC = 6
        cpu.PC += 2
		break
    
	case 0x3E: // ROL AbX
        ROL(cpu, cart, AbsX(cpu, cart), 0x3E) // Changed from 0x3E
        cpu.CYC = 7
        cpu.PC += 3
		break



		case 0x61: // ADC (Indirect,X)
        addr := IndX(cpu, cart)
        value := RM(cpu, cart, addr)
        ADC(cpu, value)
        cpu.CYC = 6
        cpu.PC += 2
        break

    case 0x65: // ADC Zero Page
        addr := Zp(cpu, cart)
        value := RM(cpu, cart, addr)
        ADC(cpu, value)
        cpu.CYC = 3
        cpu.PC += 2
        break

		case 0x69: // ADC Immediate
		value := Imm(cpu, cart)
		ADC(cpu, byte(value)) // Cast uint16 to byte
		cpu.CYC = 2
		cpu.PC += 2
		break

		

    case 0x6D: // ADC Absolute
        addr := Abs(cpu, cart)
        value := RM(cpu, cart, addr)
        ADC(cpu, value)
        cpu.CYC = 4
        cpu.PC += 3
        break

    case 0x71: // ADC (Indirect),Y
        addr := IndY(cpu, cart)
        value := RM(cpu, cart, addr)
        ADC(cpu, value)
        cpu.CYC = 5
        cpu.PC += 2
        if cpu.PageCrossed == 1 {
            cpu.CYC++
        }
        break

    case 0x75: // ADC Zero Page,X
        addr := ZpX(cpu, cart)
        value := RM(cpu, cart, addr)
        ADC(cpu, value)
        cpu.CYC = 4
        cpu.PC += 2
        break

    case 0x79: // ADC Absolute,Y
        addr := AbsY(cpu, cart)
        value := RM(cpu, cart, addr)
        ADC(cpu, value)
        cpu.CYC = 4
        cpu.PC += 3
        if cpu.PageCrossed == 1 {
            cpu.CYC++
        }
        break

    case 0x7D: // ADC Absolute,X
        addr := AbsX(cpu, cart)
        value := RM(cpu, cart, addr)
        ADC(cpu, value)
        cpu.CYC = 4
        if cpu.PageCrossed == 1 {
            cpu.CYC++
        }
        cpu.PC += 3
        break



// Add this case to your switch statement in the emulate function
case 0x5E: // LSR Absolute,X
    address := AbsX(cpu, cart)    // Calculate Absolute,X address
    LSR_M(cpu, cart, address)     // Perform LSR on memory
    cpu.CYC = 7                   // Set cycle count for LSR Absolute,X
    cpu.PC += 3                   // Increment Program Counter by 3 bytes (opcode + 2-byte address)
    break




		
			
			default:
				
				fmt.Printf("Opcode not supported: %X \n", RM(cpu, cart, cpu.PC))
				if cpu.D.Enable {
					fmt.Printf("%s\n",cpu.D.Lines[cpu.SwitchTimes])
				}
				
				cpu.Running = false
	}
	
}

func Verbose(cpu *CPU, cart *cartridge.Cartridge) {
	fmt.Printf("%4X  %2X  %2X %2X                       A:%2X X:%2X Y:%2X P:%2X SP:%2X CYC:%d SL: %d\n", cpu.PC, RM(cpu, cart, cpu.PC), RM(cpu, cart, cpu.PC+1), RM(cpu, cart, cpu.PC+2), cpu.A, cpu.X, cpu.Y, cpu.P, cpu.SP, 0, 0 )
}
