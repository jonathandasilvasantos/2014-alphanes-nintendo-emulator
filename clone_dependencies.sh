#!/bin/bash

# Script to clone Go SDL2 and PortAudio repositories into specific directories

# Create the base directory structure if it doesn't exist
mkdir -p ./src/github.com/veandco
mkdir -p ./src/github.com/gordonklaus

# Clone go-sdl2 repository
echo "Cloning go-sdl2 repository..."
git clone https://github.com/veandco/go-sdl2 ./src/github.com/veandco/go-sdl2
if [ $? -eq 0 ]; then
    echo "Successfully cloned go-sdl2"
else
    echo "Failed to clone go-sdl2"
    exit 1
fi

# Clone portaudio repository
echo "Cloning portaudio repository..."
git clone https://github.com/gordonklaus/portaudio ./src/github.com/gordonklaus/portaudio
if [ $? -eq 0 ]; then
    echo "Successfully cloned portaudio"
else
    echo "Failed to clone portaudio"
    exit 1
fi

echo "All repositories cloned successfully!"
echo "Repositories are located at:"
echo "  - ./src/github.com/veandco/go-sdl2"
echo "  - ./src/github.com/gordonklaus/portaudio"
