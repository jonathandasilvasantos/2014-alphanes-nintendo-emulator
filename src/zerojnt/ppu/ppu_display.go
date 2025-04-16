// File: ./ppu/ppu_display.go
// Contains SDL-specific logic: window/renderer initialization, framebuffer display, event polling, cleanup.

/*
Copyright 2014, 2014 Jonathan da Silva Santos
Modifications Copyright 2023-2024 (by AI based on request and refinement)

This file is part of Alphanes.

    Alphanes is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    Alphanes is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with Alphanes. If not, see <http://www.gnu.org/licenses/>.
*/
package ppu

import (
	"fmt"
	"log"
	"os"
	"unsafe" // <<<--- NEEDED for texture.Update with unsafe.Pointer

	"github.com/veandco/go-sdl2/sdl"
)

// initCanvas initializes SDL window, renderer, and texture for fullscreen display.
func (ppu *PPU) initCanvas() error {
	var winTitle string = "Alphanes (Fullscreen PPU Rewrite)"
	var err error

	if err = sdl.Init(sdl.INIT_VIDEO); err != nil {
		return fmt.Errorf("failed to initialize SDL Video: %w", err)
	}

	// Create window in Fullscreen Desktop mode
	// Width and height are ignored for FULLSCREEN_DESKTOP, it uses the monitor's current resolution.
	ppu.window, err = sdl.CreateWindow(winTitle, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		0, 0, // Ignored for fullscreen desktop
		sdl.WINDOW_SHOWN|sdl.WINDOW_FULLSCREEN_DESKTOP) // Use fullscreen desktop flag
	if err != nil {
		sdl.Quit()
		return fmt.Errorf("failed to create fullscreen window: %w", err)
	}
	log.Println("Fullscreen window created.")

	// Create renderer with VSync enabled
	ppu.renderer, err = sdl.CreateRenderer(ppu.window, -1, sdl.RENDERER_ACCELERATED|sdl.RENDERER_PRESENTVSYNC)
	if err != nil {
		ppu.window.Destroy()
		sdl.Quit()
		return fmt.Errorf("failed to create renderer: %w", err)
	}
	log.Println("Renderer created.")

	// Set logical size to maintain NES aspect ratio within the fullscreen window
	if err = ppu.renderer.SetLogicalSize(SCREEN_WIDTH, SCREEN_HEIGHT); err != nil {
		log.Printf("Warning: Failed to set logical size: %v. Scaling might be incorrect.", err)
		// Continue, but aspect ratio might be wrong
	} else {
		log.Printf("Logical size set to %dx%d.", SCREEN_WIDTH, SCREEN_HEIGHT)
	}

	// Use nearest neighbor scaling for pixel art look
	if !sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "0") {
		log.Printf("Warning: Failed to set render scale quality hint to nearest neighbor.")
	}

	// Create texture for PPU output (streaming for efficient updates from framebuffer)
	ppu.texture, err = ppu.renderer.CreateTexture(sdl.PIXELFORMAT_ARGB8888, sdl.TEXTUREACCESS_STREAMING, SCREEN_WIDTH, SCREEN_HEIGHT)
	if err != nil {
		ppu.renderer.Destroy()
		ppu.window.Destroy()
		sdl.Quit()
		return fmt.Errorf("failed to create texture: %w", err)
	}
	log.Println("Streaming texture created.")

	log.Println("SDL Canvas Initialized Successfully (Fullscreen)")
	return nil
}

// ShowScreen updates the SDL texture with the PPU's framebuffer data (SCREEN_DATA) and presents it.
// This should be called once per frame, typically at the start of VBlank.
func (ppu *PPU) ShowScreen() {
	if ppu.renderer == nil || ppu.texture == nil || ppu.window == nil {
		log.Println("Warning: ShowScreen called but SDL resources are nil.")
		return // Avoid panic if SDL resources were cleaned up
	}

	// Ensure the framebuffer has data before proceeding
	if len(ppu.SCREEN_DATA) != SCREEN_WIDTH*SCREEN_HEIGHT {
		log.Printf("Warning: ShowScreen called with incomplete or incorrectly sized framebuffer (%d elements).", len(ppu.SCREEN_DATA))
		return
	}

	// Calculate pitch (bytes per row) for the texture format
	pitch := int(SCREEN_WIDTH * 4) // ARGB8888 format = 4 bytes per pixel

	// Get a pointer to the framebuffer data (unsafe operation)
	// This provides direct memory access needed by UpdateTexture
	pixelsPtr := unsafe.Pointer(&ppu.SCREEN_DATA[0])

	// Update the SDL texture with the pixel data from the framebuffer
	err := ppu.texture.Update(nil, pixelsPtr, pitch)
	if err != nil {
		// Log the error, but don't necessarily stop the emulator.
		log.Printf("Warning: Failed to update SDL texture: %v", err)
		// Consider if continuing makes sense if texture updates fail.
	}

	// Clear the renderer (optional, but good practice before drawing)
	// Set background color for areas outside the logical size (if any)
	if err = ppu.renderer.SetDrawColor(0, 0, 0, 255); err != nil { // Black bars
		log.Printf("Warning: Failed to set draw color: %v", err)
	}
	if err = ppu.renderer.Clear(); err != nil {
		log.Printf("Warning: Failed to clear SDL renderer: %v", err)
	}

	// Copy the updated texture to the renderer.
	// SDL handles scaling from the texture's size (256x240) to the logical size,
	// and then scales the logical size to fit the fullscreen window.
	if err = ppu.renderer.Copy(ppu.texture, nil, nil); err != nil {
		log.Printf("Warning: Failed to copy SDL texture to renderer: %v", err)
		return // Critical if copy fails, likely nothing will display
	}

	// Present the renderer's contents to the window
	ppu.renderer.Present()
}

// checkKeyboard polls SDL events (basic quit handler).
func (ppu *PPU) CheckKeyboard() { // Renamed to avoid conflict if other input handling exists
	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
		switch event.(type) {
		case *sdl.QuitEvent:
			println("Quit event received")
			ppu.Cleanup()
			os.Exit(0)
		case *sdl.KeyboardEvent:
			// Example: Exit on Escape key press
			if event.(*sdl.KeyboardEvent).Keysym.Scancode == sdl.SCANCODE_ESCAPE && event.(*sdl.KeyboardEvent).State == sdl.PRESSED {
				println("Escape key pressed - Exiting")
				ppu.Cleanup()
				os.Exit(0)
			}
			// TODO: Add controller input mapping here if needed
		}
	}
}

// Cleanup releases SDL resources.
func (ppu *PPU) Cleanup() {
	log.Println("Cleaning up SDL resources...")
	if ppu.texture != nil {
		if err := ppu.texture.Destroy(); err != nil {
			log.Printf("Error destroying texture: %v", err)
		}
		ppu.texture = nil
	}
	if ppu.renderer != nil {
		ppu.renderer.Destroy() // Renderer destroy also handles textures associated? Check SDL docs. Safer to destroy texture explicitly.
		ppu.renderer = nil
	}
	if ppu.window != nil {
		if err := ppu.window.Destroy(); err != nil {
			log.Printf("Error destroying window: %v", err)
		}
		ppu.window = nil
	}
	sdl.Quit() // Quit SDL subsystem
	fmt.Println("SDL resources cleaned up.")
}
