package channels

// Triangle channel period lookup table
const TRIANGLE_PERIOD_TABLE = 32

// TriangleChannel represents the triangle wave channel in the NES APU
type TriangleChannel struct {
    enabled       bool
    period       uint16
    lengthCount  byte
    linearCount  byte
    linearReload byte
    linearControl bool
    sequencePos  byte
    timer        uint16
    // Triangle wave sequence (32 steps)
    sequence     [32]byte
}

// Triangle wave sequence (15,14,13,12,11,10,9,8,7,6,5,4,3,2,1,0,0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15)
func NewTriangleChannel() *TriangleChannel {
    t := &TriangleChannel{
        enabled: true,
        sequencePos: 0,
    }
    
    // Initialize the triangle sequence
    for i := 0; i < 16; i++ {
        t.sequence[i] = byte(15 - i)
        t.sequence[i+16] = byte(i)
    }
    
    return t
}

// WriteRegister handles writes to the triangle channel's registers
func (t *TriangleChannel) WriteRegister(addr uint16, value byte) {
    switch addr {
    case 0x4008: // Linear counter
        t.linearReload = value & 0x7F
        t.linearControl = value&0x80 != 0
    case 0x400A: // Timer low
        t.period = (t.period & 0xFF00) | uint16(value)
    case 0x400B: // Length counter load and timer high
        t.period = (t.period & 0x00FF) | (uint16(value&0x07) << 8)
        t.lengthCount = value >> 3
        t.linearCount = t.linearReload
    }
}

// Clock advances the triangle channel state
func (t *TriangleChannel) Clock() {
    if !t.enabled || t.period < 2 { // Prevent very high frequencies
        return
    }

    t.timer--
    if t.timer == 0 {
        t.timer = t.period
        if t.lengthCount > 0 && t.linearCount > 0 {
            t.sequencePos = (t.sequencePos + 1) & 0x1F // Wrap at 32 steps
        }
    }
}

// ClockLinearCounter handles the linear counter
func (t *TriangleChannel) ClockLinearCounter() {
    if t.linearControl {
        if t.linearCount > 0 {
            t.linearCount--
        }
    } else {
        t.linearCount = t.linearReload
    }
}

// GetSample returns the current sample value for the triangle channel
func (t *TriangleChannel) GetSample() float32 {
    if !t.enabled || t.lengthCount == 0 || t.linearCount == 0 {
        return 0
    }
    
    // Convert the current sequence value (0-15) to float32 (-1 to 1)
    return float32(t.sequence[t.sequencePos]) / 7.5 - 1.0
}

func (t *TriangleChannel) SetEnabled(enabled bool) {
    t.enabled = enabled
    if !enabled {
        t.lengthCount = 0
        t.linearCount = 0
    }
}
