// File: apu/channels/envelope.go
package channels

// EnvelopeUnit represents the volume envelope generator used by Pulse and Noise channels.
type EnvelopeUnit struct {
	start          bool // Flag to indicate the envelope should restart
	loop           bool // Flag to loop the envelope decay (also length counter halt)
	constant       bool // Flag to use constant volume instead of decay envelope
	dividerPeriod  byte // Reload value for the divider (also the constant volume level) (0-15)
	dividerCounter byte // Counts down from dividerPeriod
	decayLevel     byte // Current decay level (volume) (0-15)
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

// Clock advances the envelope unit's state. Called by the frame counter.
func (e *EnvelopeUnit) Clock() {
	// Check if envelope needs restarting
	if e.start {
		e.start = false      // Clear start flag
		e.decayLevel = 15    // Reset decay level to maximum
		e.dividerCounter = e.dividerPeriod // Reload divider counter
		return
	}

	// Clock the divider
	if e.dividerCounter > 0 {
		e.dividerCounter--
	} else {
		// Divider reached zero, reload it
		e.dividerCounter = e.dividerPeriod

		// Clock the decay level counter
		if e.decayLevel > 0 {
			e.decayLevel-- // Decrease volume level
		} else if e.loop { // If decay level is 0 and loop flag is set...
			e.decayLevel = 15 // ...wrap around to maximum volume
		}
	}
}

// --- Getters (Not strictly necessary but can be useful) ---

// Volume returns the current output volume of the envelope (0-15).
// Takes into account constant volume mode.
func (e *EnvelopeUnit) Volume() byte {
	if e.constant {
		return e.dividerPeriod // Use constant volume level
	}
	return e.decayLevel // Use current decay level
}