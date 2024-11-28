package channels

// Duty cycle lookup table for pulse waves
var DutyTable = [4][8]byte{
    {0, 1, 0, 0, 0, 0, 0, 0}, // 12.5%
    {0, 1, 1, 0, 0, 0, 0, 0}, // 25%
    {0, 1, 1, 1, 1, 0, 0, 0}, // 50%
    {1, 0, 0, 1, 1, 1, 1, 1}, // 75% (inverted 25%)
}

// PulseChannel represents a single pulse wave channel in the NES APU
type PulseChannel struct {
    enabled     bool
    dutyMode    byte
    dutyPos     byte
    volume      byte
    period      uint16
    lengthCount byte
    timer       uint16
}

// NewPulseChannel creates and initializes a new pulse channel
func NewPulseChannel() *PulseChannel {
    return &PulseChannel{
        enabled:  true,
        dutyMode: 0,
        volume:   15,
        period:   0,
    }
}

// WriteRegister handles writes to the pulse channel's registers
func (p *PulseChannel) WriteRegister(addr uint16, value byte) {
    switch addr & 3 {
    case 0: // 0x4000/0x4004 - Duty cycle and volume
        p.dutyMode = (value >> 6) & 3
        p.volume = value & 0x0F
    case 2: // 0x4002/0x4006 - Period low
        p.period = (p.period & 0xFF00) | uint16(value)
    case 3: // 0x4003/0x4007 - Period high and length counter
        p.period = (p.period & 0x00FF) | (uint16(value&7) << 8)
        p.lengthCount = value >> 3
        p.dutyPos = 0 // Reset duty position
    }
}

// SetEnabled enables or disables the channel
func (p *PulseChannel) SetEnabled(enabled bool) {
    p.enabled = enabled
    if !enabled {
        p.lengthCount = 0
    }
}

// Clock advances the channel's state by one CPU cycle
func (p *PulseChannel) Clock() {
    if !p.enabled || p.period < 8 { // Prevent very high frequencies
        return
    }

    p.timer--
    if p.timer == 0 {
        p.timer = p.period
        p.dutyPos = (p.dutyPos + 1) & 7
    }
}

// GetSample returns the current sample value for the channel
func (p *PulseChannel) GetSample() float32 {
    if !p.enabled || p.lengthCount == 0 {
        return 0
    }

    if DutyTable[p.dutyMode][p.dutyPos] != 0 {
        return float32(p.volume) / 15.0
    }
    return 0
}