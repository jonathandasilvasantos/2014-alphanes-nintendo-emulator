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
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	// Target 30fps (33.33ms per frame)
	targetFrameTime = time.Second / 30
)

var (
	// Reusable event for polling to reduce GC pressure
	event sdl.Event
	// Mutex for framebuffer access
	fbMutex sync.RWMutex
	// Track frame timing
	lastFrameTime time.Time
)

// initCanvas initializes SDL window, renderer, and texture for fullscreen display.
func (ppu *PPU) initCanvas() error {
	// Set GOMAXPROCS to utilize all available cores
	runtime.GOMAXPROCS(runtime.NumCPU())
	
	winTitle := "Alphanes (Optimized 30FPS PPU)"

	// Initialize SDL with only needed subsystems
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		return fmt.Errorf("failed to initialize SDL Video: %w", err)
	}

	// Create window with flags for best performance
	var err error
	ppu.window, err = sdl.CreateWindow(winTitle, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		0, 0, // Ignored for fullscreen desktop
		sdl.WINDOW_SHOWN|sdl.WINDOW_FULLSCREEN_DESKTOP)
	if err != nil {
		sdl.Quit()
		return fmt.Errorf("failed to create fullscreen window: %w", err)
	}

	// Create renderer with hardware acceleration and DISABLE VSync (we'll control timing ourselves)
	ppu.renderer, err = sdl.CreateRenderer(ppu.window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		ppu.window.Destroy()
		sdl.Quit()
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	// Set logical size to maintain aspect ratio
	if err = ppu.renderer.SetLogicalSize(SCREEN_WIDTH, SCREEN_HEIGHT); err != nil {
		log.Printf("Warning: Failed to set logical size: %v. Scaling might be incorrect.", err)
	}

	// Use nearest neighbor scaling for pixel art and better performance
	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "0")
	
	// Additional performance hints
	sdl.SetHint(sdl.HINT_RENDER_DRIVER, "opengl") // Use OpenGL for hardware acceleration
	sdl.SetHint(sdl.HINT_RENDER_BATCHING, "1")    // Enable batching for better performance
	sdl.SetHint(sdl.HINT_VIDEO_X11_NET_WM_BYPASS_COMPOSITOR, "1") // Bypass compositor for better performance
	sdl.SetHint(sdl.HINT_RENDER_VSYNC, "0")       // Disable VSync as we're manually timing
	
	// Create streaming texture - optimal for frequent updates
	ppu.texture, err = ppu.renderer.CreateTexture(
		sdl.PIXELFORMAT_ARGB8888,
		sdl.TEXTUREACCESS_STREAMING,
		SCREEN_WIDTH, SCREEN_HEIGHT,
	)
	if err != nil {
		ppu.renderer.Destroy()
		ppu.window.Destroy()
		sdl.Quit()
		return fmt.Errorf("failed to create texture: %w", err)
	}

	// Set once and reuse
	if err = ppu.renderer.SetDrawColor(0, 0, 0, 255); err != nil {
		log.Printf("Warning: Failed to set draw color: %v", err)
	}
	
	// Initialize frame timing
	lastFrameTime = time.Now()
	
	log.Println("SDL Canvas Initialized Successfully (Optimized 30FPS)")
	return nil
}

// ShowScreen updates the SDL texture with the PPU's framebuffer data and presents it.
// Now includes frame rate limiting to 30fps
func (ppu *PPU) ShowScreen() {
	if ppu.renderer == nil || ppu.texture == nil {
		return // Early return to avoid nil checks later
	}

	// Calculate time since last frame
	now := time.Now()
	elapsed := now.Sub(lastFrameTime)
	
	// Skip frame if not enough time has passed (maintain 30fps)
	if elapsed < targetFrameTime {
		sleepTime := targetFrameTime - elapsed
		time.Sleep(sleepTime)
		return
	}
	
	// Update last frame time
	lastFrameTime = now

	// Get read lock on framebuffer
	fbMutex.RLock()
	
	if len(ppu.SCREEN_DATA) != SCREEN_WIDTH*SCREEN_HEIGHT {
		fbMutex.RUnlock()
		return // Invalid framebuffer size
	}

	// Calculate pitch once (4 bytes per ARGB8888 pixel)
	const pitch = SCREEN_WIDTH * 4
	
	// Direct pointer for maximum update speed
	pixelsPtr := unsafe.Pointer(&ppu.SCREEN_DATA[0])

	// Update texture in a single call with pointer arithmetic
	err := ppu.texture.Update(nil, pixelsPtr, pitch)
	
	// We can release the lock immediately after copying data
	fbMutex.RUnlock()
	
	if err != nil {
		log.Printf("Texture update failed: %v", err)
		return
	}

	// Clear with preset color (faster than setting color each time)
	ppu.renderer.Clear()
	
	// Copy texture to renderer (SDL handles scaling)
	ppu.renderer.Copy(ppu.texture, nil, nil)
	
	// Present frame
	ppu.renderer.Present()
}

// CheckKeyboard polls SDL events efficiently with event reuse.
// Now optimized to batch event processing
func (ppu *PPU) CheckKeyboard() {
	// Process up to 10 events per call to avoid blocking too long
	for i := 0; i < 10; i++ {
		event = sdl.PollEvent()
		if event == nil {
			break // No more events to process
		}
		
		switch e := event.(type) {
		case *sdl.QuitEvent:
			ppu.Cleanup()
			os.Exit(0)
		
		case *sdl.KeyboardEvent:
			// Fast path for escape key
			if e.Keysym.Scancode == sdl.SCANCODE_ESCAPE && e.State == sdl.PRESSED {
				ppu.Cleanup()
				os.Exit(0)
			}
			// Other key handlers would go here
		
		// Only handle event types we care about
		// default: intentionally omitted for performance
		}
	}
}

// Cleanup releases SDL resources with proper error handling.
func (ppu *PPU) Cleanup() {
	// Use a single defer function to log completion
	defer fmt.Println("SDL resources cleaned up.")
	
	// Destroy resources in reverse order of creation
	if ppu.texture != nil {
		ppu.texture.Destroy()
		ppu.texture = nil
	}
	
	if ppu.renderer != nil {
		ppu.renderer.Destroy()
		ppu.renderer = nil
	}
	
	if ppu.window != nil {
		ppu.window.Destroy()
		ppu.window = nil
	}
	
	// Final SDL shutdown
	sdl.Quit()
}