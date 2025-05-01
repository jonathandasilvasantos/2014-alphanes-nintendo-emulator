// File: ./ppu/ppu_fetch.go
package ppu

// fetchNTByte fetches the Nametable byte based on the current VRAM address
func (ppu *PPU) fetchNTByte() {
	addr := 0x2000 | (ppu.v & 0x0FFF) 
	ppu.nt_byte = ppu.ReadPPUMemory(addr)
	ppu.Cart.ClockIRQCounter() 
}

// fetchATByte fetches the Attribute Table byte
func (ppu *PPU) fetchATByte() {
	addr := 0x23C0 | (ppu.v & 0x0C00) | ((ppu.v >> 4) & 0x38) | ((ppu.v >> 2) & 0x07)
	ppu.at_byte = ppu.ReadPPUMemory(addr)
	ppu.Cart.ClockIRQCounter()
}

// fetchTileDataLow fetches the low byte of the background tile pattern
func (ppu *PPU) fetchTileDataLow() {
	fineY := (ppu.v >> 12) & 7
	patternTable := ppu.IO.PPUCTRL.BACKGROUND_ADDR
	tileIndex := uint16(ppu.nt_byte)
	addr := patternTable + tileIndex*16 + fineY
	ppu.tile_data_lo = ppu.ReadPPUMemory(addr)
	ppu.Cart.ClockIRQCounter()
}

// fetchTileDataHigh fetches the high byte of the background tile pattern
func (ppu *PPU) fetchTileDataHigh() {
	fineY := (ppu.v >> 12) & 7
	patternTable := ppu.IO.PPUCTRL.BACKGROUND_ADDR
	tileIndex := uint16(ppu.nt_byte)
	addr := patternTable + tileIndex*16 + fineY + 8
	ppu.tile_data_hi = ppu.ReadPPUMemory(addr)
	ppu.Cart.ClockIRQCounter()
}

// loadBackgroundShifters loads fetched tile data into background shift registers
func (ppu *PPU) loadBackgroundShifters() {
	lutKey := uint16(ppu.tile_data_hi)<<8 | uint16(ppu.tile_data_lo)
	var decodedIndices [8]byte
	copy(decodedIndices[:], planeDecode[lutKey][:])

	var patternLoBits, patternHiBits uint16
	for i := 0; i < 8; i++ {
		pixelIndexValue := decodedIndices[i]
		lowBitOfIndex := uint16(pixelIndexValue & 1)
		highBitOfIndex := uint16((pixelIndexValue >> 1) & 1)
		shiftAmount := 7 - i
		patternLoBits |= lowBitOfIndex << shiftAmount
		patternHiBits |= highBitOfIndex << shiftAmount
	}

	ppu.bg_pattern_shift_lo = (ppu.bg_pattern_shift_lo & 0xFF00) | patternLoBits
	ppu.bg_pattern_shift_hi = (ppu.bg_pattern_shift_hi & 0xFF00) | patternHiBits

	coarseXBit1 := (ppu.v >> 1) & 1
	coarseYBit1 := (ppu.v >> 6) & 1
	attrShift := (coarseYBit1 << 2) | (coarseXBit1 << 1)
	palette_bits := (ppu.at_byte >> attrShift) & 0x03

	attr_fill_lo := uint16(0x0000)
	if (palette_bits & 0x01) != 0 { attr_fill_lo = 0x00FF }
	attr_fill_hi := uint16(0x0000)
	if (palette_bits & 0x02) != 0 { attr_fill_hi = 0x00FF }

	ppu.bg_attr_shift_lo = (ppu.bg_attr_shift_lo & 0xFF00) | attr_fill_lo
	ppu.bg_attr_shift_hi = (ppu.bg_attr_shift_hi & 0xFF00) | attr_fill_hi
}

// evaluateSprites scans OAM to find sprites visible on the next scanline
func (ppu *PPU) evaluateSprites() {
	for i := range ppu.secondaryOAM { ppu.secondaryOAM[i] = 0xFF }
	ppu.spriteCount = 0
	ppu.IO.PPUSTATUS.SPRITE_OVERFLOW = false
	ppu.spriteZeroHitPossible = false

	spriteHeight := 8
	if ppu.IO.PPUCTRL.SPRITE_SIZE_16 { spriteHeight = 16 }

	scanlineToCheck := ppu.SCANLINE
	if scanlineToCheck >= 239 || scanlineToCheck < -1 {
	    ppu.spriteCount = 0
	    return
    }
    scanlineForEval := scanlineToCheck + 1

	numSpritesFound := 0
	primaryOAM := ppu.IO.OAM

	for n := 0; n < 64; n++ {
        oamIdx := n * 4
		spriteYTop := int(primaryOAM[oamIdx])

		if scanlineForEval >= spriteYTop+1 && scanlineForEval < (spriteYTop+1+spriteHeight) {
			if numSpritesFound < 8 {
				targetIdx := numSpritesFound * 4
				ppu.secondaryOAM[targetIdx+0] = primaryOAM[oamIdx+0]
				ppu.secondaryOAM[targetIdx+1] = primaryOAM[oamIdx+1]
				ppu.secondaryOAM[targetIdx+2] = primaryOAM[oamIdx+2]
				ppu.secondaryOAM[targetIdx+3] = primaryOAM[oamIdx+3]

				if n == 0 {
					ppu.spriteZeroHitPossible = true
				}
				numSpritesFound++
			} else {
				ppu.IO.PPUSTATUS.SPRITE_OVERFLOW = true
				break
			}
		}
	}
	ppu.spriteCount = numSpritesFound
}

// fetchSprites loads pattern data for sprites found in secondary OAM
func (ppu *PPU) fetchSprites() {
	for i := 0; i < 8; i++ {
		ppu.spriteCountersX[i] = 0xFF
		ppu.spriteLatches[i] = 0
		ppu.spritePatternsLo[i] = 0
		ppu.spritePatternsHi[i] = 0
		ppu.spriteIsSprite0[i] = false
	}

	if !ppu.IO.PPUMASK.SHOW_SPRITE || ppu.spriteCount == 0 { return }

	spriteHeight := 8
	if ppu.IO.PPUCTRL.SPRITE_SIZE_16 { spriteHeight = 16 }

	scanlineToRender := ppu.SCANLINE
	if scanlineToRender < 0 || scanlineToRender > 239 {
		return
	}

	for i := 0; i < ppu.spriteCount; i++ {
		secOamIdx := i * 4
		spriteYTop := int(ppu.secondaryOAM[secOamIdx+0])
		tileIndex := ppu.secondaryOAM[secOamIdx+1]
		attributes := ppu.secondaryOAM[secOamIdx+2]
		spriteX := ppu.secondaryOAM[secOamIdx+3]

		ppu.spriteCountersX[i] = spriteX
		ppu.spriteLatches[i] = attributes
		ppu.spriteIsSprite0[i] = ppu.spriteZeroHitPossible && (i == 0)

		flipHoriz := (attributes & 0x40) != 0
		flipVert := (attributes & 0x80) != 0

		rowInSprite := scanlineToRender - spriteYTop

		if flipVert {
			rowInSprite = (spriteHeight - 1) - rowInSprite
		}

		var tileAddr uint16
		var patternTable uint16

		if spriteHeight == 8 {
			patternTable = ppu.IO.PPUCTRL.SPRITE_8_ADDR
			tileAddr = patternTable + uint16(tileIndex)*16 + uint16(rowInSprite&7)
		} else {
			patternTable = uint16(tileIndex & 0x01) * 0x1000
			tileIndexBase := tileIndex & 0xFE

			if rowInSprite >= 8 {
				tileIndexBase++
				rowInSprite -= 8
			}
			tileAddr = patternTable + uint16(tileIndexBase)*16 + uint16(rowInSprite&7)
		}

		tileLo := ppu.ReadPPUMemory(tileAddr)
		tileHi := ppu.ReadPPUMemory(tileAddr + 8)

		if flipHoriz {
			tileLo = reverseByte(tileLo)
			tileHi = reverseByte(tileHi)
		}

		ppu.spritePatternsLo[i] = tileLo
		ppu.spritePatternsHi[i] = tileHi
	}
}

// reverseByte reverses the order of bits in a byte
func reverseByte(b byte) byte {
	b = (b&0xF0)>>4 | (b&0x0F)<<4
	b = (b&0xCC)>>2 | (b&0x33)<<2
	b = (b&0xAA)>>1 | (b&0x55)<<1
	return b
}