/*
Copyright 2014, 2015 Jonathan da Silva SAntos

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
    along with Alphanes.  If not, see <http://www.gnu.org/licenses/>.
*/
package debug

import "fmt"
import "io/ioutil"
import "strings"
//import "zerojnt/ppu"
import "log"

type Debug struct {
	Lines []string
	Verbose bool
	Enable bool
}

type PPUDebug struct {
	DUMP []byte
	Enable bool
}

func OpenPPUDumpFile(filename string) PPUDebug {
	var d PPUDebug
	
	fmt.Printf("Openning PPU dump file: %s\n", filename)
	
	content, err := ioutil.ReadFile(filename)
	if err != nil {
	    log.Fatal("Error. Cannot open the debug file.")
	}
	d.Enable = false
        d.DUMP = content
	return d
}




func OpenDebugFile(filename string) Debug {
	var d Debug
	
	fmt.Printf("Openning debug file: %s\n", filename)
	
	content, err := ioutil.ReadFile(filename)
	if err != nil {
	    log.Fatal("Error. Cannot open the debug file.")
	}
	d.Lines = strings.Split(string(content), "\n")
	d.Enable = true
	return d
}

func GetPC(line string) string {
	return "0x"+line[0:4]
}

func GetOpcode(line string) string {
	return "0x"+line[6:8]
}

func GetA(line string) string {
	return "0x"+line[50:52]
}

func GetX(line string) string {
	return "0x"+line[55:57]
}

func GetY(line string) string {
	return "0x"+line[60:62]
}

func GetP(line string) string {
	return "0x"+line[65:67]
}

func GetSP(line string) string {
	return "0x"+line[71:73]
}

func GetSL(line string) string {
	return "0x"+line[76:]
}

func PrintLine(line string) {
	fmt.Printf("%s A:%s X:%s Y:%s P:%s SP:%s\n", GetPC(line), GetA(line), GetX(line), GetY(line), GetP(line), GetSP(line))
}
