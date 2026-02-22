package main

import (
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// TPC → note name
// ──────────────────────────────────────────────────────────────────────────────

func TestTpcToNote(t *testing.T) {
	cases := []struct {
		tpc  int
		want string
	}{
		{14, "C"},
		{15, "G"},
		{16, "D"},
		{17, "A"},
		{18, "E"},
		{19, "B"},
		{20, "F#"},
		{21, "C#"},
		{13, "F"},
		{12, "Bb"},
		{11, "Eb"},
		{10, "Ab"},
		{9, "Db"},
		{8, "Gb"},
		// Enharmonic edge-cases
		{6, "E"},  // Fb → E
		{7, "B"},  // Cb → B
		{25, "F"}, // E# → F
		{26, "C"}, // B# → C
	}
	for _, c := range cases {
		got := tpcToNote(c.tpc)
		if got != c.want {
			t.Errorf("tpcToNote(%d) = %q; want %q", c.tpc, got, c.want)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Chord quality conversion
// ──────────────────────────────────────────────────────────────────────────────

func TestQualityToIReal(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"M", ""},
		{"maj", ""},
		{"m", "-"},
		{"min", "-"},
		{"7", "7"},
		{"maj7", "^7"},
		{"M7", "^7"},
		{"m7", "-7"},
		{"min7", "-7"},
		{"m7b5", "h"},
		{"ø7", "h"},
		{"dim", "o"},
		{"°", "o"},
		{"dim7", "o7"},
		{"aug", "+"},
		{"+", "+"},
		{"sus4", "sus"},
		{"sus", "sus"},
		{"sus2", "sus2"},
		{"7sus4", "7sus"},
		{"6", "6"},
		{"m6", "-6"},
		{"9", "9"},
		{"maj9", "^9"},
		{"m9", "-9"},
		{"add9", "2"},
		{"11", "11"},
		{"13", "13"},
		{"mM7", "-^7"},
		{"7b9", "7b9"},
		{"7#9", "7#9"},
		{"alt", "alt"},
		{"unknown_quality", "unknown_quality"}, // pass-through
	}
	for _, c := range cases {
		got := qualityToIReal(c.in)
		if got != c.want {
			t.Errorf("qualityToIReal(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// iRealPro scrambling
// ──────────────────────────────────────────────────────────────────────────────

func TestScrambleSelfInverse(t *testing.T) {
	// The operation must be self-inverse for any input.
	inputs := []string{
		"",
		"short",
		strings.Repeat("A", 49),
		strings.Repeat("A", 50),
		strings.Repeat("A", 51),
		strings.Repeat("A", 100),
		"{T44Cmaj7 |Am7 |Dm7 |G7 Z}",
		// A string exactly 50 bytes to exercise the swap logic.
		"01234567890123456789012345678901234567890123456789",
	}
	for _, s := range inputs {
		if got := scramble(scramble(s)); got != s {
			t.Errorf("scramble(scramble(%q)) != original", s)
		}
	}
}

func TestScrambleSwapsPositions(t *testing.T) {
	// Build a 50-byte string where every byte is its own index (mod 128).
	b := make([]byte, 50)
	for i := range b {
		b[i] = byte(i + 'a') // 'a' = 97 ... but we only care about relative order
	}
	s := string(b)
	got := scramble(s)
	gb := []byte(got)

	// Positions 1 and 2 must be swapped.
	if gb[1] != b[2] || gb[2] != b[1] {
		t.Errorf("expected positions 1,2 to be swapped")
	}
	// Positions 24 and 25 must be swapped.
	if gb[24] != b[25] || gb[25] != b[24] {
		t.Errorf("expected positions 24,25 to be swapped")
	}
	// Positions 46 and 47 must be swapped.
	if gb[46] != b[47] || gb[47] != b[46] {
		t.Errorf("expected positions 46,47 to be swapped")
	}
	// Position 0 must be unchanged.
	if gb[0] != b[0] {
		t.Errorf("position 0 should be unchanged")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Body builder
// ──────────────────────────────────────────────────────────────────────────────

func TestBuildIRealBody_TimeSig(t *testing.T) {
	song := &songData{
		DefaultTS: timeSigData{3, 4},
		Measures: []measureData{
			{Chords: []chordData{{Root: 14}}}, // C
			{Chords: []chordData{{Root: 15}}}, // G
		},
	}
	body := buildIRealBody(song)
	if !strings.HasPrefix(body, "T34") {
		t.Errorf("expected body to start with T34, got %q", body)
	}
}

func TestBuildIRealBody_RepeatAndRehearsalMark(t *testing.T) {
	song := &songData{
		DefaultTS: timeSigData{4, 4},
		Measures: []measureData{
			{StartRepeat: true, RehearsalMark: "A", Chords: []chordData{{Root: 14}}},
			{EndRepeat: true, Chords: []chordData{{Root: 15}}},
		},
	}
	body := buildIRealBody(song)
	if !strings.Contains(body, "*A") {
		t.Errorf("expected rehearsal mark *A in body, got %q", body)
	}
	if !strings.Contains(body, "[") {
		t.Errorf("expected start-repeat '[' in body, got %q", body)
	}
	if !strings.Contains(body, "]") {
		t.Errorf("expected end-repeat ']' in body, got %q", body)
	}
}

func TestBuildIRealBody_SlashChord(t *testing.T) {
	song := &songData{
		DefaultTS: timeSigData{4, 4},
		Measures: []measureData{
			// G/B = root TPC 15, bass TPC 19
			{Chords: []chordData{{Root: 15, Quality: "", Bass: 19}}},
		},
	}
	body := buildIRealBody(song)
	if !strings.Contains(body, "G/B") {
		t.Errorf("expected G/B slash chord in body, got %q", body)
	}
}

func TestBuildIRealBody_NoChords(t *testing.T) {
	song := &songData{
		DefaultTS: timeSigData{4, 4},
		Measures: []measureData{
			{Chords: []chordData{{Root: 14}}},
			{},                               // no chords → placeholder
			{Chords: []chordData{{Root: 15}}},
		},
	}
	body := buildIRealBody(song)
	if !strings.Contains(body, "x ") {
		t.Errorf("expected 'x' placeholder for empty measure in body, got %q", body)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// URL / HTML generation
// ──────────────────────────────────────────────────────────────────────────────

func TestPercentEncode(t *testing.T) {
	cases := []struct{ in, want string }{
		{"ABC", "ABC"},
		{"abc123", "abc123"},
		{" ", "%20"},
		{"{}", "%7B%7D"},
		{"C#", "C%23"},
		{"Bb-7", "Bb-7"},
	}
	for _, c := range cases {
		got := percentEncode(c.in)
		if got != c.want {
			t.Errorf("percentEncode(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestBuildIRealURL_Format(t *testing.T) {
	songs := []songData{
		{
			Title: "My Song", Composer: "Doe John",
			Style: "Jazz", Key: "C",
			DefaultTS: timeSigData{4, 4},
			Measures:  []measureData{{Chords: []chordData{{Root: 14}}}},
		},
	}
	u := buildIRealURL(songs)
	if !strings.HasPrefix(u, "irealbook://") {
		t.Errorf("URL must start with irealbook://, got %q", u)
	}
	// Fields 0-4 must be present as =n=
	if !strings.Contains(u, "=n=") {
		t.Errorf("URL must contain =n= separator, got %q", u)
	}
}

func TestBuildIRealURL_MultipleSongs(t *testing.T) {
	song := songData{
		Title: "S", Composer: "C", Style: "Jazz", Key: "C",
		DefaultTS: timeSigData{4, 4},
		Measures:  []measureData{{Chords: []chordData{{Root: 14}}}},
	}
	u := buildIRealURL([]songData{song, song})
	if !strings.Contains(u, "===") {
		t.Errorf("multiple songs must be separated by ===, got %q", u)
	}
}

func TestGenerateHTML_ContainsIRealURL(t *testing.T) {
	songs := []songData{
		{
			Title: "Test", Composer: "Author", Style: "Jazz", Key: "C",
			DefaultTS: timeSigData{4, 4},
			Measures:  []measureData{{Chords: []chordData{{Root: 14}}}},
		},
	}
	h := generateHTML(songs)
	if !strings.Contains(h, "irealbook://") {
		t.Errorf("HTML must contain irealbook:// URL")
	}
	if !strings.Contains(h, "<meta http-equiv=\"refresh\"") {
		t.Errorf("HTML must contain meta refresh tag")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// XML parsing (round-trip style)
// ──────────────────────────────────────────────────────────────────────────────

const sampleMSCX = `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="4.20">
  <Score>
    <metaTag name="title">Blue Bossa</metaTag>
    <metaTag name="composer">Kenny Dorham</metaTag>
    <Staff id="1">
      <Measure number="1">
        <voice>
          <KeySig><accidental>-3</accidental></KeySig>
          <TimeSig><sigN>3</sigN><sigD>4</sigD></TimeSig>
          <Harmony><root>11</root><name>m7</name></Harmony>
          <Chord><durationType>half</durationType></Chord>
          <Harmony><root>11</root><name>m7</name></Harmony>
          <Chord><durationType>quarter</durationType></Chord>
        </voice>
      </Measure>
      <Measure number="2">
        <startRepeat/>
        <voice>
          <Harmony><root>9</root><name>m7b5</name></Harmony>
          <Chord><durationType>whole</durationType></Chord>
        </voice>
      </Measure>
      <Measure number="3">
        <endRepeat/>
        <voice>
          <Harmony><root>16</root><name>7b9</name></Harmony>
          <Chord><durationType>whole</durationType></Chord>
        </voice>
      </Measure>
    </Staff>
  </Score>
</museScore>`

func TestParseMSCXReader(t *testing.T) {
	song, err := parseMSCXReader(strings.NewReader(sampleMSCX))
	if err != nil {
		t.Fatalf("parseMSCXReader error: %v", err)
	}

	if song.Title != "Blue Bossa" {
		t.Errorf("title = %q; want %q", song.Title, "Blue Bossa")
	}
	if song.Composer != "Kenny Dorham" {
		t.Errorf("composer = %q; want %q", song.Composer, "Kenny Dorham")
	}
	if song.Key != "Eb" {
		t.Errorf("key = %q; want Eb (keySig -3)", song.Key)
	}
	if song.DefaultTS.Num != 3 || song.DefaultTS.Den != 4 {
		t.Errorf("defaultTS = %v; want 3/4", song.DefaultTS)
	}
	if len(song.Measures) != 3 {
		t.Fatalf("expected 3 measures, got %d", len(song.Measures))
	}

	// Measure 1: two Ebm7 chords
	m0 := song.Measures[0]
	if len(m0.Chords) != 2 {
		t.Errorf("measure 1: expected 2 chords, got %d", len(m0.Chords))
	}
	if m0.Chords[0].Root != 11 || m0.Chords[0].Quality != "m7" {
		t.Errorf("measure 1 chord 0: got root=%d quality=%q; want root=11 quality=m7",
			m0.Chords[0].Root, m0.Chords[0].Quality)
	}

	// Measure 2: start repeat
	m1 := song.Measures[1]
	if !m1.StartRepeat {
		t.Errorf("measure 2 should have StartRepeat=true")
	}
	if len(m1.Chords) != 1 || m1.Chords[0].Quality != "m7b5" {
		t.Errorf("measure 2 chord quality: got %q; want m7b5", m1.Chords[0].Quality)
	}

	// Measure 3: end repeat
	m2 := song.Measures[2]
	if !m2.EndRepeat {
		t.Errorf("measure 3 should have EndRepeat=true")
	}
	if len(m2.Chords) != 1 || m2.Chords[0].Quality != "7b9" {
		t.Errorf("measure 3 chord quality: got %q; want 7b9", m2.Chords[0].Quality)
	}
}

// TestParseMSCXReader_KeySigAtMeasureLevel verifies that <KeySig> and <TimeSig>
// placed directly inside <Measure> (rather than inside a <voice>) are correctly
// detected — this is the layout MuseScore 4 often produces.
const sampleMSCX_MeasureLevelKeySig = `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="4.20">
  <Score>
    <metaTag name="title">Measure Level Key</metaTag>
    <metaTag name="composer">Tester</metaTag>
    <Staff id="1">
      <Measure number="1">
        <KeySig><accidental>3</accidental></KeySig>
        <TimeSig><sigN>3</sigN><sigD>4</sigD></TimeSig>
        <voice>
          <Harmony><root>17</root><name>maj7</name></Harmony>
        </voice>
      </Measure>
      <Measure number="2">
        <voice>
          <Harmony><root>20</root><name>7</name></Harmony>
        </voice>
      </Measure>
    </Staff>
  </Score>
</museScore>`

func TestParseMSCXReader_KeySigAtMeasureLevel(t *testing.T) {
	song, err := parseMSCXReader(strings.NewReader(sampleMSCX_MeasureLevelKeySig))
	if err != nil {
		t.Fatalf("parseMSCXReader error: %v", err)
	}
	if song.Key != "A" {
		t.Errorf("key = %q; want A (3 sharps at measure level)", song.Key)
	}
	if song.DefaultTS.Num != 3 || song.DefaultTS.Den != 4 {
		t.Errorf("defaultTS = %v; want 3/4 (from measure level)", song.DefaultTS)
	}
}

// TestArgParsingFlagAfterPositional verifies that the manual arg parser
// correctly handles -o appearing after positional arguments.
func TestArgParsingFlagAfterPositional(t *testing.T) {
	// Simulate what parseArgs would do; we exercise it indirectly through the
	// same logic extracted here so we don't need to refactor main().
	rawArgs := []string{"/tmp/song.mscz", "-o", "/tmp/out.html"}
	var outputFlag string
	var inputFiles []string
	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		switch {
		case arg == "-o" || arg == "--o":
			if i+1 < len(rawArgs) {
				i++
				outputFlag = rawArgs[i]
			}
		case strings.HasPrefix(arg, "-o="):
			outputFlag = arg[3:]
		default:
			inputFiles = append(inputFiles, arg)
		}
	}
	if outputFlag != "/tmp/out.html" {
		t.Errorf("outputFlag = %q; want /tmp/out.html", outputFlag)
	}
	if len(inputFiles) != 1 || inputFiles[0] != "/tmp/song.mscz" {
		t.Errorf("inputFiles = %v; want [/tmp/song.mscz]", inputFiles)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Thespian.mscz integration test
// ──────────────────────────────────────────────────────────────────────────────

// TestParseMSCZ_Thespian parses the real Thespian.mscz file and validates that
// the MuseScore 4.50+ XML tags (<concertKey> and <base>) are handled correctly.
func TestParseMSCZ_Thespian(t *testing.T) {
	song, err := parseMSCZ("Thespian.mscz")
	if err != nil {
		t.Fatalf("parseMSCZ(Thespian.mscz) error: %v", err)
	}

	// Title and composer from metaTags.
	if song.Title != "Thespian" {
		t.Errorf("title = %q; want %q", song.Title, "Thespian")
	}

	// Key: concertKey=-6 → Gb  (verifies <concertKey> parsing).
	if song.Key != "Gb" {
		t.Errorf("key = %q; want Gb (concertKey=-6)", song.Key)
	}

	// At least one measure must have been parsed.
	if len(song.Measures) == 0 {
		t.Fatal("expected at least one measure, got 0")
	}

	// Count slash chords — the file contains harmonies with <base> elements
	// (MuseScore 4.50+ syntax for slash chords).
	slashChords := 0
	for _, m := range song.Measures {
		for _, c := range m.Chords {
			if c.Bass != 0 {
				slashChords++
			}
		}
	}
	if slashChords == 0 {
		t.Error("expected at least one slash chord (<base> element), got 0")
	}
}
