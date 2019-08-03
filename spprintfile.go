package main

/*
*** WORK IN PROGRESS - NOT COMPLETE ***

Command-line program to print a text file via the SmartParallel interface.

There's currently no checking to ensure the file is a valid and plain text
file. Passing it a binary file or something with weird unicode characters
will result in 'undefined behaviour'.

Currently uses the stianeikeland/go-rpio library for accessing the RPi's
GPIO pins.
*/

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/stianeikeland/go-rpio"
	"github.com/tarm/serial"
)

const (
	splitChar    = 32          // space char, decides where to split long lines
	terminator   = 0           // null terminator marks end of transmission
	ctsPin       = rpio.Pin(8) // GPIO for CTS
	sendBufSize  = 256         // send buffer size, in bytes
	comPort      = "/dev/ttyS0"
	baudRate     = 19200
	timeoutLimit = 10              // max number of tries
	timeoutDelay = time.Second / 2 // interval between tries
)

var (
	lineend     = []byte{13, 10}     // not constant, may change later
	transmitEnd = []byte{terminator} // terminator as byte array
	// defaults
	printerColumns = 80
	splitLongLines = true
	//filepath := ""
	filepath = "/home/pi/code/test.txt" // for testing only
)

func main() {

	// GPIO
	gpioErr := rpio.Open()
	if gpioErr != nil {
		log.Fatal("*** ERROR: Could not open GPIO")
	}
	defer rpio.Close()
	ctsPin.Input()
	ctsPin.PullUp() // make CTS active low

	// configure serial
	com := &serial.Config{Name: comPort, Baud: baudRate}
	serialPort, err := serial.OpenPort(com)
	if err != nil {
		log.Fatal(err)
	}

	// command-line flags
	flag.StringVar(&filepath, "f", filepath, "filename (with full path)")
	flag.IntVar(&printerColumns, "c", printerColumns, "number of columns")
	flag.BoolVar(&splitLongLines, "s", splitLongLines, "split long lines on spaces")
	flag.Parse()
	if filepath == "" {
		log.Fatal("*** ERROR: No file specified")
	}

	// *************************************************************************
	// *****   READ FILE                                                   *****
	// *************************************************************************

	fh, err := os.Open(filepath)
	if err != nil {
		log.Fatal("*** ERROR: Problem opening file", err)
	}
	defer fh.Close()
	scanner := bufio.NewScanner(fh) // to read line-by-line
	lines := []string{}             // array to hold lines
	for scanner.Scan() {            // iterate over lines in file
		line := scanner.Text() // get next line
		if len(line) <= printerColumns {
			lines = append(lines, line)
		} else { // line is too long for printer
			for len(line) > printerColumns {
				splitIdx := printerColumns
				foundSpace := false
				removeLeadingSpace := 0
				if splitLongLines { // do we want to split long lines at spaces?
					if (line[printerColumns-1] != splitChar) &&
						(line[printerColumns] != splitChar) {
						// the line isn't going to split at a space
						for !foundSpace && splitIdx > 0 {
							splitIdx--
							if line[splitIdx] == splitChar {
								foundSpace = true
								removeLeadingSpace = 1
							}
						}
						if !foundSpace {
							// we never found a space, so just split the text
							// anyway by resetting to default column setting
							splitIdx = printerColumns
						}
					} else {
						// the line will naturally split on a space at the
						// default column setting. But is the space at the end
						// of this line or the beginning of the next? That will
						// determine whether we need to remove a leading space
						// from the beginning of the next line.
						if line[printerColumns] == splitChar {
							removeLeadingSpace = 1
						}
					}
				}
				// get next line
				lines = append(lines, line[:splitIdx])
				// truncate line by removing text found above.
				// If we split on a space above, the next line
				// will start with that space, which we now need to remove
				line = line[splitIdx+removeLeadingSpace:]
			}
			// add any remaining text from paragraph
			if len(line) > 0 {
				lines = append(lines, line)
			}
		}
	}
	// *************************************************************************
	// *****   SEND TO SMARTPARALLEL                                       *****
	// *************************************************************************
	fmt.Println("Sending to printer")
	timeoutCounter := 0
	timedOut := false
	lineCount := 0
	// If a timeout happens, while waiting for CTS to go low, sending of lines
	// will stop, so that's a fail condition. The main loop below will continue,
	// but silently, without ouput. That's okay because, although it takes time,
	// this is not a performance-critical program.
	for _, line := range lines {
		for !timedOut {
			if ctsPin.Read() == rpio.Low {
				timeoutCounter = 0
				lineCount++
				fmt.Println(line)
				_, writeError := serialPort.Write([]byte(line))
				if writeError != nil {
					fmt.Println("fuck", writeError)
				}
				serialPort.Write(lineend)
				serialPort.Write(transmitEnd)
			} else {
				timeoutCounter++
				fmt.Print(".")
				if timeoutCounter == timeoutLimit {
					timedOut = true
					fmt.Println("timed out")
					log.Println("*** ERROR: Timed out after", lineCount, "lines")
				} else {
					time.Sleep(timeoutDelay)
				}
			}
		}
	}
	log.Println("Sent", lineCount, "lines")
}
