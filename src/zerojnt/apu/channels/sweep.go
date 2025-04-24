// File: apu/channels/sweep.go
package channels

// SweepUnit handles pitch sweeping for Pulse channels
type SweepUnit struct {
	channelNum int // 1 or 2, affects negate behavior

	// Register Values ($4001 / $4005)
	enabled bool // Sweep enabled flag
	period  byte // Divider period (0-7)
	negate  bool // Negate flag (0: add, 1: subtract)
	shift   byte // Shift count (0-7)

	// Internal State
	dividerCounter byte   // Counts down from period
	reload         bool   // Flag to reload divider
	targetPeriod   uint16 // Calculated target period after sweep
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

// isMuting checks if the channel should be muted
func (s *SweepUnit) isMuting(currentPeriod uint16) bool {
	if currentPeriod < 8 {
		return true
	}
	if s.targetPeriod > 0x7FF {
		return true
	}
	return false
}