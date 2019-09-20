package main

/*
*** WORK IN PROGRESS - NOT COMPLETE ***

TO DO:
	- writing to serial as goroutine ?
	- reading from serial as separate goroutine ?

Command-line program to print a text file via the SmartParallel interface.

There's currently no checking to ensure the file is a valid and plain text
file. Passing it a binary file or something with unicode characters will result
in 'undefined behaviour'.

Currently uses the stianeikeland/go-rpio library for accessing the RPi's
GPIO pins.

Usage:
	spprintfile -f <file>
		The file must be specified this way, otherwise the program throws
		an error (file not found.)
Other flags:
	-b	Ignore blank lines. Default: false. *** NOT IMPLEMENTED YET ***
	-c <int>
		Number of print columns. Defaults to 80. Other valid values are 132
		(condensed mode) and 40 (double-width mode). All other values are
		ignored and the program will default back to 80.
	-p 	Get printer status. If this is passed, all other flags are ignored.
		Default: false.
	-s 	Split long lines (default). Normally, any lines longer that the
		number of columns will be split by looking for a suitable space - so
		that words don't get split. If there is no space earlier in the line,
		the text will simply be split at the defined number of columns.
		Default: false.
	-t	Truncate lines instead of splitting/wrapping them. Default: false.

*/

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mspeculatrix/msgolib/smartparallel"
	"github.com/stianeikeland/go-rpio"
	"github.com/tarm/serial"
)

const (
	splitChar      = 32           // space, decides where to split long lines
	terminator     = 0            // null terminator marks end of transmission
	ctsPin         = rpio.Pin(18) // GPIO for CTS - BCM numbering
	ctsActiveLevel = rpio.Low     // active low or active high?
	//sendBufSize      = 256             // send buffer size, in bytes
	comPort          = "/dev/ttyS0"    // Rpi3/4 mini-UART
	baudRate         = 19200           // fast enough
	timeoutLimit     = 10              // max number of tries
	defaultColumns   = 80              // because it's an Epson MX-80
	sendTimeoutDelay = time.Second / 2 // interval between tries
	readTimeout      = 2               // seconds
	readBufSize      = 1024            // bytes
)

var (
	lineend           = []byte{13, 10}        // CR and LF
	transmitEnd       = []byte{terminator}    // terminator as byte array
	validColumnWidths = [...]int{40, 80, 132} // fixed size array
	//printerState      = "UNKNOWN"
	//printerStates     = [...]string{"READY", "DONE", "INIT", "OFFLINE",
	//	"PRINTING", "BUSY", "ERROR", "PE", "ACK_TIMEOUT", "BUSY_TIMEOUT"}
	readBuf = make([]byte, readBufSize)
	// defaults
	printerReady          = false
	prevPrinterReadyState = printerReady
	// command line arguments
	printerColumns   = defaultColumns
	ignoreBlankLines = false
	splitLongLines   = false
	truncateLines    = false
	getStatus        = false
	//filepath := ""
	filepath = "/home/pi/code/test.txt" // for testing only
)

func printerIsReady() bool {
	if ctsPin.Read() == ctsActiveLevel {
		printerReady = true
	} else {
		printerReady = false
	}
	if printerReady != prevPrinterReadyState {
		if printerReady {
			fmt.Println("-- clear to send")
		} else {
			fmt.Println("-- printer is not ready")
		}
		prevPrinterReadyState = printerReady
	}
	return printerReady
}

func main() {
	// GPIO
	gpioErr := rpio.Open()
	if gpioErr != nil {
		log.Fatal("*** ERROR: Could not open GPIO")
	}
	defer rpio.Close()
	ctsPin.Input()
	//ctsPin.PullUp() // make CTS active low

	// configure serial
	com := &serial.Config{Name: comPort, Baud: baudRate, ReadTimeout: readTimeout}
	serialPort, err := serial.OpenPort(com)
	if err != nil {
		log.Fatal(err)
	}

	serialPort.Write([]byte("Testing the SmartParallel"))
	serialPort.Write(transmitEnd)
	// command-line flags
	flag.StringVar(&filepath, "f", filepath, "filename (with full path)")
	flag.BoolVar(&ignoreBlankLines, "b", ignoreBlankLines, "ignore blank lines")
	flag.IntVar(&printerColumns, "c", printerColumns, "number of columns")
	flag.BoolVar(&getStatus, "p", getStatus, "get printer status")
	flag.BoolVar(&splitLongLines, "s", splitLongLines, "split long lines on spaces")
	flag.BoolVar(&truncateLines, "t", truncateLines, "truncate lines instead of splitting")
	flag.Parse()
	if filepath == "" && getStatus == false {
		log.Fatal("*** ERROR: No file specified")
	}
	validColumns := false
	for _, cols := range validColumnWidths {
		if printerColumns == cols {
			validColumns = true
		}
	}
	if !validColumns {
		printerColumns = defaultColumns
	}

	// *************************************************************************
	// *****   READ FILE                                                   *****
	// *************************************************************************
	fmt.Println("=========================")
	if getStatus {
		fmt.Println("Checking printer status")
		serialPort.Write([]byte{smartparallel.SerialCommandChar}) // command byte
		serialPort.Write([]byte{smartparallel.CmdReportState})
		checkSerialInput(serialPort)
	} else {
		fmt.Println("Reading from file...")
		fh, err := os.Open(filepath)
		if err != nil {
			log.Fatal("*** ERROR: Problem opening file", err)
		}
		defer fh.Close()
		scanner := bufio.NewScanner(fh) // to read line-by-line
		lines := []string{}             // array to hold lines
		for scanner.Scan() {            // iterate over lines in file
			line := scanner.Text() // get next line
			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
			if len(line) <= printerColumns {
				lines = append(lines, line)
			} else { // line is too long for printer
				if truncateLines {
					line = line[:printerColumns]
				} else {
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
						} // if dontSplitLongLines
						// get next line
						lines = append(lines, line[:splitIdx])
						// truncate line by removing text found above.
						// If we split on a space above, the next line
						// will start with that space, which we now need to remove
						line = line[splitIdx+removeLeadingSpace:]
					} // for len(line)
					// add any remaining text from paragraph
					if len(line) > 0 {
						lines = append(lines, line)
					}
				} // if truncateLines
			} // if len(line)
		} // for scanner.Scan()

		fmt.Println("........................")
		fmt.Println("Lines read:")
		for _, line := range lines {
			fmt.Println(line)
		}
		// *************************************************************************
		// *****   SEND TO SMARTPARALLEL                                       *****
		// *************************************************************************
		fmt.Println("........................")
		fmt.Println("Sending to SmartParallel")
		timeoutCounter := 0
		timedOut := false
		lineCount := 0
		// If a timeout happens, while waiting for CTS to go high, sending of lines
		// will stop, so that's a fail condition. The outside loop below will
		// continue, but silently, without ouput. That's okay because, although it
		// takes time, it's a very small amount of time and this is not a
		// performance-critical program.
		for _, line := range lines {
			lineSent := false
			for !timedOut && !lineSent {
				if printerIsReady() {
					fmt.Print(">")
					timeoutCounter = 0
					lineCount++
					fmt.Println(line)
					_, writeError := serialPort.Write([]byte(line))
					if writeError != nil {
						fmt.Println("*** ERROR: Write error", writeError)
					}
					serialPort.Write(lineend)
					serialPort.Write(transmitEnd)
					lineSent = true
				} else {
					timeoutCounter++
					fmt.Print(".")
					if timeoutCounter == timeoutLimit {
						timedOut = true
						fmt.Println("*** ERROR: Timed out after", lineCount, "lines sent")
					} else {
						time.Sleep(sendTimeoutDelay)
					}
				}
			}
			// this is purely for experimental purposes - won't be a part of the
			// final program.
			// But we might want some way od dealing with messages from the
			// SmartParallel.
			if lineSent {
				// read from the serial port. The number of bytes read is
				// in n
				checkSerialInput(serialPort)
			}
		}
		log.Println("Sent", lineCount, "lines")
	} // else
}

func checkSerialInput(sPort *serial.Port) {
	n, err := sPort.Read(readBuf)
	time.Sleep(10 * time.Millisecond)
	if err != nil {
		if err.Error() == "EOF" {
			//fmt.Println("-- no serial data")
		} else {
			fmt.Println("Error read input", err)
		}
	} else if n > 0 {
		fmt.Println(n, "chars")
		fmt.Printf("%s\n", readBuf[:n])
	}
}
