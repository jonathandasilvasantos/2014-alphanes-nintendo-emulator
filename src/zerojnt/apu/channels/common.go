// File: apu/channels/common.go
package channels

import "math"

// Length counter lookup table used by pulse, triangle, and noise channels
var LengthTable = []byte{
	10, 254, 20, 2, 40, 4, 80, 6,
	160, 8, 60, 10, 14, 12, 26, 14,
	12, 16, 24, 18, 48, 20, 96, 22,
	192, 24, 72, 26, 16, 28, 32, 30,
}

// polyBlepCorrection calculates anti-aliasing correction for waveform discontinuities
func polyBlepCorrection(phase uint32, phaseInc uint32) float32 {
	if phaseInc == 0 {
		return 0.0
	}

	dt_norm := float64(phaseInc) / math.Exp2(32)
	phase_norm := float64(phase) / math.Exp2(32)

	var correction float32 = 0.0

	if phase_norm < dt_norm { // Rising edge correction
		t_over_dt := phase_norm / dt_norm
		correction = float32(t_over_dt - t_over_dt*t_over_dt/2.0 - 0.5)
	} else if phase_norm > 1.0-dt_norm { // Falling edge correction
		t_minus_1_over_dt := (phase_norm - 1.0) / dt_norm
		correction = float32((t_minus_1_over_dt*t_minus_1_over_dt / 2.0) + t_minus_1_over_dt + 0.5)
	}

	return correction
}