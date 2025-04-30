package ppu

// Process executes one PPU cycle, updating state and potentially rendering a pixel.
func Process(ppu *PPU) {
	currentScanline := ppu.SCANLINE
	currentCycle := ppu.CYC

	if currentScanline == -1 { // Pre-render Scanline
		if currentCycle == 1 {
			// Clear VBlank, Sprite 0 Hit, Sprite Overflow flags
			ppu.IO.PPUSTATUS.VBLANK = false // End of VBlank
			ppu.IO.NMI = false // Drop NMI line level
			ppu.IO.PPUSTATUS.SPRITE_0_BIT = false
			ppu.IO.PPUSTATUS.SPRITE_OVERFLOW = false
		}

		ppu.handleBackgroundFetchingAndShifting()

		if ppu.isRenderingEnabled() {
			if currentCycle >= 280 && currentCycle <= 304 {
				ppu.transferAddressY()
			}
			if currentCycle == 257 {
				ppu.transferAddressX()
			}
		}

		if ppu.isRenderingEnabled() {
			if currentCycle == 256 {
				ppu.evaluateSprites()
			}
			if currentCycle == 257 {
				ppu.fetchSprites()
			}
		}

	} else if currentScanline >= 0 && currentScanline <= 239 { // Visible Scanlines
		if ppu.isRenderingEnabled() && currentCycle >= 1 && currentCycle <= 256 {
			ppu.renderPixel()
		}

		ppu.handleBackgroundFetchingAndShifting()

		if ppu.isRenderingEnabled() {
			if currentCycle == 256 {
				ppu.incrementScrollY()
			}
			if currentCycle == 257 {
				ppu.transferAddressX()
			}
		}

		if ppu.isRenderingEnabled() {
			if currentCycle == 256 {
				ppu.evaluateSprites()
			}
			if currentCycle == 257 {
				ppu.fetchSprites()
			}
		}

	} else if currentScanline == 240 { // Post-render Scanline
		// PPU is idle, frame data complete

	} else if currentScanline >= 241 && currentScanline <= 260 { // Vertical Blanking Scanlines
		if currentScanline == 241 && currentCycle == 1 {
			ppu.IO.PPUSTATUS.VBLANK = true // Set VBlank flag
			if ppu.IO.PPUCTRL.GEN_NMI {
				// Set the NMI line high. CPU will detect the rising edge if NMI_Latched was false.
				ppu.IO.NMI = true
			}
			ppu.ShowScreen()
		}
	}

	// Advance Timers
	ppu.CYC++
	if ppu.CYC >= CYCLES_PER_SCANLINE {
		ppu.CYC = 0
		ppu.SCANLINE++
		if ppu.SCANLINE > 260 {
			ppu.SCANLINE = -1
			ppu.frameOdd = !ppu.frameOdd

			// Odd Frame Cycle Skip
			if ppu.frameOdd && ppu.isRenderingEnabled() {
				ppu.CYC = 1
			}
		}
	}
}

// handleBackgroundFetchingAndShifting manages the 8-cycle fetch/shift pattern for background tiles.
func (ppu *PPU) handleBackgroundFetchingAndShifting() {
	if !ppu.isRenderingEnabled() {
		return
	}

	// Update Shifters
	if (ppu.CYC >= 2 && ppu.CYC <= 257) || (ppu.CYC >= 322 && ppu.CYC <= 337) {
		ppu.updateShifters()
	}

	// Background Memory Fetches
	isFetchRange := (ppu.CYC >= 1 && ppu.CYC <= 256) || (ppu.CYC >= 321 && ppu.CYC <= 336)

	if isFetchRange {
		fetchCycleMod8 := ppu.CYC % 8
		switch fetchCycleMod8 {
		case 1:
			ppu.loadBackgroundShifters()
			ppu.fetchNTByte()
		case 3:
			ppu.fetchATByte()
		case 5:
			ppu.fetchTileDataLow()
		case 7:
			ppu.fetchTileDataHigh()
		case 0:
			if ppu.CYC <= 256 || (ppu.CYC >= 328 && ppu.CYC <= 336) {
				ppu.incrementScrollX()
			}
		}
	}
}

// updateShifters shifts background and sprite registers each cycle during active rendering periods.
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

// renderPixel determines and outputs the final pixel color for the current cycle into SCREEN_DATA.
//go:nosplit
func (ppu *PPU) renderPixel() {
	fb := ppu.SCREEN_DATA      // alias for faster bounds analysis
	pixelX := ppu.CYC - 1
	pixelY := ppu.SCANLINE

	if pixelX|pixelY < 0 || pixelX >= SCREEN_WIDTH || pixelY >= SCREEN_HEIGHT {
		return    // coordinates out of range – keep fast exit, no logging
	}

	rowStart := pixelY * SCREEN_WIDTH
	if rowStart+SCREEN_WIDTH > len(fb) {
		return    // impossible unless fb was modified elsewhere
	}

	// Calculate Background Pixel & Palette Information
	bgPixel := byte(0)
	bgPalette := byte(0)
	bgIsOpaque := false

	if ppu.IO.PPUMASK.SHOW_BACKGROUND {
		if !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_BACKGROUND) {
			mux := uint16(0x8000) >> ppu.x

			p0_bg := boolToByte((ppu.bg_pattern_shift_lo & mux) > 0)
			p1_bg := boolToByte((ppu.bg_pattern_shift_hi & mux) > 0)
			bgPixel = (p1_bg << 1) | p0_bg

			if bgPixel != 0 {
				bgIsOpaque = true
				pal0_bg := boolToByte((ppu.bg_attr_shift_lo & mux) > 0)
				pal1_bg := boolToByte((ppu.bg_attr_shift_hi & mux) > 0)
				bgPalette = (pal1_bg << 1) | pal0_bg
			}
		}
	}

	// Calculate Sprite Pixel & Palette Information
	sprPixel := byte(0)
	sprPalette := byte(0)
	sprPriority := byte(1)
	sprIsOpaque := false
	spriteZeroPixelRendered := false

	if ppu.IO.PPUMASK.SHOW_SPRITE {
		if !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_SPRITE) {
			for i := 0; i < ppu.spriteCount; i++ {
				if ppu.spriteCountersX[i] == 0 {
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

	// Combine Background & Sprite based on opacity and priority
	finalPixel := byte(0)
	finalPalette := byte(0)

	if !bgIsOpaque && !sprIsOpaque {
		finalPixel = 0
		finalPalette = 0
	} else if !bgIsOpaque && sprIsOpaque {
		finalPixel = sprPixel
		finalPalette = sprPalette + 4
	} else if bgIsOpaque && !sprIsOpaque {
		finalPixel = bgPixel
		finalPalette = bgPalette
	} else {
		// Sprite 0 Hit Detection
		if spriteZeroPixelRendered && bgIsOpaque && pixelX >= 0 && pixelX < 255 {
			showBG := !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_BACKGROUND)
			showSP := !(pixelX < 8 && !ppu.IO.PPUMASK.SHOW_LEFTMOST_8_SPRITE)
			if showBG && showSP && !ppu.IO.PPUSTATUS.SPRITE_0_BIT {
				ppu.IO.PPUSTATUS.SPRITE_0_BIT = true
			}
		}

		// Priority Multiplexer
		if sprPriority == 0 {
			finalPixel = sprPixel
			finalPalette = sprPalette + 4
		} else {
			finalPixel = bgPixel
			finalPalette = bgPalette
		}
	}

	// Final Color Lookup and Framebuffer Write
	var paletteAddr uint16
	if finalPixel == 0 {
		paletteAddr = PALETTE_RAM
	} else {
		paletteAddr = PALETTE_RAM | (uint16(finalPalette) << 2) | uint16(finalPixel)
	}

	// Read the color index from palette RAM
	colorEntryIndex := ppu.ReadPPUMemory(paletteAddr)

	// Apply Grayscale if enabled
	if ppu.IO.PPUMASK.GREYSCALE {
		colorEntryIndex &= 0x30
	}

	// Look up the final color from the color table
	finalColor := ppu.colors[colorEntryIndex&0x3F]

	// Write to Screen Buffer - optimized with no bounds check
	fb[rowStart+pixelX] = finalColor
}

// incrementScrollX handles the horizontal VRAM address increment
func (ppu *PPU) incrementScrollX() {
	if (ppu.v & 0x001F) == 31 {
		ppu.v &= ^uint16(0x001F)
		ppu.v ^= 0x0400
	} else {
		ppu.v++
	}
}

// incrementScrollY handles the vertical VRAM address increment
func (ppu *PPU) incrementScrollY() {
	if (ppu.v & 0x7000) != 0x7000 {
		ppu.v += 0x1000
	} else {
		ppu.v &= ^uint16(0x7000)
		y := (ppu.v & 0x03E0) >> 5
		if y == 29 {
			y = 0
			ppu.v ^= 0x0800
		} else if y == 31 {
			y = 0
		} else {
			y++
		}
		ppu.v = (ppu.v & ^uint16(0x03E0)) | (y << 5)
	}
}

// transferAddressX copies horizontal bits from t to v
func (ppu *PPU) transferAddressX() {
	ppu.v = (ppu.v & 0xFBE0) | (ppu.t & 0x041F)
}

// transferAddressY copies vertical bits from t to v
func (ppu *PPU) transferAddressY() {
	ppu.v = (ppu.v & 0x841F) | (ppu.t & 0x7BE0)
}