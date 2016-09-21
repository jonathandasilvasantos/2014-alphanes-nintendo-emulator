Alphanes, 2014, 2015 - Author: Jonathan da Silva Santos (zerojnt)
	e-mail: silva.santos.jonathan@gmail.com
	
Alphanes is an attempt to create a Nintendo Entertainment System written in GO Language.

* The CPU (6502) is completed and already can handle ROMs that its mapper is zero.
* Unfortunatelly I can't find any good wrapper (and or) any good library to display graphics in GO; So I couldn't develop the Nintendo PPU.

I hope this code help you to understand more about the 6502 processor and the Nintentdo Memory Mappers.

=========================

How to use:

	1) Compile the code go install src/zerojnt/alphanes.go
	2) Run alphanes passing as parametter the rom file. Ex: ./alphanes mario.nes
	
How to debug:

	1) If you want to log the emulator, pass as second parametter ther name os the debug file that you would like to create. Ex: ./alphanes mario.rom mario.txt
	
Thank you
	