package ppu

// fetchNTByte fetches the Nametable byte based on the current VRAM address 'v'.
func (ppu *PPU) fetchNTByte() {
	addr := 0x2000 | (ppu.v & 0x0FFF) // Nametable base + NT address bits from v
	ppu.nt_byte = ppu.ReadPPUMemory(addr)

	// Update A12 state for potential IRQ clocking (if needed here)
	// ppu.Cart.ClockIRQCounter((addr & 0x1000) != 0) // Example - Usually done after tile fetches
}

// fetchATByte fetches the Attribute Table byte based on 'v'.
func (ppu *PPU) fetchATByte() {
	// Address: 0x23C0 | Nametable bits | CoarseY bits / 4 | CoarseX bits / 4
	addr := 0x23C0 | (ppu.v & 0x0C00) | ((ppu.v >> 4) & 0x38) | ((ppu.v >> 2) & 0x07)
	ppu.at_byte = ppu.ReadPPUMemory(addr)

	// Update A12 state for potential IRQ clocking (if needed here)
	// ppu.Cart.ClockIRQCounter((addr & 0x1000) != 0) // Example
}

// fetchTileDataLow fetches the low byte of the background tile pattern.
func (ppu *PPU) fetchTileDataLow() {
	fineY := (ppu.v >> 12) & 7
	patternTable := ppu.IO.PPUCTRL.BACKGROUND_ADDR // $0000 or $1000
	tileIndex := uint16(ppu.nt_byte)
	addr := patternTable + tileIndex*16 + fineY
	ppu.tile_data_lo = ppu.ReadPPUMemory(addr)

	// Clock the mapper IRQ counter with the A12 state *during* this fetch
	// currentA12State := (addr & 0x1000) != 0 // Argument removed to match function signature
	ppu.Cart.ClockIRQCounter()
}

// fetchTileDataHigh fetches the high byte of the background tile pattern.
func (ppu *PPU) fetchTileDataHigh() {
	fineY := (ppu.v >> 12) & 7
	patternTable := ppu.IO.PPUCTRL.BACKGROUND_ADDR
	tileIndex := uint16(ppu.nt_byte)
	addr := patternTable + tileIndex*16 + fineY + 8 // High plane is +8 bytes offset
	ppu.tile_data_hi = ppu.ReadPPUMemory(addr)

	// Clock the mapper IRQ counter with the A12 state *during* this fetch
	// currentA12State := (addr & 0x1000) != 0 // Argument removed to match function signature
	ppu.Cart.ClockIRQCounter()
}

// loadBackgroundShifters loads fetched tile data into background shift registers.
func (ppu *PPU) loadBackgroundShifters() {
	// Load pattern data
	ppu.bg_pattern_shift_lo = (ppu.bg_pattern_shift_lo & 0xFF00) | uint16(ppu.tile_data_lo)
	ppu.bg_pattern_shift_hi = (ppu.bg_pattern_shift_hi & 0xFF00) | uint16(ppu.tile_data_hi)

	// Determine attribute bits for the quadrant defined by v
	shift := ((ppu.v >> 4) & 4) | (ppu.v & 2)     // Selects the correct 2 bits from AT byte
	palette_bits := (ppu.at_byte >> shift) & 0x03 // Get 2 palette index bits

	// Expand bits to fill the lower byte of attribute shifters
	attr_fill_lo := uint16(0x0000)
	if (palette_bits & 0x01) != 0 { attr_fill_lo = 0x00FF }
	attr_fill_hi := uint16(0x0000)
	if (palette_bits & 0x02) != 0 { attr_fill_hi = 0x00FF }

	// Load attribute data
	ppu.bg_attr_shift_lo = (ppu.bg_attr_shift_lo & 0xFF00) | attr_fill_lo
	ppu.bg_attr_shift_hi = (ppu.bg_attr_shift_hi & 0xFF00) | attr_fill_hi
}

// evaluateSprites scans OAM to find sprites visible on the next scanline.
func (ppu *PPU) evaluateSprites() {
	// Clear secondary OAM
	for i := range ppu.secondaryOAM { ppu.secondaryOAM[i] = 0xFF }
	ppu.spriteCount = 0
	ppu.IO.PPUSTATUS.SPRITE_OVERFLOW = false
	ppu.spriteZeroHitPossible = false

	spriteHeight := 8
	if ppu.IO.PPUCTRL.SPRITE_SIZE_16 { spriteHeight = 16 }

	scanlineToCheck := ppu.SCANLINE
	if scanlineToCheck < 0 { scanlineToCheck = 0 }

	numSpritesFound := 0
	primaryOAM := ppu.IO.OAM

	// Iterate through all 64 primary OAM entries
	for n := 0; n < 64; n++ {
        oamIdx := n * 4
		spriteY := int(primaryOAM[oamIdx]) + 1

		// Check if sprite is in range for next scanline
		if scanlineToCheck >= spriteY && scanlineToCheck < (spriteY+spriteHeight) {
			if numSpritesFound < 8 { // Copy to secondary OAM
				targetIdx := numSpritesFound * 4
				ppu.secondaryOAM[targetIdx+0] = primaryOAM[oamIdx+0] // Y
				ppu.secondaryOAM[targetIdx+1] = primaryOAM[oamIdx+1] // Tile Index
				ppu.secondaryOAM[targetIdx+2] = primaryOAM[oamIdx+2] // Attributes
				ppu.secondaryOAM[targetIdx+3] = primaryOAM[oamIdx+3] // X

				// Check if this is sprite 0
				if n == 0 {
					ppu.spriteZeroHitPossible = true
				}
				numSpritesFound++
			} else {
				// Handle overflow
				ppu.IO.PPUSTATUS.SPRITE_OVERFLOW = true
				break
			}
		}
	}
	ppu.spriteCount = numSpritesFound
}

// fetchSprites loads pattern data for sprites in secondary OAM.
func (ppu *PPU) fetchSprites() {
	// Clear sprite pipeline registers
	for i := 0; i < 8; i++ {
		ppu.spriteCountersX[i] = 0xFF // Mark inactive
		ppu.spriteLatches[i] = 0
		ppu.spritePatternsLo[i] = 0
		ppu.spritePatternsHi[i] = 0
		ppu.spriteIsSprite0[i] = false
	}

	if !ppu.IO.PPUMASK.SHOW_SPRITE { return }

	spriteHeight := 8
	if ppu.IO.PPUCTRL.SPRITE_SIZE_16 { spriteHeight = 16 }

	// Fetch data for sprites in secondary OAM
	for i := 0; i < ppu.spriteCount; i++ {
		secOamIdx := i * 4
		spriteY := uint16(ppu.secondaryOAM[secOamIdx+0]) + 1
		tileIndex := ppu.secondaryOAM[secOamIdx+1]
		attributes := ppu.secondaryOAM[secOamIdx+2]
		spriteX := ppu.secondaryOAM[secOamIdx+3]

		// Load rendering state
		ppu.spriteCountersX[i] = spriteX
		ppu.spriteLatches[i] = attributes
		ppu.spriteIsSprite0[i] = ppu.spriteZeroHitPossible && (i == 0)

		// Determine pattern row based on flip and scanline
		flipHoriz := (attributes & 0x40) != 0
		flipVert := (attributes & 0x80) != 0
		scanlineToRender := uint16(ppu.SCANLINE)
		if scanlineToRender < 0 { scanlineToRender = 0 }

		row := scanlineToRender - spriteY

		if flipVert {
			row = uint16(spriteHeight-1) - row
		}

		// Determine pattern table and tile address
		var tileAddr uint16
		var patternTable uint16

		if spriteHeight == 8 { // 8x8 Sprites
			patternTable = ppu.IO.PPUCTRL.SPRITE_8_ADDR
			row &= 7
			tileAddr = patternTable + uint16(tileIndex)*16 + row
		} else { // 8x16 Sprites
			patternTable = uint16(tileIndex & 0x01) * 0x1000
			tileIndexBase := tileIndex & 0xFE

			if row >= 8 { // Bottom half of sprite
				tileIndexBase++ 
				row -= 8
			}
			row &= 7
			tileAddr = patternTable + uint16(tileIndexBase)*16 + row
		}

		// Fetch pattern bytes
		tileLo := ppu.ReadPPUMemory(tileAddr)
		tileHi := ppu.ReadPPUMemory(tileAddr + 8)
		
		// Clock the mapper IRQ counter with A12 state during sprite fetch
		// currentA12State := (tileAddr & 0x1000) != 0 // Argument removed to match function signature
		ppu.Cart.ClockIRQCounter() // Clock for lo byte fetch

		// Apply horizontal flip if needed
		if flipHoriz {
			tileLo = reverseByte(tileLo)
			tileHi = reverseByte(tileHi)
		}

		// Load into shift registers
		ppu.spritePatternsLo[i] = tileLo
		ppu.spritePatternsHi[i] = tileHi
	}
}