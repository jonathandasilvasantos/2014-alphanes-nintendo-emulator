package cpu

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"zerojnt/cartridge"
)

func GetA(debugLine string) string {
	idx := strings.Index(debugLine, "A:")
	if idx == -1 {
		return ""
	}
	rest := debugLine[idx+2:]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func GetX(debugLine string) string {
	idx := strings.Index(debugLine, "X:")
	if idx == -1 {
		return ""
	}
	rest := debugLine[idx+2:]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func GetY(debugLine string) string {
	idx := strings.Index(debugLine, "Y:")
	if idx == -1 {
		return ""
	}
	rest := debugLine[idx+2:]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func GetP(debugLine string) string {
	idx := strings.Index(debugLine, "P:")
	if idx == -1 {
		return ""
	}
	rest := debugLine[idx+2:]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func GetSP(debugLine string) string {
	idx := strings.Index(debugLine, "SP:")
	if idx == -1 {
		return ""
	}
	rest := debugLine[idx+3:] // SP is "SP:"
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func GetPC(debugLine string) string {
	// PC is at the start of the line
	// Assuming PC is the first 4 characters
	if len(debugLine) < 4 {
		return ""
	}
	return debugLine[:4]
}

func GetOpcode(debugLine string) string {
	// The opcode is the first byte after the PC
	// Let's split the line by spaces
	fields := strings.Fields(debugLine)
	if len(fields) < 2 {
		return ""
	}
	// The opcode is the first byte in the bytes field
	return fields[1]
}

func DebugA(cpu *CPU, cart *cartridge.Cartridge) byte {
	debugLine := cpu.D.Lines[cpu.SwitchTimes+1]
	A_str := GetA(debugLine)
	A, errA := strconv.ParseUint(A_str, 16, 8)
	if errA != nil {
		log.Fatal(errA)
	}
	return byte(A)
}

func DebugX(cpu *CPU, cart *cartridge.Cartridge) byte {
	debugLine := cpu.D.Lines[cpu.SwitchTimes+1]
	X_str := GetX(debugLine)
	X, errX := strconv.ParseUint(X_str, 16, 8)
	if errX != nil {
		log.Fatal(errX)
	}
	return byte(X)
}

func DebugY(cpu *CPU, cart *cartridge.Cartridge) byte {
	debugLine := cpu.D.Lines[cpu.SwitchTimes+1]
	Y_str := GetY(debugLine)
	Y, errY := strconv.ParseUint(Y_str, 16, 8)
	if errY != nil {
		log.Fatal(errY)
	}
	return byte(Y)
}

func DebugP(cpu *CPU, cart *cartridge.Cartridge) byte {
	debugLine := cpu.D.Lines[cpu.SwitchTimes+1]
	P_str := GetP(debugLine)
	P, errP := strconv.ParseUint(P_str, 16, 8)
	if errP != nil {
		log.Fatal(errP)
	}
	return byte(P)
}

func DebugOp(cpu *CPU, cart *cartridge.Cartridge) byte {
	debugLine := cpu.D.Lines[cpu.SwitchTimes+1]
	OP_str := GetOpcode(debugLine)
	OP, errOP := strconv.ParseUint(OP_str, 16, 8)
	if errOP != nil {
		log.Fatal(errOP)
	}
	return byte(OP)
}

func DebugCompare(cpu *CPU, cart *cartridge.Cartridge) {
	debugLine := cpu.D.Lines[cpu.SwitchTimes]

	A_str := GetA(debugLine)
	X_str := GetX(debugLine)
	Y_str := GetY(debugLine)
	P_str := GetP(debugLine)
	SP_str := GetSP(debugLine)
	PC_str := GetPC(debugLine)

	A, errA := strconv.ParseUint(A_str, 16, 8)
	X, errX := strconv.ParseUint(X_str, 16, 8)
	Y, errY := strconv.ParseUint(Y_str, 16, 8)
	P, errP := strconv.ParseUint(P_str, 16, 8)
	SP, errSP := strconv.ParseUint(SP_str, 16, 8)
	PC, errPC := strconv.ParseUint(PC_str, 16, 16)

	if errA != nil {
		log.Fatal(errA)
	}
	if errX != nil {
		log.Fatal(errX)
	}
	if errY != nil {
		log.Fatal(errY)
	}
	if errP != nil {
		log.Fatal(errP)
	}
	if errSP != nil {
		log.Fatal(errSP)
	}
	if errPC != nil {
		log.Fatal(errPC)
	}

	var err bool = false

	if A != uint64(cpu.A) {
		fmt.Printf("Error: A:%02X Debug A:%02X\n", uint64(cpu.A), A)
		err = true
	}

	if X != uint64(cpu.X) {
		fmt.Printf("Error: X:%02X Debug X:%02X\n", uint64(cpu.X), X)
		err = true
	}

	if Y != uint64(cpu.Y) {
		fmt.Printf("Error: Y:%02X Debug Y:%02X\n", uint64(cpu.Y), Y)
		err = true
	}

	if P != uint64(cpu.P) {
		fmt.Printf("Error: P:%02X Debug P:%02X\n", uint64(cpu.P), P)
		err = true
	}

	if SP != uint64(cpu.SP) {
		fmt.Printf("Error: SP:%02X Debug SP:%02X\n", uint64(cpu.SP), SP)
		err = true
	}

	if PC != uint64(cpu.PC) {
		fmt.Printf("Error: PC:%04X Debug PC:%04X\n", uint64(cpu.PC), PC)
		err = true
	}

	if err {
		fmt.Printf("Error at line: %d -- %02X %02X %02X - SwitchTime: %d\n", cpu.SwitchTimes, RM(cpu, cart, cpu.PC), RM(cpu, cart, cpu.PC+1), RM(cpu, cart, cpu.PC+2), cpu.SwitchTimes)
		for i := 3; i > 0; i-- {
			idx := cpu.SwitchTimes - i
			if idx >= 0 && idx < len(cpu.D.Lines) {
				fmt.Printf("%s\n", cpu.D.Lines[idx])
			}
		}
		fmt.Printf("%s\n", debugLine)
		cpu.Running = false
	}
}
