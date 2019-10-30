package main

/*
*** WORKING VERSION but still a WORK IN PROGRESS ***

TO DO:
	- writing to serial as goroutine ?
	- reading from serial as separate goroutine ?
	- move more functions into the library msgolib/smartparallel

Platform: Raspberry Pi

Command-line program to print a text file via the SmartParallel interface.

There's currently no checking to ensure the file is a valid and plain text
file. Passing it a binary file or something with unicode characters or anything
else fancy will result in 'undefined behaviour'.

Currently uses the stianeikeland/go-rpio library for accessing the RPi's
GPIO pins.

Usage:
	spprintfile -f <file>
		The file must be specified, otherwise the program throws
		an error (file not found.)
Other flags:
	-b	Ignore blank lines. Default: false. *** NOT IMPLEMENTED YET ***
	-c <int>
		Number of print columns. Defaults to 80. Other valid values are 132
		(condensed mode) and 40 (double-width mode). All other values are
		ignored and the program will default back to 80.
	-m <string>
		Print mode - alternative to -c and will override -c if used
		together. Valid modes are:
			'norm'	- normal (default) - equivalent to -c 80.
			'cond'	- condensed - equivalent to -c 132
			'wide'	- wide/enlarged - equivalent to -c 40
	-p 	Get printer status. If this is passed, all other flags are ignored.
		Default: false.
	-s 	Split long lines (default). Any lines longer that the number of columns
		will be split by looking for a suitable space - so that words don't get
		split. If there is no space earlier in the line, the text will simply be
		split at the defined number of columns, which is also the behaviour if
		this option is set to false. Default: false.
	-t	Truncate lines instead of splitting/wrapping them. Default: false.
	-v  Verbose mode. Default: false.
	-x  Experimental mode - could mean anything. All other flags ignored.
		Default: false.

*/

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mspeculatrix/msgolib/smartparallel"
	"github.com/stianeikeland/go-rpio"
	"github.com/tarm/serial"
)

const (
	splitChar        = 32              // space, where to split long lines
	ctsPin           = rpio.Pin(18)    // GPIO for CTS - BCM numbering
	ctsActiveLevel   = rpio.Low        // active low or active high?
	comPort          = "/dev/ttyS0"    // Rpi3/4 mini-UART
	baudRate         = 19200           // fast enough
	timeoutLimit     = 10              // max number of tries
	sendTimeoutDelay = time.Second / 2 // interval between tries
	readTimeout      = 2               // seconds
)

var (
	readBuf = make([]byte, smartparallel.ReadBufSize) // serial input buffer
	// defaults
	interfaceReady          = false
	prevInterfaceReadyState = interfaceReady
	// command line arguments
	printerColumns   = 80
	printMode        = "norm"
	infoBanner       = false
	ignoreBlankLines = false
	splitLongLines   = false
	truncateLines    = false
	experimental     = false
	verbose          = false
	debug            = false
	filepath         = ""
	modeSetCode      = []byte{}
	modeUnsetCode    = []byte{}
)

func interfaceIsReady() bool {
	if ctsPin.Read() == ctsActiveLevel {
		interfaceReady = true
	} else {
		interfaceReady = false
	}
	if interfaceReady != prevInterfaceReadyState {
		if interfaceReady {
			debugPrintln("-- CTS OK")
		} else {
			debugPrintln("-- CTS OFFLINE")
		}
		prevInterfaceReadyState = interfaceReady
	}
	return interfaceReady
}

func printerState(sPort *serial.Port) string {
	var result string
	sPort.Write([]byte{smartparallel.SerialCommandChar}) // command byte
	sPort.Write([]byte{smartparallel.CmdReportState})
	sPort.Write([]byte{smartparallel.Terminator})
	_, result = smartparallel.CheckSerialInput(sPort, readBuf)
	return result
}

func debugPrint(msgs ...string) {
	if debug {
		for _, msg := range msgs {
			fmt.Print(msg)
		}
	}
}

func debugPrintln(msgs ...string) {
	if debug {
		for _, msg := range msgs {
			fmt.Print(msg)
		}
		fmt.Println()
	}
}

func verbosePrint(msgs ...string) {
	if verbose {
		for _, msg := range msgs {
			fmt.Print(msg)
		}
	}
}

func verbosePrintln(msgs ...string) {
	if verbose {
		for _, msg := range msgs {
			fmt.Print(msg)
		}
		fmt.Println()
	}
}

func main() {
	// GPIO
	gpioErr := rpio.Open()
	if gpioErr != nil {
		log.Fatal("*** ERROR: Could not open GPIO ***")
	}
	defer rpio.Close()
	ctsPin.Input()
	//ctsPin.PullUp() // CTS is active low

	// configure serial
	com := &serial.Config{Name: comPort, Baud: baudRate,
		ReadTimeout: readTimeout}
	serialPort, err := serial.OpenPort(com)
	if err != nil {
		log.Fatal(err)
	}

	// command-line flags
	flag.StringVar(&filepath, "f", filepath, "filename (with full path)")
	flag.BoolVar(&ignoreBlankLines, "b", ignoreBlankLines, "ignore blank lines")
	flag.BoolVar(&debug, "d", debug, "debug mode - more verbose")
	flag.BoolVar(&infoBanner, "i", infoBanner, "print banner")
	flag.StringVar(&printMode, "m", printMode, "print mode")
	flag.BoolVar(&splitLongLines, "s", splitLongLines,
		"split long lines on spaces")
	flag.BoolVar(&truncateLines, "t", truncateLines,
		"truncate lines instead of splitting")
	flag.BoolVar(&verbose, "v", verbose, "verbose mode")
	flag.BoolVar(&experimental, "x", experimental, "experimental mode")
	flag.Parse()
	if filepath == "" {
		log.Fatal("*** ERROR: No file specified")
	}
	if debug {
		verbose = true
	}

	// *************************************************************************
	// *****   READ FILE or GET STATUS                                     *****
	// *************************************************************************
	if experimental {
		// Put anything you want here just to try stuff out
	} else {
		verbosePrintln("Reading from file: ", filepath)
		fh, err := os.Open(filepath)
		if err != nil {
			log.Fatal("*** ERROR: Problem opening file", err)
		}
		defer fh.Close()
		scanner := bufio.NewScanner(fh) // to read line-by-line
		lines := []string{}             // array to hold lines
		switch printMode {
		case "emph":
			modeSetCode = []byte{27, 69}   // ESC E
			modeUnsetCode = []byte{27, 70} // ESC F
			printerColumns = 80
		case "wide":
			modeSetCode = []byte{27, 87}   // ESC W
			modeUnsetCode = []byte{27, 87} // ESC W
			printerColumns = 40
		case "cond":
			modeSetCode = []byte{15}   // SHIFT IN
			modeUnsetCode = []byte{18} // DC2
			printerColumns = 132
		default:
			modeSetCode = []byte{0}
			modeUnsetCode = []byte{0}
			printerColumns = 80
		}
		if infoBanner {
			dt := time.Now()
			date := dt.Format("2006-01-02")
			time := dt.Format("15:04")
			banner := ">>> File: " + filepath +
				"  -  Mode: " + printMode +
				"  -  Printed: " + date +
				"  " + time + " <<<"
			dotline := strings.Repeat("-", len(banner))
			lines = append(lines, dotline)
			lines = append(lines, banner)
			lines = append(lines, dotline)
		}
		for scanner.Scan() { // iterate over lines in file
			line := scanner.Text() // get next line
			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
			if len(line) <= printerColumns {
				lines = append(lines, line)
			} else { // line is too long for printer
				if truncateLines { // we've opted simply to trucate long lines
					line = line[:printerColumns]
				} else {
					for len(line) > printerColumns {
						splitIdx := printerColumns
						foundSpace := false
						removeLeadingSpace := 0
						if splitLongLines { // split long lines at spaces?
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
									// we never found a space, so just split the
									// text anyway by resetting to default
									// column setting
									splitIdx = printerColumns
								}
							} else {
								// the line will naturally split on a space at
								// the default column setting. But is the space
								// at the end of this line or the beginning of
								// the next? That will determine whether we need
								// to remove a leading space from the beginning
								// of the next line.
								if line[printerColumns] == splitChar {
									removeLeadingSpace = 1
								}
							}
						} // if dontSplitLongLines
						// get next line
						lines = append(lines, line[:splitIdx])
						// truncate line by removing text found above.
						// If we split on a space above, the next line will
						// start with that space, which we now need to remove
						line = line[splitIdx+removeLeadingSpace:]
					} // for len(line)
					// By now, the length of any remaining text in line is
					// <= printerColumns, so we can simply add any remaining
					// text from the paragraph.
					if len(line) > 0 {
						lines = append(lines, line)
					}
				} // if truncateLines
			} // if len(line)
		} // for scanner.Scan()

		verbosePrintln("Lines read: ", strconv.Itoa(len(lines)))

		// *********************************************************************
		// *****   SEND TO SMARTPARALLEL                                   *****
		// *********************************************************************
		verbosePrintln("Sending to SmartParallel")
		verbosePrint("Initialising printer... ")
		if interfaceIsReady() {
			verbosePrintln("ready")
			serialPort.Write(smartparallel.Init)
			serialPort.Write(smartparallel.TransmitEnd)
			for ctsActiveLevel == ctsPin.Read() {
				// wait for CTS to go high
			}
		} else {
			verbosePrintln("ERROR")
			log.Fatal("Interface wasn't ready")
		}

		timeoutCounter := 0
		// ctsTimeoutCounter := 0
		timedOut := false
		lineCount := 0
		// If a timeout happens, while waiting for CTS to go high, sending of
		// lines will stop, so that's a fail condition. The outside loop below
		// will continue, but silently, without ouput. That's okay because,
		// although it takes time, it's a very small amount of time and this is
		// not a performance-critical program.
		if printMode != "norm" {
			debugPrintln("Mode: ", printMode)
			serialPort.Write(modeSetCode)
			//serialPort.Write(smartparallel.TransmitEnd)
		}
		for _, line := range lines {
			lineSent := false
			for !timedOut && !lineSent {
				if interfaceIsReady() {
					timeoutCounter = 0
					verbosePrintln(line)
					_, writeError := serialPort.Write([]byte(line))
					if writeError != nil {
						log.Fatal("*** ERROR: Write error", writeError)
					}
					serialPort.Write(smartparallel.LineEnd)
					serialPort.Write(smartparallel.TransmitEnd)
					lineSent = true
					lineCount++
					// Wait for CTS to go offline. It should go offline as
					// soon as SmartParallel receives the
					// smartparallel.TransmitEnd terminator above. It will then
					// stay offline until the SmartParallel has finished
					// printing the text sent, so we should have a reasonable
					// amount of time in which to detect that it's offline
					debugPrintln("-- waiting for CTS to go offline")
					for ctsActiveLevel == ctsPin.Read() {
						// do nothing - just loop while the CTS line is still
						// in the active (online) state, waiting for it to
						// go offline.
						// Could possibly put a tiny delay in here, but too
						// much could risk missing the signal change.
						// An interrupt would be nice. Is that too much to ask?
					}
					debugPrintln("-- CTS went offline")
				} else {
					timeoutCounter++
					debugPrint(".")
					if timeoutCounter == timeoutLimit {
						timedOut = true
						debugPrintln("!")
						verbosePrintln("*** ERROR: Timed out after ",
							strconv.Itoa(lineCount), " lines sent")
						timeoutCounter = 0
					} else {
						time.Sleep(sendTimeoutDelay)
					}
				}
			}
		} // for line...
		if printMode != "norm" {
			serialPort.Write(modeUnsetCode)
			serialPort.Write(smartparallel.TransmitEnd)
		}
		verbosePrintln("Sent ", strconv.Itoa(lineCount), " lines")
	} // else
}
