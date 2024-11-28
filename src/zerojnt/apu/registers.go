package apu

const (
    // Pulse 1 registers
    PULSE1_CONTROL = 0x4000
    PULSE1_SWEEP   = 0x4001
    PULSE1_TIMER_L = 0x4002
    PULSE1_TIMER_H = 0x4003

    // Pulse 2 registers
    PULSE2_CONTROL = 0x4004
    PULSE2_SWEEP   = 0x4005
    PULSE2_TIMER_L = 0x4006
    PULSE2_TIMER_H = 0x4007

    // Status register
    STATUS_REG = 0x4015

    // Frame counter register
    FRAME_COUNTER = 0x4017
)

type RegisterHandler struct {
    apu *APU
}

func NewRegisterHandler(apu *APU) *RegisterHandler {
    return &RegisterHandler{
        apu: apu,
    }
}

func (r *RegisterHandler) Write(addr uint16, value byte) {
    switch {
    case addr >= 0x4000 && addr <= 0x4007:
        r.apu.WriteRegister(addr, value)
    case addr == STATUS_REG:
        r.apu.WriteRegister(addr, value)
    case addr == FRAME_COUNTER:
        // Frame counter control - not implemented yet
    }
}

func (r *RegisterHandler) Read(addr uint16) byte {
    if addr == STATUS_REG {
        // Return channel status
        var status byte = 0
        // TODO: Implement status bits
        return status
    }
    return 0
}