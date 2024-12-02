package channels

var NOISE_PERIOD_TABLE = [16]uint16{
    4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 380, 508, 762, 1016, 2034, 4068,
}

type EnvelopeUnit struct {
    start      bool
    loop       bool
    constant   bool
    value      byte
    divider    byte
    counter    byte
    decayLevel byte
}

// NoiseChannel represents the noise generator channel in the NES APU
type NoiseChannel struct {
    enabled        bool
    mode           bool    // false = long mode (93-bit), true = short mode (32767 steps)
    shiftRegister  uint16  // 15-bit shift register
    period         uint16
    timer          uint16
    volume         byte
    lengthCount    byte
    envelope       EnvelopeUnit
    useEnvelope    bool
}

func NewNoiseChannel() *NoiseChannel {
    return &NoiseChannel{
        enabled:       true,
        shiftRegister: 1,    // Initialize with 1
        period:        0,
        volume:        15,
        envelope: EnvelopeUnit{
            value:      15,
            divider:    0,
            counter:    0,
            decayLevel: 15,
        },
    }
}

// WriteRegister handles writes to the noise channel's registers
func (n *NoiseChannel) WriteRegister(addr uint16, value byte) {
    switch addr & 3 {
    case 0: // $400C - Volume/Envelope
        n.useEnvelope = (value & 0x10) == 0
        if n.useEnvelope {
            n.envelope.value = value & 0x0F
        } else {
            n.volume = value & 0x0F
        }
        n.envelope.loop = (value & 0x20) != 0
    case 2: // $400E - Mode/Period
        n.mode = (value & 0x80) != 0
        n.period = NOISE_PERIOD_TABLE[value&0x0F]
        n.timer = n.period
    case 3: // $400F - Length counter load and envelope restart
        if n.enabled {
            n.lengthCount = value >> 3
        }
        n.envelope.start = true
    }
}

// Clock advances the noise channel state
func (n *NoiseChannel) Clock() {
    if !n.enabled {
        return
    }

    // Clock timer
    if n.timer > 0 {
        n.timer--
    }

    if n.timer == 0 {
        n.timer = n.period

        // Calculate feedback bit
        var feedback uint16
        if n.mode {
            // Short mode (32767 steps)
            feedback = ((n.shiftRegister & 0x01) ^ ((n.shiftRegister >> 6) & 0x01)) & 0x01
        } else {
            // Long mode (93-bit)
            feedback = ((n.shiftRegister & 0x01) ^ ((n.shiftRegister >> 1) & 0x01)) & 0x01
        }

        // Shift right by one
        n.shiftRegister = (n.shiftRegister >> 1) | (feedback << 14)
    }

    // Clock envelope
    if n.envelope.start {
        n.envelope.start = false
        n.envelope.decayLevel = 15
        n.envelope.counter = n.envelope.value
    } else if n.envelope.counter > 0 {
        n.envelope.counter--
    } else {
        n.envelope.counter = n.envelope.value
        if n.envelope.decayLevel > 0 {
            n.envelope.decayLevel--
        } else if n.envelope.loop {
            n.envelope.decayLevel = 15
        }
    }
}

// GetSample returns the current sample value for the noise channel
func (n *NoiseChannel) GetSample() float32 {
    if !n.enabled || n.lengthCount == 0 {
        return 0
    }

    if (n.shiftRegister & 0x01) == 0 {
        if n.useEnvelope {
            return float32(n.envelope.decayLevel) / 15.0
        }
        return float32(n.volume) / 15.0
    }
    return 0
}

func (n *NoiseChannel) SetEnabled(enabled bool) {
    n.enabled = enabled
    if !enabled {
        n.lengthCount = 0
    }
}

// ClockLengthCounter clocks the length counter if enabled
func (n *NoiseChannel) ClockLengthCounter() {
    if n.enabled && n.lengthCount > 0 && !n.envelope.loop {
        n.lengthCount--
    }
}