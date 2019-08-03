# SmartParallel-printfile

*** Work in progress ***

Go command line utility to print a text file via the SmartParallel interface.

There's currently no checking to ensure the file is a valid and plain text file. Passing it a binary file or something with weird unicode characters will result in 'undefined behaviour'.

Currently uses the stianeikeland/go-rpio library for accessing the RPi's
GPIO pins.

### Usage:
**spprintfile -f <file>**
		The file must be specified this way, otherwise the program throws an error (file not found.)

Other flags:
**-c <int>**
Number of columns. Defaults to 80. Other valid values are 132 (condensed mode) and 40 (double-width mode). All other values are ignored and the program will default back to 80.

**-s=<bool>**
Split long lines (default). Normally, any lines longer that the number of columns will be split by looking for a suitable space - so that words don't get split. If there is no space earlier in the line, the text will simply be split at the defined number of columns. This latter behaviour can be enforced with: -s=false
