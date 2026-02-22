// muserealpro converts MuseScore 4 files (.mscz / .mscx) into iRealPro
// chord-chart HTML files that can be imported directly into iRealPro.
//
// Usage:
//
//	muserealpro [-o output.html] input.mscz [input2.mscz ...]
package main

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// MuseScore XML structures
// ──────────────────────────────────────────────────────────────────────────────

type museScoreDoc struct {
	XMLName xml.Name `xml:"museScore"`
	Score   scoreXML `xml:"Score"`
}

type scoreXML struct {
	MetaTags []metaTagXML `xml:"metaTag"`
	Staves   []staffXML   `xml:"Staff"`
}

type metaTagXML struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

type staffXML struct {
	ID       int          `xml:"id,attr"`
	Measures []measureXML `xml:"Measure"`
}

// emptyElem is used for self-closing XML elements whose presence matters but
// whose content does not (e.g. <startRepeat/>).
type emptyElem struct{}

type measureXML struct {
	XMLName     xml.Name   `xml:"Measure"`
	Number      string     `xml:"number,attr"`
	StartRepeat *emptyElem `xml:"startRepeat"`
	EndRepeat   *emptyElem `xml:"endRepeat"`
	// KeySig/TimeSig/RehearsalMark can appear directly in the measure in
	// MuseScore 4 as well as inside a <voice> element.
	KeySigs        []keySigXML        `xml:"KeySig"`
	TimeSigs       []timeSigXML       `xml:"TimeSig"`
	RehearsalMarks []rehearsalMarkXML `xml:"RehearsalMark"`
	Voices         []voiceXML         `xml:"voice"`
}

type voiceXML struct {
	Harmonies      []harmonyXML       `xml:"Harmony"`
	TimeSigs       []timeSigXML       `xml:"TimeSig"`
	KeySigs        []keySigXML        `xml:"KeySig"`
	RehearsalMarks []rehearsalMarkXML `xml:"RehearsalMark"`
	BarLines       []barLineXML       `xml:"BarLine"`
}

type harmonyXML struct {
	Root int    `xml:"root"`
	Name string `xml:"name"`
	Bass int    `xml:"bass"`
}

type timeSigXML struct {
	SigN int `xml:"sigN"`
	SigD int `xml:"sigD"`
}

type keySigXML struct {
	Accidental int `xml:"accidental"`
}

type rehearsalMarkXML struct {
	Text string `xml:"text"`
}

type barLineXML struct {
	Subtype string `xml:"subtype"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal data model
// ──────────────────────────────────────────────────────────────────────────────

type timeSigData struct {
	Num, Den int
}

type chordData struct {
	Root    int    // TPC (Tonal Pitch Class, MuseScore encoding)
	Quality string // chord quality string from MuseScore <name> element
	Bass    int    // TPC for bass note; 0 means no slash bass
}

type measureData struct {
	StartRepeat   bool
	EndRepeat     bool
	RehearsalMark string
	TimeSig       *timeSigData // non-nil only for mid-song time-sig changes
	Chords        []chordData
}

type songData struct {
	Title     string
	Composer  string
	Style     string
	Key       string
	DefaultTS timeSigData // time signature that opens the song
	Measures  []measureData
}

// ──────────────────────────────────────────────────────────────────────────────
// TPC → note name
// ──────────────────────────────────────────────────────────────────────────────

// MuseScore Tonal Pitch Class: C=14 is the centre; sharps go up, flats go down.
var tpcNoteNames = map[int]string{
	6: "E", 7: "B", 8: "Gb", 9: "Db", 10: "Ab",
	11: "Eb", 12: "Bb", 13: "F", 14: "C", 15: "G",
	16: "D", 17: "A", 18: "E", 19: "B",
	20: "F#", 21: "C#", 22: "G#", 23: "D#", 24: "A#",
	25: "F", 26: "C",
}

func tpcToNote(tpc int) string {
	if n, ok := tpcNoteNames[tpc]; ok {
		return n
	}
	return "C"
}

// ──────────────────────────────────────────────────────────────────────────────
// Chord quality conversion: MuseScore name → iRealPro symbol
// ──────────────────────────────────────────────────────────────────────────────

func qualityToIReal(name string) string {
	switch strings.TrimSpace(name) {
	case "", "M", "maj", "major":
		return ""
	case "m", "mi", "min", "minor":
		return "-"
	case "7":
		return "7"
	case "M7", "maj7", "Maj7", "Ma7", "^7":
		return "^7"
	case "m7", "mi7", "min7":
		return "-7"
	case "m7b5", "m7-5", "ø7", "ø", "h", "h7", "half-dim", "hdim7":
		return "h"
	case "dim", "°", "o", "d":
		return "o"
	case "dim7", "°7", "o7":
		return "o7"
	case "aug", "+", "aug5", "augmented":
		return "+"
	case "aug7", "7+5", "7#5", "+7", "7+":
		return "7+"
	case "sus4", "sus":
		return "sus"
	case "sus2":
		return "sus2"
	case "7sus4", "7sus", "dom7sus4":
		return "7sus"
	case "6":
		return "6"
	case "m6", "min6":
		return "-6"
	case "9":
		return "9"
	case "M9", "maj9", "Maj9":
		return "^9"
	case "m9", "min9":
		return "-9"
	case "add9", "add2":
		return "2"
	case "m(add9)", "madd9", "m(add2)":
		return "-2"
	case "11":
		return "11"
	case "m11", "min11":
		return "-11"
	case "13":
		return "13"
	case "m13", "min13":
		return "-13"
	case "mM7", "mMaj7", "m(maj7)", "minmaj7", "mM":
		return "-^7"
	case "7b9":
		return "7b9"
	case "7#9":
		return "7#9"
	case "7b5", "7-5":
		return "7b5"
	case "7#11":
		return "7#11"
	case "alt", "7alt":
		return "alt"
	case "6/9", "69":
		return "69"
	default:
		return name // pass unknown qualities through as-is
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Key-signature number → key name
// ──────────────────────────────────────────────────────────────────────────────

var keySigNames = map[int]string{
	-7: "Cb", -6: "Gb", -5: "Db", -4: "Ab", -3: "Eb",
	-2: "Bb", -1: "F", 0: "C", 1: "G", 2: "D", 3: "A",
	4: "E", 5: "B", 6: "F#", 7: "C#",
}

// ──────────────────────────────────────────────────────────────────────────────
// iRealPro chord-body builder
// ──────────────────────────────────────────────────────────────────────────────

func buildIRealBody(song *songData) string {
	var sb strings.Builder

	// Opening time-signature marker (e.g. T44 for 4/4, T34 for 3/4).
	ts := song.DefaultTS
	sb.WriteString(fmt.Sprintf("T%d%d", ts.Num, ts.Den))

	for i, m := range song.Measures {
		// Rehearsal / section marker.
		if m.RehearsalMark != "" {
			switch strings.ToUpper(strings.TrimSpace(m.RehearsalMark)) {
			case "A":
				sb.WriteString("*A")
			case "B":
				sb.WriteString("*B")
			case "C":
				sb.WriteString("*C")
			case "D":
				sb.WriteString("*D")
			case "I", "INTRO":
				sb.WriteString("*i")
			case "V", "VERSE":
				sb.WriteString("*v")
			default:
				sb.WriteString("*A")
			}
		}

		// Mid-song time-signature change.
		if m.TimeSig != nil {
			sb.WriteString(fmt.Sprintf("T%d%d", m.TimeSig.Num, m.TimeSig.Den))
		}

		// Start-repeat barline.
		if m.StartRepeat {
			sb.WriteString("[")
		}

		// Chord symbols for this measure.
		if len(m.Chords) == 0 {
			// No chord symbol: indicate "no change" with a space.
			sb.WriteString("x ")
		} else {
			for _, c := range m.Chords {
				chord := tpcToNote(c.Root) + qualityToIReal(c.Quality)
				if c.Bass != 0 {
					chord += "/" + tpcToNote(c.Bass)
				}
				sb.WriteString(chord)
				sb.WriteString(" ")
			}
		}

		// Closing barline.
		isLast := i == len(song.Measures)-1
		switch {
		case m.EndRepeat:
			sb.WriteString("]")
		case isLast:
			sb.WriteString("Z")
		default:
			sb.WriteString("|")
		}
	}

	return sb.String()
}

// ──────────────────────────────────────────────────────────────────────────────
// iRealPro scrambling
// ──────────────────────────────────────────────────────────────────────────────

// scramble applies (or reverses) the iRealPro character-level obfuscation.
// The operation is self-inverse: scramble(scramble(s)) == s.
// It divides the byte slice into 50-byte blocks and swaps the pairs at
// positions (1,2), (24,25), and (46,47) within each full block.
func scramble(s string) string {
	b := []byte(s)
	for i := 0; i+50 <= len(b); i += 50 {
		b[i+1], b[i+2] = b[i+2], b[i+1]
		b[i+24], b[i+25] = b[i+25], b[i+24]
		b[i+46], b[i+47] = b[i+47], b[i+46]
	}
	return string(b)
}

// ──────────────────────────────────────────────────────────────────────────────
// URL / HTML building
// ──────────────────────────────────────────────────────────────────────────────

// percentEncode percent-encodes every byte that is not an unreserved URI
// character (letters, digits, '-', '_', '.', '~').
func percentEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// buildIRealURL builds the full irealbook:// URL for one or more songs.
func buildIRealURL(songs []songData) string {
	parts := make([]string, 0, len(songs))
	for _, s := range songs {
		body := buildIRealBody(&s)
		body = "{" + body + "}"
		body = scramble(body)

		// Song format: Title=Composer=Style=Key=n=Body
		part := strings.Join([]string{
			percentEncode(s.Title),
			percentEncode(s.Composer),
			percentEncode(s.Style),
			percentEncode(s.Key),
			"n",
			percentEncode(body),
		}, "=")
		parts = append(parts, part)
	}
	return "irealbook://" + strings.Join(parts, "===")
}

// generateHTML produces the HTML import file that iRealPro recognises.
func generateHTML(songs []songData) string {
	iRealURL := buildIRealURL(songs)

	title := songs[0].Title
	if len(songs) > 1 {
		title = fmt.Sprintf("%d Songs", len(songs))
	}

	// The URL is already percent-encoded so it is safe as an HTML attribute
	// value.  We still HTML-escape the title for display.
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta http-equiv="refresh" content="0; url=%s">
  <title>iRealPro – %s</title>
</head>
<body>
<p>Opening in iRealPro…</p>
<p>If iRealPro does not open automatically,
<a href="%s">click here to import</a>.</p>
</body>
</html>
`, iRealURL, html.EscapeString(title), iRealURL)
}

// ──────────────────────────────────────────────────────────────────────────────
// MuseScore parsing
// ──────────────────────────────────────────────────────────────────────────────

// parseMSCZ opens a .mscz archive, extracts the embedded .mscx file, and
// parses it.
func parseMSCZ(path string) (*songData, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".mscx") {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open %s inside archive: %w", f.Name, err)
			}
			song, err := parseMSCXReader(rc)
			rc.Close()
			return song, err
		}
	}
	return nil, fmt.Errorf("no .mscx file found inside %s", path)
}

// parseMSCXFile opens a plain .mscx file and parses it.
func parseMSCXFile(path string) (*songData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseMSCXReader(f)
}

// parseMSCXReader decodes a MuseScore XML stream and returns a songData.
func parseMSCXReader(r io.Reader) (*songData, error) {
	var doc museScoreDoc
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("XML decode: %w", err)
	}

	song := &songData{Style: "Jazz"}

	// ── Metadata ──────────────────────────────────────────────────────────
	for _, mt := range doc.Score.MetaTags {
		v := strings.TrimSpace(mt.Value)
		if v == "" {
			continue
		}
		switch mt.Name {
		case "title", "workTitle":
			if song.Title == "" {
				song.Title = v
			}
		case "composer", "arranger":
			if song.Composer == "" {
				song.Composer = v
			}
		}
	}
	if song.Title == "" {
		song.Title = "Untitled"
	}
	if song.Composer == "" {
		song.Composer = "Unknown"
	}

	// ── Staff ─────────────────────────────────────────────────────────────
	if len(doc.Score.Staves) == 0 {
		return nil, fmt.Errorf("no staves found in score")
	}
	staff := doc.Score.Staves[0]

	// ── Walk measures ─────────────────────────────────────────────────────
	keySig := 0
	keySigFound := false
	defaultTS := timeSigData{4, 4}
	defaultTSFound := false

	for _, mx := range staff.Measures {
		md := measureData{}

		// Repeat markers at the measure level.
		if mx.StartRepeat != nil {
			md.StartRepeat = true
		}
		if mx.EndRepeat != nil {
			md.EndRepeat = true
		}

		// Key/time sig and rehearsal marks at the measure level (MuseScore 4
		// sometimes places them here rather than inside a <voice>).
		for _, ks := range mx.KeySigs {
			if !keySigFound {
				keySig = ks.Accidental
				keySigFound = true
			}
		}
		for _, ts := range mx.TimeSigs {
			if ts.SigN <= 0 || ts.SigD <= 0 {
				continue
			}
			if !defaultTSFound {
				defaultTS = timeSigData{ts.SigN, ts.SigD}
				defaultTSFound = true
			} else {
				t := timeSigData{ts.SigN, ts.SigD}
				md.TimeSig = &t
			}
		}
		for _, rm := range mx.RehearsalMarks {
			if rm.Text != "" && md.RehearsalMark == "" {
				md.RehearsalMark = rm.Text
			}
		}

		for _, v := range mx.Voices {
			// BarLine-based repeat markers (alternative encoding).
			for _, bl := range v.BarLines {
				switch bl.Subtype {
				case "start-repeat":
					md.StartRepeat = true
				case "end-repeat", "end-start-repeat":
					md.EndRepeat = true
				}
			}

			// Time signature.
			for _, ts := range v.TimeSigs {
				if ts.SigN <= 0 || ts.SigD <= 0 {
					continue
				}
				if !defaultTSFound {
					defaultTS = timeSigData{ts.SigN, ts.SigD}
					defaultTSFound = true
				} else {
					t := timeSigData{ts.SigN, ts.SigD}
					md.TimeSig = &t
				}
			}

			// Key signature (only the first one determines the song key).
			for _, ks := range v.KeySigs {
				if !keySigFound {
					keySig = ks.Accidental
					keySigFound = true
				}
			}

			// Rehearsal marks.
			for _, rm := range v.RehearsalMarks {
				if rm.Text != "" && md.RehearsalMark == "" {
					md.RehearsalMark = rm.Text
				}
			}

			// Chord symbols (Harmony elements).
			for _, h := range v.Harmonies {
				// TPC valid range for practical use: 6 (Fb) … 26 (B#).
				if h.Root < 6 || h.Root > 26 {
					continue
				}
				cd := chordData{Root: h.Root, Quality: h.Name}
				if h.Bass >= 6 && h.Bass <= 26 {
					cd.Bass = h.Bass
				}
				md.Chords = append(md.Chords, cd)
			}
		}

		song.Measures = append(song.Measures, md)
	}

	// ── Derive song key ───────────────────────────────────────────────────
	if name, ok := keySigNames[keySig]; ok {
		song.Key = name
	} else {
		song.Key = "C"
	}
	song.DefaultTS = defaultTS

	return song, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func defaultOutputPath(inputPath string) string {
	ext := filepath.Ext(inputPath)
	return inputPath[:len(inputPath)-len(ext)] + ".html"
}

// ──────────────────────────────────────────────────────────────────────────────
// main
// ──────────────────────────────────────────────────────────────────────────────

func main() {
	// Parse arguments manually so that -o may appear anywhere in the list,
	// including after positional arguments (e.g. "input.mscz -o out.html").
	var outputFlag string
	var inputFiles []string
	rawArgs := os.Args[1:]
	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		switch {
		case arg == "-o":
			if i+1 < len(rawArgs) {
				i++
				outputFlag = rawArgs[i]
			} else {
				fmt.Fprintln(os.Stderr, "flag -o requires an argument")
				os.Exit(1)
			}
		case strings.HasPrefix(arg, "-o="):
			outputFlag = arg[3:]
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(os.Stderr, "Usage: muserealpro [-o output.html] input.mscz [input2.mscz ...]")
			fmt.Fprintln(os.Stderr, "Converts MuseScore 4 files to iRealPro chord charts (HTML format).")
			os.Exit(0)
		default:
			inputFiles = append(inputFiles, arg)
		}
	}

	if len(inputFiles) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: muserealpro [-o output.html] input.mscz [input2.mscz ...]")
		fmt.Fprintln(os.Stderr, "Converts MuseScore 4 files to iRealPro chord charts (HTML format).")
		os.Exit(1)
	}

	// Parse every input file.
	var songs []songData
	for _, inputPath := range inputFiles {
		var (
			song *songData
			err  error
		)
		switch strings.ToLower(filepath.Ext(inputPath)) {
		case ".mscz":
			song, err = parseMSCZ(inputPath)
		case ".mscx":
			song, err = parseMSCXFile(inputPath)
		default:
			fmt.Fprintf(os.Stderr, "Unsupported file type: %s (expected .mscz or .mscx)\n", inputPath)
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", inputPath, err)
			continue
		}
		songs = append(songs, *song)
		fmt.Printf("Parsed: %s  (by %s, key %s)\n", song.Title, song.Composer, song.Key)
	}
	if len(songs) == 0 {
		fmt.Fprintln(os.Stderr, "No songs were successfully parsed.")
		os.Exit(1)
	}

	// Determine the output path.
	outputPath := outputFlag
	if outputPath == "" {
		defaultPath := defaultOutputPath(inputFiles[0])
		// On Windows (single-file "Open With" scenario) show a save dialog;
		// on other platforms just use the default path beside the input file.
		var ok bool
		outputPath, ok = getSaveFilePath(defaultPath, len(inputFiles) == 1)
		if !ok {
			// User cancelled the dialog.
			fmt.Fprintln(os.Stderr, "Save cancelled.")
			os.Exit(0)
		}
	}

	// Generate and write the HTML file.
	htmlContent := generateHTML(songs)
	if err := os.WriteFile(outputPath, []byte(htmlContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputPath, err)
		os.Exit(1)
	}
	fmt.Printf("Saved: %s\n", outputPath)
}
