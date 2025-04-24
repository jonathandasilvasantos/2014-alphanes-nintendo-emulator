// File: apu/channels/envelope.go
package channels

// EnvelopeUnit represents the volume envelope generator used by Pulse and Noise channels.
type EnvelopeUnit struct {
	start          bool // Flag to restart the envelope
	loop           bool // Flag to loop the envelope decay
	constant       bool // Flag for constant volume mode
	dividerPeriod  byte // Reload value for divider (0-15)
	dividerCounter byte // Counts down from dividerPeriod
	decayLevel     byte // Current volume level (0-15)
}

// Reset initializes the envelope unit to its power-up state.
func (e *EnvelopeUnit) Reset() {
	e.start = false
	e.loop = false
	e.constant = false
	e.dividerPeriod = 0
	e.dividerCounter = 0
	e.decayLevel = 0
}

// Clock advances the envelope unit's state.
func (e *EnvelopeUnit) Clock() {
	// Check if envelope needs restarting
	if e.start {
		e.start = false
		e.decayLevel = 15
		e.dividerCounter = e.dividerPeriod
		return
	}

	// Clock the divider
	if e.dividerCounter > 0 {
		e.dividerCounter--
	} else {
		// Reload divider when it reaches zero
		e.dividerCounter = e.dividerPeriod

		// Update decay level
		if e.decayLevel > 0 {
			e.decayLevel--
		} else if e.loop {
			e.decayLevel = 15
		}
	}
}

// Volume returns the current output volume (0-15)
func (e *EnvelopeUnit) Volume() byte {
	if e.constant {
		return e.dividerPeriod
	}
	return e.decayLevel
}