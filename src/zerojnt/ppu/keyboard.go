package ppu

import (
	"log"
	"github.com/veandco/go-sdl2/sdl"
)

// CheckKeyboard drains the SDL event queue, prints specific details for
// handled events, and tells the caller whether the user asked to quit.
func (ppu *PPU) CheckKeyboard() (quit bool) {
	// Flush OS message queue – needed on some platforms when the render
	// loop is very tight.
	sdl.PumpEvents()

	// Drain up to 32 events per call (raise if you need more throughput).
	for processed := 0; processed < 32; processed++ {
		// Use a local variable for the polled event
		currentEvent := sdl.PollEvent() // Use a distinct local variable name
		if currentEvent == nil {
			break // queue empty
		}

		// Switch directly on the event type using type assertion
		switch e := currentEvent.(type) {
		// ── Window close ────────────────────────────────────────────────
		case sdl.QuitEvent:
			log.Printf("[EVT] QuitEvent at %d ms", e.Timestamp)
			quit = true

		// ── Keyboard ────────────────────────────────────────────────────
		case sdl.KeyboardEvent:
			



			// Compare Scancode and State directly
			if e.State == sdl.PRESSED || e.State == sdl.RELEASED {


				log.Printf("[EVT] Key %s (sc:%d sym:0x%x) %s",
				sdl.GetKeyName(e.Keysym.Sym),
				e.Keysym.Scancode,
				e.Keysym.Sym,
				keyStateString(e.State),
			)

				// TODO, create a method KeyDown and KeyUp, call this method passing a string that is the name of key. Call this method according by the  State
			}

		// ── Mouse buttons & motion (useful while debugging) ────────────
		case sdl.MouseButtonEvent:
			log.Printf("[EVT] Mouse button %d %s at (%d,%d)",
				e.Button, buttonStateString(e.State), e.X, e.Y)

		case *sdl.MouseMotionEvent:
			log.Printf("[EVT] Mouse motion at (%d,%d) – rel (%d,%d)",
				e.X, e.Y, e.XRel, e.YRel)

		// --- ADDED log for unhandled types ---
		default:
			//log.Printf("[EVT] Unhandled event type: %T", e)
		}
	}
	return
}

// Small helpers to keep the key/button‑state logs tidy
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