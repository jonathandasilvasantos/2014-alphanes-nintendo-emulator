
package cpu

// Branch applies the 6502 branching rules:
//   • +1 cycle when the branch is taken.
//   • +1 additional cycle if the branch destination crosses a page
//     boundary (i.e. the high‑order byte changes).
// The caller is expected to have preset cpu.CYCSpecial to 0 before the
// call.  The helper updates CYCSpecial with the correct penalty and
// finally sets the program counter to the destination address.
func Branch(cpu *CPU, target uint16) {
    // Cycle penalty for a taken branch (always +1).
    cpu.CYCSpecial++

    // Reference PC after the operand byte has been fetched (PC+2).
    oldPC := cpu.PC + 2

    // Extra cycle if destination is on a different 256‑byte page.
    if (oldPC & 0xFF00) != (target & 0xFF00) {
        cpu.CYCSpecial++
    }

    // Jump to the computed target address.
    cpu.PC = target
}
