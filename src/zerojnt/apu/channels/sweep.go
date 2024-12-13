package channels

// SweepUnit represents the sweep unit of a pulse channel
type SweepUnit struct {
	enabled bool
	divider byte
	counter byte
	reload  bool
	negate  bool
	shift   byte
}

func NewSweepUnit() *SweepUnit {
	return &SweepUnit{
		enabled: false,
		divider: 0,
		counter: 0,
		reload:  false,
		negate:  false,
		shift:   0,
	}
}

func (s *SweepUnit) Write(value byte) {
	s.enabled = (value & 0x80) != 0
	s.divider = (value >> 4) & 0x07
	s.negate = (value & 0x08) != 0
	s.shift = value & 0x07
	s.reload = true
}

func (s *SweepUnit) Clock() {
	if s.counter == 0 && s.enabled {
		s.counter = s.divider
	} else if s.counter > 0 {
		s.counter--
	}

	if s.reload {
		s.counter = s.divider
		s.reload = false
	}
}

// Reset initializes the sweep unit to its default power-up state
func (s *SweepUnit) Reset() {
    s.enabled = false
    s.divider = 0
    s.counter = 0
    s.reload = false
    s.negate = false
    s.shift = 0
}