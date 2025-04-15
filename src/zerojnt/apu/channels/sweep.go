// File: apu/channels/sweep.go
package channels

// SweepUnit handles the pitch sweeping effect for Pulse channels.
type SweepUnit struct {
	channelNum int // 1 or 2, needed for negate behavior difference

	// Register Values ($4001 / $4005)
	enabled bool // Bit 7: Sweep enabled
	period  byte // Bits 6-4: Divider period (0-7)
	negate  bool // Bit 3: Negate flag (0: add, 1: subtract)
	shift   byte // Bits 2-0: Shift count (0-7)

	// Internal State
	dividerCounter byte   // Counts down from 'period'
	reload         bool   // Flag to reload dividerCounter on next clock
	targetPeriod   uint16 // The calculated target period after sweep adjustment (for muting check)
}

// NewSweepUnit creates and initializes a new SweepUnit.
func NewSweepUnit() *SweepUnit {
	s := &SweepUnit{}
	s.Reset()
	return s
}

// Reset initializes the sweep unit to its power-up state.
func (s *SweepUnit) Reset() {
	s.enabled = false
	s.period = 0
	s.negate = false
	s.shift = 0
	s.dividerCounter = 0
	s.reload = false
	s.targetPeriod = 0
}

// Write updates the sweep unit's settings from a register write ($4001/$4005).
func (s *SweepUnit) Write(value byte) {
	s.enabled = (value & 0x80) != 0
	s.period = (value >> 4) & 0x07
	s.negate = (value & 0x08) != 0
	s.shift = value & 0x07
	s.reload = true // Request a reload of the divider counter
}

// Clock advances the sweep unit's state. Called by the frame counter.
// It returns the potentially modified timer period for the pulse channel.
func (s *SweepUnit) Clock(currentPeriod uint16) uint16 {
	// Calculate the change amount based on current period and shift count
	change := currentPeriod >> s.shift

	// Calculate the target period after applying the change
	if s.negate {
		s.targetPeriod = currentPeriod - change
		// Pulse channel 1 subtracts an extra 1 in negate mode
		if s.channelNum == 1 {
			s.targetPeriod--
		}
	} else {
		s.targetPeriod = currentPeriod + change
	}

	// Muting condition check (independent of divider clocking)
	muted := s.isMuting(currentPeriod)

	// Clock the divider
	if s.dividerCounter == 0 && s.enabled && s.shift > 0 && !muted {
		// Divider is zero, sweep is enabled, shift is non-zero, and not muted: Apply the sweep.
		currentPeriod = s.targetPeriod // Update the channel's period

		// Check for overflow after update (another muting condition)
		if currentPeriod > 0x7FF {
			currentPeriod = 0x7FF // Clamp or mute? Let's clamp for now. Muting handled by isMuting.
		}
		if currentPeriod < 8 { // Ensure minimum period
             // If period becomes less than 8, it mutes, but the value itself doesn't necessarily clamp?
             // Let the pulse channel handle muting based on period < 8.
		}
	}

	// Reload or decrement the divider counter
	if s.dividerCounter == 0 || s.reload {
		s.dividerCounter = s.period // Reload counter
		s.reload = false            // Clear reload flag
	} else if s.enabled { // Only decrement if enabled? NESDev implies yes.
		s.dividerCounter--
	}

	// Return the (possibly updated) period
	return currentPeriod
}

// isMuting checks if the sweep unit should mute the channel.
// Muting occurs if the target period is > $7FF OR the current period < 8.
func (s *SweepUnit) isMuting(currentPeriod uint16) bool {
	if currentPeriod < 8 {
		return true
	}
	if s.targetPeriod > 0x7FF {
		return true
	}
	return false
}