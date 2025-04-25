package input

import (
	"github.com/veandco/go-sdl2/sdl"
	"zerojnt/ioports"
)

type InputHandler struct {
	io *ioports.IOPorts
}

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

var keyStates = make(map[string]bool)

func NewInputHandler(io *ioports.IOPorts) *InputHandler {
	if io == nil {
		return nil
	}

	handler := &InputHandler{
		io: io,
	}
	return handler
}

func (ih *InputHandler) HandleEvent(currentEvent sdl.Event) {
	switch e := currentEvent.(type) {
	case *sdl.QuitEvent:
		return

	case sdl.KeyboardEvent:
		keyName := sdl.GetKeyName(e.Keysym.Sym)
		isPressed := (e.State == sdl.PRESSED)

		if currentState, exists := keyStates[keyName]; !exists || currentState != isPressed {
			keyStates[keyName] = isPressed
			
			if isPressed {
				ih.KeyDown(keyName)
			} else {
				ih.KeyUp(keyName)
			}
		}
	}
}

func mapKeyToPadBit(key string) (pad int, bit byte, ok bool) {
	switch key {
	case "Z":
		return 0, ButtonA, true
	case "X":
		return 0, ButtonB, true
	case "Space":
		return 0, ButtonSelect, true
	case "Return":
		return 0, ButtonStart, true
	case "Up":
		return 0, ButtonUp, true
	case "Down":
		return 0, ButtonDown, true
	case "Left":
		return 0, ButtonLeft, true
	case "Right":
		return 0, ButtonRight, true
	}
	return 0, 0, false
}

func (ih *InputHandler) KeyDown(keyName string) {
	pad, bit, ok := mapKeyToPadBit(keyName)
	if ok && pad >= 0 && pad < len(ih.io.Controllers) {
		ih.io.Controllers[pad].CurrentButtons |= (1 << bit)
	}
}

func (ih *InputHandler) KeyUp(keyName string) {
	pad, bit, ok := mapKeyToPadBit(keyName)
	if ok && pad >= 0 && pad < len(ih.io.Controllers) {
		ih.io.Controllers[pad].CurrentButtons &^= (1 << bit)
	}
}

func IsKeyPressed(keyName string) bool {
	state, exists := keyStates[keyName]
	return exists && state
}

func keyStateString(state uint8) string {
	if state == uint8(sdl.PRESSED) {
		return "DOWN"
	}
	return "UP"
}

func buttonStateString(state uint8) string {
	if state == uint8(sdl.PRESSED) {
		return "DOWN"
	}
	return "UP"
}