// File: apu/channels/envelope.go
package channels

// EnvelopeUnit represents the volume envelope generator
type EnvelopeUnit struct {
    start      bool
    loop       bool
    constant   bool
    value      byte
    divider    byte
    counter    byte
    decayLevel byte
}

// Clock advances the envelope unit's state
func (e *EnvelopeUnit) Clock() {
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

// SetStart sets the start flag of the envelope
func (e *EnvelopeUnit) SetStart(start bool) {
    e.start = start
}

// Reset initializes the envelope unit to its default power-up state
func (e *EnvelopeUnit) Reset() {
    e.start      = false
    e.loop       = false
    e.constant   = false
    e.value      = 0
    e.divider    = 0
    e.counter    = 0
    e.decayLevel = 0
}