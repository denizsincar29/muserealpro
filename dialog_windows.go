//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// getSaveFilePath returns the path where the HTML output should be written.
//
// On Windows, when showDialog is true (i.e. the program was invoked with a
// single input file and no -o flag, the typical "Open With" scenario), a
// native Save File dialog is displayed via PowerShell so the user can choose
// the destination.  If the dialog is cancelled or fails, the function falls
// back to defaultPath and returns ok=true so that the caller still saves the
// file next to the input.
//
// When showDialog is false (multiple files, or the user already provided -o),
// defaultPath is returned immediately.
func getSaveFilePath(defaultPath string, showDialog bool) (string, bool) {
	if !showDialog {
		return defaultPath, true
	}

	// Escape single quotes for PowerShell single-quoted strings.
	safe := strings.ReplaceAll(defaultPath, "'", "''")

	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$d = New-Object System.Windows.Forms.SaveFileDialog
$d.Filter      = 'HTML files (*.html)|*.html|All files (*.*)|*.*'
$d.DefaultExt  = 'html'
$d.FileName    = [System.IO.Path]::GetFileName('%s')
$d.InitialDirectory = [System.IO.Path]::GetDirectoryName('%s')
if ($d.ShowDialog() -eq 'OK') { Write-Output $d.FileName }
`, safe, safe)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		// PowerShell unavailable or the script failed; use the default path.
		return defaultPath, true
	}

	result := strings.TrimSpace(string(out))
	if result == "" {
		// User cancelled the dialog; signal caller to abort.
		return "", false
	}
	return result, true
}
