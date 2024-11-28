package apu

type Envelope struct {
    start      bool
    loop       bool
    constant   bool
    value      byte
    divider    byte
    counter    byte
    decayLevel byte
}

func NewEnvelope() *Envelope {
    return &Envelope{
        value:      15,
        divider:    0,
        counter:    0,
        decayLevel: 15,
    }
}

func (e *Envelope) Write(value byte) {
    e.loop = value&0x20 != 0
    e.constant = value&0x10 != 0
    e.value = value & 0x0F
}

func (e *Envelope) Clock() {
    if e.start {
        e.start = false
        e.decayLevel = 15
        e.counter = e.value
        return
    }

    if e.counter > 0 {
        e.counter--
        return
    }

    e.counter = e.value
    if e.decayLevel > 0 {
        e.decayLevel--
    } else if e.loop {
        e.decayLevel = 15
    }
}

func (e *Envelope) Output() byte {
    if e.constant {
        return e.value
    }
    return e.decayLevel
}