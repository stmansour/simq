package util

import (
	"fmt"
	"log"
	"unicode"
)

// PrintHexAndASCII formats the data in buffer so we can get an idea of what it holds
func PrintHexAndASCII(buffer []byte, maxChars int) {
	if maxChars < 1 || maxChars > len(buffer) {
		maxChars = len(buffer)
	}

	for i := 0; i < maxChars; i += 16 {
		// Print hex values
		for j := 0; j < 16; j++ {
			if i+j < maxChars {
				fmt.Printf("%02X ", buffer[i+j])
			} else {
				fmt.Print("   ")
			}
		}

		// Print ASCII values
		fmt.Print(" | ")
		for j := 0; j < 16; j++ {
			if i+j < maxChars {
				b := buffer[i+j]
				if unicode.IsPrint(rune(b)) {
					fmt.Printf("%c", b)
				} else {
					fmt.Print(".")
				}
			}
		}
		fmt.Println()
	}
}

// LogHexAndASCII formats the data in buffer so we can get an idea of what it holds
// and logs it to the log file
func LogHexAndASCII(buffer []byte, maxChars int) {
	var s string
	if maxChars < 1 || maxChars > len(buffer) {
		maxChars = len(buffer)
	}

	for i := 0; i < maxChars; i += 16 {
		s = "\n"
		for j := 0; j < 16; j++ {
			if i+j < maxChars {
				s += fmt.Sprintf("%02X ", buffer[i+j])
			} else {
				s += "   "
			}
		}

		// Print ASCII values
		s += " | "
		for j := 0; j < 16; j++ {
			if i+j < maxChars {
				b := buffer[i+j]
				if unicode.IsPrint(rune(b)) {
					s += fmt.Sprintf("%c", b)
				} else {
					s += "."
				}
			}
		}
		s += "\n"
	}

	log.Printf("%s", s)
}
