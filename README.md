# SmartParallel-printfile

Go command line utility for the Raspberry Pi to print a text file via the SmartParallel interface.

Now working, but still a work in progress.

There's currently no checking to ensure the file is a valid and plain text file. Passing it a binary file or something with weird unicode characters will result in 'undefined behaviour'.

Currently uses the stianeikeland/go-rpio library for accessing the RPi's
GPIO pins.

### Usage:
**spprintfile [flags] -f \<file>**

The file must be specified this way, otherwise the program throws an error (file not found.)

Other flags:

**-c \<int>**

Number of columns. Defaults to 80. Other valid values are 132 (condensed mode) and 40 (double-width mode). All other values are ignored and the program will default back to 80.

**-p**

Get the printer status. If this is used, all other flags are ignored. Default: false.

**-s**

Split long lines. Normally, any lines longer that the number of columns will be split crudely, simply by splitting at the last column. With this mode selected, it looks for a suitable space earllier in the line and splits there - so that lines don't get split mid-word. If there is no space earlier in the line, the text will simply be split at the defined number of columns. Default: false.

**-t**

Truncate lines instead of splitting/wrapping them. Default: false.

**-v**

Verbose mode. Print various entertaining messages. Default: off.

**-x**

Experimental mode. Will execute a block of code that can be edited for the purposes of mucking about. Default: false.
