// File: apu/channels/sweep.go
package channels

// SweepUnit handles pitch sweeping for Pulse channels
type SweepUnit struct {
	channelNum int // Channel number (1 or 2)

	// Register values
	enabled bool // Sweep enabled flag
	period  byte // Divider period
	negate  bool // Direction (false: add, true: subtract)
	shift   byte // Shift count

	// Internal state
	dividerCounter byte   // Divider counter
	reload         bool   // Reload flag
	targetPeriod   uint16 // Target period after sweep
}

// NewSweepUnit creates a new SweepUnit
func NewSweepUnit() *SweepUnit {
	s := &SweepUnit{}
	s.Reset()
	return s
}

// Reset initializes the sweep unit
func (s *SweepUnit) Reset() {
	s.enabled = false
	s.period = 0
	s.negate = false
	s.shift = 0
	s.dividerCounter = 0
	s.reload = false
	s.targetPeriod = 0
}

// Write updates sweep settings from register write
func (s *SweepUnit) Write(value byte) {
	s.enabled = (value & 0x80) != 0
	s.period = (value >> 4) & 0x07
	s.negate = (value & 0x08) != 0
	s.shift = value & 0x07
	s.reload = true
}

// Clock advances the sweep unit state
func (s *SweepUnit) Clock(currentPeriod uint16) uint16 {
	// Calculate change amount
	change := currentPeriod >> s.shift

	// Calculate target period
	if s.negate {
		s.targetPeriod = currentPeriod - change
		// Channel 1 subtracts an extra 1 in negate mode
		if s.channelNum == 1 {
			s.targetPeriod--
		}
	} else {
		s.targetPeriod = currentPeriod + change
	}

	// Check muting condition
	muted := s.isMuting(currentPeriod)

	// Apply sweep if conditions met
	if s.dividerCounter == 0 && s.enabled && s.shift > 0 && !muted {
		currentPeriod = s.targetPeriod

		// Check for overflow
		if currentPeriod > 0x7FF {
			currentPeriod = 0x7FF
		}
	}

	// Handle divider counter
	if s.dividerCounter == 0 || s.reload {
		s.dividerCounter = s.period
		s.reload = false
	} else if s.enabled {
		s.dividerCounter--
	}

	return currentPeriod
}

// isMuting returns true when the channel should be silenced
func (s *SweepUnit) isMuting(period uint16) bool {
	if period < 8 {
		return true
	}
	if s.shift == 0 {
		return false
	}

	// Calculate target period
	delta := period >> s.shift
	var target uint16
	if s.negate {
		target = period - delta - btoi16(s.channelNum == 1)
	} else {
		target = period + delta
	}
	return target > 0x7FF
}

// Convert bool to uint16
func btoi16(b bool) uint16 {
	if b {
		return 1
	}
	return 0
}