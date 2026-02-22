//go:build !windows

package main

// getSaveFilePath returns the path where the HTML output should be written.
//
// On non-Windows platforms no dialog is shown; the default path (next to the
// input file, with a .html extension) is always used.
func getSaveFilePath(defaultPath string, showDialog bool) (string, bool) {
	return defaultPath, true
}
