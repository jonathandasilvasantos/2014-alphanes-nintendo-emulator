// File: ppu/plane_decode.go
package ppu

import "log"

// planeDecode is the lookup table: [65536][8]byte
// Index: uint16(patternHigh << 8 | patternLow)
// Value: [8]byte containing the 2-bit color index (0-3) for each pixel (left-to-right)
var planeDecode [1 << 16][8]byte

// initPlaneDecode precomputes the pattern table plane decoding LUT.
func initPlaneDecode() {
	log.Println("Precomputing PPU pattern plane decode LUT...")
	for val := 0; val < (1 << 16); val++ {
		patternLow := byte(val & 0xFF)
		patternHigh := byte(val >> 8)

		for bit := 0; bit < 8; bit++ {
			// Calculate pixel index (0=leftmost, 7=rightmost)
			pixelIndex := 7 - bit

			// Extract bits for this pixel position
			lowBit := (patternLow >> bit) & 1
			highBit := (patternHigh >> bit) & 1

			// Combine bits to form the 2-bit color index
			colorIndex := (highBit << 1) | lowBit

			// Store in the LUT at the correct pixel position
			planeDecode[val][pixelIndex] = colorIndex
		}
	}
	log.Println("PPU pattern plane decode LUT computed.")
}

// init explicitly calls the LUT generation function when the package is loaded.
func init() {
	initPlaneDecode()
}