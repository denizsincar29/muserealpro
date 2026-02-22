# task
Make a program in Go (or rust) that parses musescore 4 file and converts to IRealPro chord charts

## Description
This program will read a musescore 4 file, extract the chord information and other relevant musical data like rehearsal marks and repeats, and convert it into a format compatible with IRealPro chord charts. The output will be an HTML file containing the chord charts that can be imported into IRealPro.

## Implementation
Search the web for libraries or your own implementation methods to parse Musescore4 files and make IRealPro files.
Keep in mind that the go musescore library (last commit 4 years ago) is for musescore 3, so skip it.
Make the program compatable with "open with" in windows, and create a native save file dialog if ran on windows and using open with. Otherwise save near the input file or by -o flag.
Or better, compile with feature openwith or how is it done in go...

## Notes
Search out rust libraries for parsing musescore or music xml. If the implementation in rust is better or easier, concidder deleting go.mod and writing the program in rust instead.