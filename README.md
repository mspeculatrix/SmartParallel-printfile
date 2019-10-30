# spprintfile

See: https://mansfield-devine.com/speculatrix/projects/smartparallel-serial-to-parallel-printer-interface/

Go command line utility for the Raspberry Pi to print a text file via the SmartParallel interface.

Now working, but still (and forever) a work in progress.

There's currently no checking to ensure the file is a valid plain text file. Passing it a binary file or something with weird unicode characters will result in 'undefined behaviour'.

Currently uses the stianeikeland/go-rpio library for accessing the RPi's GPIO pins.

### Usage:
**spprintfile [flags] -f \<file>**

The file must be specified this way, otherwise the program throws an error (file not found.)

Optional flags:

**-b**

Ignore blank lines. ** NOT IMPLEMENTED YET **

**-d**

Debug mode. Produces more verbose output than -v.

**-i**

Print info banner. Prints the filename, print mode, date and time before the text.

**-m \<string>**

Print mode. Sets number of columns and print style (assumes Epson control codes). Valid values are:
	'norm'	- normal, 80-column (default)
	'cond'	- condensed, 132-column.
	'emph'	- emphasised, 80-column
All other values are ignored and the program will default back to 80.

**-s**

Split long lines. Normally, any lines longer that the number of columns will be split crudely, simply by splitting at the last column. With this mode selected, it looks for a suitable space earlier in the line and splits there - so that lines don't get split mid-word. If there is no space earlier in the line, the text will simply be split at the defined number of columns. Default: false.

**-t**

Truncate lines instead of splitting/wrapping them. Default: false.

**-v**

Verbose mode. Print various entertaining messages. Default: off.

**-x**

Experimental mode. Will execute a block of code that can be edited for the purposes of mucking about. Default: false.
