package apu

var LengthTable = []byte{
    10, 254, 20,  2, 40,  4, 80,  6,
    160,  8, 60, 10, 14, 12, 26, 14,
    12, 16, 24, 18, 48, 20, 96, 22,
    192, 24, 72, 26, 16, 28, 32, 30,
}

type LengthCounter struct {
    counter byte
    enabled bool
    halt    bool
}

func NewLengthCounter() *LengthCounter {
    return &LengthCounter{
        counter: 0,
        enabled: false,
        halt:    false,
    }
}

func (l *LengthCounter) Load(value byte) {
    if l.enabled {
        l.counter = LengthTable[value>>3]
    }
}

func (l *LengthCounter) Clock() {
    if !l.halt && l.counter > 0 {
        l.counter--
    }
}

func (l *LengthCounter) SetEnabled(enabled bool) {
    l.enabled = enabled
    if !enabled {
        l.counter = 0
    }
}