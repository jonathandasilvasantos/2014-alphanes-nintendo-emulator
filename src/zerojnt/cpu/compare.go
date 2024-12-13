package cpu

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"zerojnt/cartridge"
)

// Helper function to extract register values from a log line
func extractRegisterValue(line, registerName string) (uint64, error) {
    startIndex := strings.Index(line, registerName)
    if startIndex == -1 {
        return 0, fmt.Errorf("register %s not found in log line", registerName)
    }
    valueStr := line[startIndex+len(registerName) : startIndex+len(registerName)+2]
    return strconv.ParseUint(valueStr, 16, 8)
}

// Helper function to extract a 16-bit value from a log line (e.g., PC)
func extractUint16Value(line, identifier string) (uint16, error) {
    startIndex := strings.Index(line, identifier)
    if startIndex == -1 {
        return 0, fmt.Errorf("identifier %s not found in log line", identifier)
    }
    valueStr := line[startIndex+len(identifier) : startIndex+len(identifier)+4]
    value, err := strconv.ParseUint(valueStr, 16, 16)
    if err != nil {
        return 0, err
    }
    return uint16(value), nil
}

// Helper function to extract the opcode from a log line
func extractOpcode(line string) (byte, error) {
    fields := strings.Fields(line)
    if len(fields) < 2 {
        return 0, fmt.Errorf("invalid log line format: insufficient fields")
    }
    opcodeStr := fields[1]
    opcode, err := strconv.ParseUint(opcodeStr, 16, 8)
    if err != nil {
        return 0, err
    }
    return byte(opcode), nil
}

// DebugCompare compares the current CPU state with the expected state from the debug log
func DebugCompare(cpu *CPU, cart *cartridge.Cartridge) {
    if cpu.SwitchTimes >= len(cpu.D.Lines) {
        fmt.Println("Error: Attempting to compare beyond the available debug lines.")
        cpu.Running = false
        return
    }
    debugLine := cpu.D.Lines[cpu.SwitchTimes]

    // Extract expected values from the debug line
    expectedA, errA := extractRegisterValue(debugLine, "A:")
    expectedX, errX := extractRegisterValue(debugLine, "X:")
    expectedY, errY := extractRegisterValue(debugLine, "Y:")
    expectedP, errP := extractRegisterValue(debugLine, "P:")
    expectedSP, errSP := extractRegisterValue(debugLine, "SP:")
    expectedPC, errPC := extractUint16Value(debugLine, "") // PC is at the beginning
    expectedOpcode, errOpcode := extractOpcode(debugLine)

    if errA != nil || errX != nil || errY != nil || errP != nil || errSP != nil || errPC != nil || errOpcode != nil {
        log.Fatalf("Error parsing debug line %d: %v, %v, %v, %v, %v, %v, %v", cpu.SwitchTimes, errA, errX, errY, errP, errSP, errPC, errOpcode)
    }

    var err bool = false

    // Compare actual CPU state with expected values
    if uint64(cpu.A) != expectedA {
        fmt.Printf("Error at line %d: A mismatch: Expected %02X, Got %02X\n", cpu.SwitchTimes, expectedA, cpu.A)
        err = true
    }
    if uint64(cpu.X) != expectedX {
        fmt.Printf("Error at line %d: X mismatch: Expected %02X, Got %02X\n", cpu.SwitchTimes, expectedX, cpu.X)
        err = true
    }
    if uint64(cpu.Y) != expectedY {
        fmt.Printf("Error at line %d: Y mismatch: Expected %02X, Got %02X\n", cpu.SwitchTimes, expectedY, cpu.Y)
        err = true
    }
    if uint64(cpu.P) != expectedP {
        fmt.Printf("Error at line %d: P mismatch: Expected %02X, Got %02X\n", cpu.SwitchTimes, expectedP, cpu.P)
        err = true
    }
    if uint64(cpu.SP) != expectedSP {
        fmt.Printf("Error at line %d: SP mismatch: Expected %02X, Got %02X\n", cpu.SwitchTimes, expectedSP, cpu.SP)
        err = true
    }
    if cpu.PC != expectedPC {
        fmt.Printf("Error at line %d: PC mismatch: Expected %04X, Got %04X\n", cpu.SwitchTimes, expectedPC, cpu.PC)
        err = true
    }
    if RM(cpu, cart, cpu.PC) != byte(expectedOpcode) {
        fmt.Printf("Error at line %d: Opcode mismatch: Expected %02X, Got %02X\n", cpu.SwitchTimes, expectedOpcode, RM(cpu, cart, cpu.PC))
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