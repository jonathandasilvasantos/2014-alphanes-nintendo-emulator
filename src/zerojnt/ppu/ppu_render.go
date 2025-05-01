// File: ./ppu/ppu_render.go
package ppu

// Process executes one PPU cycle, updating state and potentially rendering a pixel.
func Process(ppu *PPU) {
	currentScanline := ppu.SCANLINE
	currentCycle := ppu.CYC

	// Pre-render Scanline (-1)
	if currentScanline == -1 {
		if currentCycle == 1 {
			// Clear flags at cycle 1
			ppu.IO.PPUSTATUS.VBLANK = false
			ppu.IO.NMI = false
			ppu.IO.PPUSTATUS.SPRITE_0_BIT = false
			ppu.IO.PPUSTATUS.SPRITE_OVERFLOW = false
			ppu.spriteZeroBeingRendered = false
		}

		// Background data fetching and shifting 
		ppu.handleBackgroundFetchingAndShifting()

		// Address updates if rendering is enabled
		if ppu.isRenderingEnabled() {
			// Vertical address transfer near the end of the scanline
			if currentCycle >= 280 && currentCycle <= 304 {
				ppu.transferAddressY()
			}
			// Horizontal address transfer at cycle 257
			if currentCycle == 257 {
				ppu.transferAddressX()
			}
		}

		// Sprite OAMADDR reset
		if currentCycle >= 257 && currentCycle <= 320 {
			ppu.IO.OAMADDR = 0
		}

	} else if currentScanline >= 0 && currentScanline <= 239 { // Visible Scanlines (0-239)
		// Pixel Rendering during cycles 1-256
		if ppu.isRenderingEnabled() && currentCycle >= 1 && currentCycle <= 256 {
			ppu.renderPixel()
		}

		// Background Data Fetching & Shifting
		ppu.handleBackgroundFetchingAndShifting()

		// Address Updates if rendering is enabled
		if ppu.isRenderingEnabled() {
			// Increment vertical VRAM address at cycle 256
			if currentCycle == 256 {
				ppu.incrementScrollY()
			}
			// Copy horizontal VRAM address components at cycle 257
			if currentCycle == 257 {
				ppu.transferAddressX()
			}
		}

		// Sprite Processing for NEXT Scanline
		if currentCycle == 257 {
			ppu.IO.OAMADDR = 0
		}
		// Sprite Evaluation for cycles 257-320
		if ppu.isRenderingEnabled() && currentCycle >= 257 && currentCycle <= 320 {
			if currentCycle == 257 {
				ppu.evaluateSprites()
			}
		}
		// Sprite Fetching for cycles 321-340
		if ppu.isRenderingEnabled() && currentCycle >= 321 && ppu.CYC <= 340 {
			if currentCycle == 321 {
				ppu.fetchSprites()
			}
		}

	} else if currentScanline == 240 { // Post-render Scanline (240)
		// PPU is idle

	} else if currentScanline >= 241 && currentScanline <= 260 { // Vertical Blanking Scanlines (241-260)
		// VBlank period starts at scanline 241, cycle 1
		if currentScanline == 241 && currentCycle == 1 {
			ppu.IO.PPUSTATUS.VBLANK = true
			// Check if NMI generation is enabled
			if ppu.IO.PPUCTRL.GEN_NMI {
				ppu.IO.NMI = true
			}
			// Update the display if not skipping render
			if !ppu.skipRenderThisFrame {
				ppu.ShowScreen()
			}
			// Reset the flag after the decision point
			ppu.skipRenderThisFrame = false
		}
	}

	// Advance Timers
	ppu.CYC++
	if ppu.CYC >= CYCLES_PER_SCANLINE { // End of a scanline
		ppu.CYC = 0 
		ppu.SCANLINE++
		if ppu.SCANLINE > 260 { // End of a frame
			ppu.SCANLINE = -1
			ppu.frameOdd = !ppu.frameOdd

			// Odd Frame Cycle Skip when rendering is enabled
			if ppu.frameOdd && ppu.isRenderingEnabled() {
				ppu.CYC = 1
			}
		}
	}
}

// handleBackgroundFetchingAndShifting manages the 8-cycle fetch/shift pattern for background tiles.
func (ppu *PPU) handleBackgroundFetchingAndShifting() {
	// Skip if rendering is disabled
	if !ppu.isRenderingEnabled() {
		return
	}

	// Update Background Shifters
	if (ppu.CYC >= 2 && ppu.CYC <= 257) || (ppu.CYC >= 322 && ppu.CYC <= 337) {
		ppu.updateShifters()
	}

	// Background Memory Fetches
	isFetchCycle := (ppu.CYC >= 1 && ppu.CYC <= 256) || (ppu.CYC >= 321 && ppu.CYC <= 336)

	if isFetchCycle {
		// Determine fetch step within 8-cycle pattern
		fetchStep := ppu.CYC % 8

		switch fetchStep {
		case 1: 
			// Load previously fetched data into shift registers
			ppu.loadBackgroundShifters()
			// Fetch Nametable byte for next tile
			ppu.fetchNTByte()
		case 3:
			// Fetch Attribute Table byte for next tile
			ppu.fetchATByte()
		case 5:
			// Fetch low Pattern Table byte for next tile
			ppu.fetchTileDataLow()
		case 7:
			// Fetch high Pattern Table byte for next tile
			ppu.fetchTileDataHigh()
		case 0:
			// Increment horizontal VRAM address
			ppu.incrementScrollX()
		}
	}
}

// updateShifters shifts background pattern and attribute registers left by one bit.
func (ppu *PPU) updateShifters() {
	// Shift Background Registers
	if ppu.IO.PPUMASK.SHOW_BACKGROUND {
		ppu.bg_pattern_shift_lo <<= 1
		ppu.bg_pattern_shift_hi <<= 1
		ppu.bg_attr_shift_lo <<= 1
		ppu.bg_attr_shift_hi <<= 1
	}

	// Shift Sprite Registers
	if ppu.IO.PPUMASK.SHOW_SPRITE {
		for i := 0; i < ppu.spriteCount; i++ {
			if ppu.spriteCountersX[i] > 0 {
				ppu.spriteCountersX[i]--
			} else {
				ppu.spritePatternsLo[i] <<= 1
				ppu.spritePatternsHi[i] <<= 1
			}
		}
	}
}

// renderPixel determines and outputs the final pixel color for the current cycle.
//go:nosplit
func (ppu *PPU) renderPixel() {
	fb := ppu.SCREEN_DATA
	pixelX := ppu.CYC - 1
	pixelY := ppu.SCANLINE

	// Bounds check
	if uint(pixelX) >= SCREEN_WIDTH || uint(pixelY) >= SCREEN_HEIGHT {
		return
	}

	// Calculate framebuffer index
	fbIndex := pixelY*SCREEN_WIDTH + pixelX
	if fbIndex >= len(fb) {
		return
	}

	// Calculate Background Pixel & Palette
	bgPixel := byte(0)
	bgPalette := byte(0)
	bgIsOpaque := false

	if ppu.IO.PPUMASK.SHOW_BACKGROUND {
		if !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_BACKGROUND) {
			// Determine bit position based on fine X scroll
			fineXSelectBit := uint16(1) << (15 - ppu.x)

			// Extract pattern bits
			p0_bg := byte((ppu.bg_pattern_shift_lo & fineXSelectBit) >> (15 - ppu.x))
			p1_bg := byte((ppu.bg_pattern_shift_hi & fineXSelectBit) >> (15 - ppu.x))
			bgPixel = (p1_bg << 1) | p0_bg

			if bgPixel != 0 {
				bgIsOpaque = true
				// Get attribute bits
				attr0_bg := byte((ppu.bg_attr_shift_lo & fineXSelectBit) >> (15 - ppu.x))
				attr1_bg := byte((ppu.bg_attr_shift_hi & fineXSelectBit) >> (15 - ppu.x))
				bgPalette = (attr1_bg << 1) | attr0_bg
			}
		}
	}

	// Calculate Sprite Pixel & Palette
	sprPixel := byte(0)
	sprPalette := byte(0)
	sprPriority := byte(1) // Default to behind BG
	sprIsOpaque := false
	spriteZeroPixelRendered := false

	if ppu.IO.PPUMASK.SHOW_SPRITE {
		if !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_SPRITE) {
			for i := 0; i < ppu.spriteCount; i++ {
				if ppu.spriteCountersX[i] == 0 {
					// Get sprite pixel data
					p0_spr := (ppu.spritePatternsLo[i] >> 7) & 1
					p1_spr := (ppu.spritePatternsHi[i] >> 7) & 1
					currentSprPixelData := (p1_spr << 1) | p0_spr

					if currentSprPixelData != 0 && !sprIsOpaque {
						sprPixel = currentSprPixelData
						sprPalette = (ppu.spriteLatches[i] & 0x03)
						sprPriority = (ppu.spriteLatches[i] & 0x20) >> 5
						sprIsOpaque = true

						if ppu.spriteIsSprite0[i] {
							spriteZeroPixelRendered = true
						}

						break
					}
				}
			}
		}
	}

	// Combine Background & Sprite
	finalPixel := byte(0)
	finalPalette := byte(0)

	// Decision logic for final pixel
	if !bgIsOpaque && !sprIsOpaque {
		// Both transparent: universal background color
		finalPixel = 0
		finalPalette = 0
	} else if !bgIsOpaque && sprIsOpaque {
		// Background transparent, Sprite opaque: use sprite
		finalPixel = sprPixel
		finalPalette = sprPalette + 4
	} else if bgIsOpaque && !sprIsOpaque {
		// Background opaque, Sprite transparent: use background
		finalPixel = bgPixel
		finalPalette = bgPalette
	} else {
		// Both opaque: check sprite 0 hit and apply priority
		if spriteZeroPixelRendered && bgIsOpaque && pixelX < 255 {
			showBG := !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_BACKGROUND)
			showSP := !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_SPRITE)
			if showBG && showSP {
				if !ppu.IO.PPUSTATUS.SPRITE_0_BIT {
					ppu.IO.PPUSTATUS.SPRITE_0_BIT = true
				}
			}
		}

		// Apply priority
		if sprPriority == 0 {
			finalPixel = sprPixel
			finalPalette = sprPalette + 4
		} else {
			finalPixel = bgPixel
			finalPalette = bgPalette
		}
	}

	// Final Color Lookup
	var paletteAddr uint16
	if finalPixel == 0 {
		paletteAddr = PALETTE_RAM // 0x3F00
	} else {
		paletteAddr = PALETTE_RAM | (uint16(finalPalette) << 2) | uint16(finalPixel)
	}

	// Get color index from palette RAM
	colorEntryIndex := ppu.ReadPPUMemory(paletteAddr)

	// Apply Grayscale if enabled
	if ppu.IO.PPUMASK.GREYSCALE {
		colorEntryIndex &= 0x30
	}

	// Look up final color
	finalColor := ppu.colors[colorEntryIndex&0x3F]

	// Write to screen buffer
	fb[fbIndex] = finalColor
}

// incrementScrollX handles the horizontal VRAM address increment.
func (ppu *PPU) incrementScrollX() {
	if !ppu.isRenderingEnabled() { return }

	// Check if coarse X is at maximum
	if (ppu.v & 0x001F) == 31 {
		ppu.v &= ^uint16(0x001F) // Reset coarse X
		ppu.v ^= 0x0400          // Switch horizontal nametable
	} else {
		ppu.v++ // Increment coarse X
	}
}

// incrementScrollY handles the vertical VRAM address increment.
func (ppu *PPU) incrementScrollY() {
	if !ppu.isRenderingEnabled() { return }

	// Increment fine Y scroll
	if (ppu.v & 0x7000) != 0x7000 {
		ppu.v += 0x1000
	} else {
		// Reset fine Y and handle coarse Y
		ppu.v &= ^uint16(0x7000)

		// Update coarse Y
		y := (ppu.v & 0x03E0) >> 5
		if y == 29 {
			y = 0
			ppu.v ^= 0x0800        // Switch vertical nametable
		} else if y == 31 {
			y = 0
		} else {
			y++
		}
		
		ppu.v = (ppu.v & ^uint16(0x03E0)) | (y << 5)
	}
}

// transferAddressX copies horizontal bits from temporary to current VRAM address.
func (ppu *PPU) transferAddressX() {
	if !ppu.isRenderingEnabled() { return }
	// Copy Coarse X and Nametable select H bit
	ppu.v = (ppu.v &^ 0x041F) | (ppu.t & 0x041F)
}

// transferAddressY copies vertical bits from temporary to current VRAM address.
func (ppu *PPU) transferAddressY() {
	if !ppu.isRenderingEnabled() { return }
	// Copy Fine Y, Nametable select V bit, and Coarse Y
	ppu.v = (ppu.v &^ 0x7BE0) | (ppu.t & 0x7BE0)
}