package ppu

import (
	"log"
	"github.com/veandco/go-sdl2/sdl"
)

// Controller button bit mapping (NES standard)
const (
	ButtonA      byte = 0
	ButtonB      byte = 1
	ButtonSelect byte = 2
	ButtonStart  byte = 3
	ButtonUp     byte = 4
	ButtonDown   byte = 5
	ButtonLeft   byte = 6
	ButtonRight  byte = 7
)

// Global key state map
var keyStates = make(map[string]bool)

// mapKeyToPadBit maps key names to controller pad and button bits
func mapKeyToPadBit(key string) (pad int, bit byte, ok bool) {
	// Player 1 mappings
	switch key {
	case "Z":      return 0, ButtonA, true
	case "X":      return 0, ButtonB, true
	case "Space":  return 0, ButtonSelect, true
	case "Return": return 0, ButtonStart, true
	case "Up":     return 0, ButtonUp, true
	case "Down":   return 0, ButtonDown, true
	case "Left":   return 0, ButtonLeft, true
	case "Right":  return 0, ButtonRight, true
	}
	return 0, 0, false // Key not mapped
}

// CheckKeyboard drains the SDL event queue, updates controller state, and returns quit status.
func (ppu *PPU) CheckKeyboard() (quit bool) {
	// Flush OS message queue
	sdl.PumpEvents()

	// Drain up to 32 events per call
	for processed := 0; processed < 32; processed++ {
		// Use a local variable for the polled event
		currentEvent := sdl.PollEvent()
		if currentEvent == nil {
			break // queue empty
		}

		// Switch directly on the event type using type assertion
		switch e := currentEvent.(type) {
		// Window close
		case sdl.QuitEvent:
			log.Printf("[EVT] QuitEvent at %d ms", e.Timestamp)
			quit = true

		// Keyboard
		case sdl.KeyboardEvent:
			keyName := sdl.GetKeyName(e.Keysym.Sym)
			isPressed := (e.State == sdl.PRESSED)

			// Log only if state changed to avoid spam
			if keyStates[keyName] != isPressed {
				log.Printf("[EVT] Key %s (sc:%d sym:0x%x) %s",
					keyName,
					e.Keysym.Scancode,
					e.Keysym.Sym,
					keyStateString(e.State),
				)
			}

			// Update raw key state map
			keyStates[keyName] = isPressed

			// Update NES controller state by calling appropriate methods
			if isPressed {
				ppu.KeyDown(keyName)
			} else {
				ppu.KeyUp(keyName)
			}

		// Mouse buttons
		case sdl.MouseButtonEvent:
			log.Printf("[EVT] Mouse button %d %s at (%d,%d)",
				e.Button, buttonStateString(e.State), e.X, e.Y)
		}
	}
	return quit
}

// KeyDown updates the NES controller state when a mapped key is pressed.
func (ppu *PPU) KeyDown(keyName string) {
	pad, bit, ok := mapKeyToPadBit(keyName)
	if ok && pad < len(ppu.IO.Controllers) { // Check if pad index is valid
		// Set the corresponding bit in CurrentButtons
		ppu.IO.Controllers[pad].CurrentButtons |= (1 << bit)
	}
}

// KeyUp updates the NES controller state when a mapped key is released.
func (ppu *PPU) KeyUp(keyName string) {
	pad, bit, ok := mapKeyToPadBit(keyName)
	if ok && pad < len(ppu.IO.Controllers) { // Check if pad index is valid
		// Clear the corresponding bit in CurrentButtons
		ppu.IO.Controllers[pad].CurrentButtons &^= (1 << bit)
	}
}

// IsKeyPressed checks the raw SDL key state map.
func IsKeyPressed(keyName string) bool {
	state, exists := keyStates[keyName]
	return exists && state
}

// Helper functions for state logging
func keyStateString(state sdl.ButtonState) string {
	if state == sdl.PRESSED {
		return "DOWN"
	}
	return "UP"
}

func buttonStateString(state sdl.ButtonState) string {
	if state == sdl.PRESSED {
		return "DOWN"
	}
	return "UP"
}