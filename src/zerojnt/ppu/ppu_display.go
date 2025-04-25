// File: ./ppu/ppu_display.go
// Contains SDL-specific logic for PPU display functionality

package ppu

import (
	"fmt"
	"log"
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
	// Reusable event for polling
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

	// Create renderer with hardware acceleration
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

	// Use nearest neighbor scaling for pixel art
	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "0")
	
	// Performance hints
	sdl.SetHint(sdl.HINT_RENDER_DRIVER, "opengl")
	sdl.SetHint(sdl.HINT_RENDER_BATCHING, "1")
	sdl.SetHint(sdl.HINT_VIDEO_X11_NET_WM_BYPASS_COMPOSITOR, "1")
	sdl.SetHint(sdl.HINT_RENDER_VSYNC, "0")
	
	// Create streaming texture
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

	// Set draw color
	if err = ppu.renderer.SetDrawColor(0, 0, 0, 255); err != nil {
		log.Printf("Warning: Failed to set draw color: %v", err)
	}
	
	// Initialize frame timing
	lastFrameTime = time.Now()
	
	log.Println("SDL Canvas Initialized Successfully (Optimized 30FPS)")
	return nil
}

// ShowScreen updates the SDL texture with the PPU's framebuffer data and presents it.
func (ppu *PPU) ShowScreen() {
	if ppu.renderer == nil || ppu.texture == nil {
		return
	}

	// Calculate time since last frame
	now := time.Now()
	elapsed := now.Sub(lastFrameTime)
	
	// Skip frame if not enough time has passed
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
		return
	}

	// Calculate pitch for ARGB8888 format
	const pitch = SCREEN_WIDTH * 4
	
	// Direct pointer for maximum update speed
	pixelsPtr := unsafe.Pointer(&ppu.SCREEN_DATA[0])

	// Update texture with framebuffer data
	err := ppu.texture.Update(nil, pixelsPtr, pitch)
	
	fbMutex.RUnlock()
	
	if err != nil {
		log.Printf("Texture update failed: %v", err)
		return
	}

	// Clear with preset color
	ppu.renderer.Clear()
	
	// Copy texture to renderer
	ppu.renderer.Copy(ppu.texture, nil, nil)
	
	// Present frame
	ppu.renderer.Present()
}

// Cleanup releases SDL resources
func (ppu *PPU) Cleanup() {
	defer fmt.Println("SDL resources cleaned up.")
	
	// Destroy resources in reverse order
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
	
	sdl.Quit()
}