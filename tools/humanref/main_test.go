package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// writeXLSX builds the smallest workbook readDays understands: shared
// strings plus one sheet laid out like the COMAP file (headers in row 2).
func writeXLSX(t *testing.T, sheet string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.xlsx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		"xl/sharedStrings.xml":     `<sst><si><t>Word</t></si><si><t>crane</t></si><si><t xml:space="preserve">clen </t></si><si><t>nymph</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData>` + sheet + `</sheetData></worksheet>`,
	} {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	f.Close()
	return path
}

func row(r, word string, reported string, pcts ...string) string {
	s := `<row r="` + r + `"><c r="D` + r + `" t="s"><v>` + word + `</v></c><c r="E` + r + `"><v>` + reported + `</v></c>`
	for i, p := range pcts {
		s += `<c r="` + string(rune('G'+i)) + r + `"><v>` + p + `</v></c>`
	}
	return s + `</row>`
}

func TestReadDays(t *testing.T) {
	path := writeXLSX(t,
		`<row r="2"><c r="D2" t="s"><v>0</v></c></row>`+
			row("3", "1", "1000", "0", "5", "20", "35", "25", "10", "5")+ // crane, sums to 100
			row("4", "2", "900", "0", "5", "20", "35", "25", "10", "5")+ // "clen ": a typo, not on the answer list
			row("5", "3", "800", "1", "2", "3", "4", "5", "6", "7")) // nymph: sums to 28
	days, skipped, err := readDays(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 || days[0].word != "crane" || days[0].reported != 1000 {
		t.Fatalf("days = %+v, want just crane with 1000 reports", days)
	}
	if want := [7]float64{0, 5, 20, 35, 25, 10, 5}; days[0].pct != want {
		t.Errorf("pct = %v, want %v", days[0].pct, want)
	}
	if len(skipped["not on the answer list"]) != 1 || len(skipped["percentages sum outside 98-102"]) != 1 {
		t.Errorf("skipped = %v", skipped)
	}
}
