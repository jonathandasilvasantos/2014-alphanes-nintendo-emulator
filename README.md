# Alphanes NES Emulator

Alphanes is a Nintendo Entertainment System (NES) emulator written in Go. It aims to emulate the NES hardware, allowing users to play classic NES games on modern computers.

This project was originally developed by Jonathan da Silva Santos between 2014-2015.

## Features

Based on the provided source code, Alphanes includes implementations for several key NES components:

*   **CPU (Ricoh 2A03):**
    *   Emulation of the 6502-based processor core.
    *   Implementation of most official 6502 instructions and addressing modes.
    *   Basic NMI (Non-Maskable Interrupt) handling.
    *   Cycle timing approximation aiming for NTSC frequency (1.789773 MHz).
*   **PPU (Picture Processing Unit - RP2C02):**
    *   Scanline-based rendering targeting 60 FPS (NTSC).
    *   Background rendering pipeline (Nametable, Attribute Table, Pattern fetching).
    *   Sprite rendering pipeline (OAM scan, secondary OAM, pattern fetching, sprite-0 hit).
    *   Basic scrolling implementation (via `v`, `t`, `x`, `w` registers).
    *   Palette RAM and color rendering.
    *   VBlank NMI generation.
    *   SDL2 integration for display output (appears optimized for 30fps updates in `ppu_display.go`, while the main loop targets 60fps logic).
*   **APU (Audio Processing Unit):**
    *   Implementation of Pulse (x2), Triangle, and Noise channels.
    *   Basic DMC channel structure (appears less complete).
    *   Envelope generators and Sweep units (for Pulse channels).
    *   Length counters.
    *   Frame Counter logic with 4-step and 5-step modes, IRQ generation.
    *   Mixing of audio channels using non-linear formulas.
    *   Audio output via PortAudio library using a ring buffer for sample transfer.
*   **Cartridge Loading:**
    *   Supports the iNES ROM file format.
    *   Parses header information.
    *   Loads PRG (Program) and CHR (Character/Graphics) ROM data.
    *   Supports CHR RAM.
    *   Supports battery-backed SRAM (PRG RAM).
*   **Memory Mappers:**
    *   Provides a framework for different memory mapping hardware found on NES cartridges.
    *   Implemented Mappers:
        *   Mapper 0 (NROM)
        *   Mapper 1 (MMC1 - with variant detection like SNROM, SUROM, etc.)
        *   Mapper 2 (UNROM)
        *   Mapper 4 (MMC3 - includes PRG/CHR banking, IRQ counter)
*   **Input:**
    *   Handles keyboard input via SDL2.
    *   Maps keys to standard NES controller buttons (Controller 1).
*   **Debugging:**
    *   Optional support for comparing CPU state against a `nestest.log`-compatible debug file.
    *   Optional support for loading a PPU memory dump file (`.ppu`).
    *   Performance reporting (FPS, CPU utilization estimate).

## Building and Running

### Prerequisites

1.  **Go Compiler:** Ensure you have a recent version of Go installed (https://golang.org/doc/install).
2.  **SDL2 Development Libraries:** You need the SDL2 library headers and library files.
    *   **Ubuntu/Debian:** `sudo apt-get install libsdl2-dev`
    *   **macOS (Homebrew):** `brew install sdl2`
    *   **Windows (MSYS2/MinGW):** Use `pacman` to install the `mingw-w64-x86_64-SDL2` package.
3.  **PortAudio Development Libraries:** You need the PortAudio library headers and library files.
    *   **Ubuntu/Debian:** `sudo apt-get install portaudio19-dev`
    *   **macOS (Homebrew):** `brew install portaudio`
    *   **Windows (MSYS2/MinGW):** Use `pacman` to install the `mingw-w64-x86_64-portaudio` package.

### Building

Navigate to the root directory containing the `alphanes` folder and run:

```bash
go build ./alphanes


This will create an executable file named alphanes (or alphanes.exe on Windows) in the current directory.

Running

To run the emulator, provide the path to an NES ROM file (.nes extension):

./alphanes <path/to/your/rom.nes>
IGNORE_WHEN_COPYING_START
content_copy
download
Use code with caution.
Bash
IGNORE_WHEN_COPYING_END

Optional Debugging:

If you have a debug log file (e.g., from nestest) or a PPU dump file, you can provide its path as the second argument:

# For CPU state comparison
./alphanes nestest.nes nestest.log

# For PPU dump analysis (exact usage depends on implementation)
./alphanes your_game.nes your_game_dump.ppu

# For both (if supported by argument parsing logic)
./alphanes your_game.nes your_game.debug.ppu
IGNORE_WHEN_COPYING_START
content_copy
download
Use code with caution.
Bash
IGNORE_WHEN_COPYING_END
Controls (Default Keyboard Mapping)

A Button: Z

B Button: X

Select: Space

Start: Return (Enter)

Up: Up Arrow

Down: Down Arrow

Left: Left Arrow

Right: Right Arrow

Quit Emulator: Escape

Current Status & Limitations

This is an emulator likely developed for learning or specific testing purposes. Its accuracy compared to real hardware or mature emulators may vary.

While several mappers are implemented, edge cases or less common mapper features might be missing.

Audio emulation includes the main channels but may lack perfect accuracy or advanced features (DMC seems basic).

Performance seems reasonable, but optimization might be ongoing (e.g., PPU display update limited to 30fps).

Controller support is limited to keyboard input for Player 1.

Save states are not explicitly mentioned in the provided code. SRAM saving depends on the cartridge/mapper implementation.

License

Alphanes is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

Alphanes is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License along with Alphanes. If not, see http://www.gnu.org/licenses/.

Author

Jonathan da Silva Santos (Original Author, 2014-2015)