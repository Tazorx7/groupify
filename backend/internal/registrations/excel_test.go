package registrations

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func sampleRows() []AdminRegistration {
	at := time.Date(2026, 11, 11, 9, 14, 2, 0, time.UTC)
	return []AdminRegistration{
		{ID: 1, FullName: "Ama Mensah", IndexNumber: "00123456", TopicName: "Distributed Systems", CreatedAt: at},
		{ID: 2, FullName: "Kofi Boateng", IndexNumber: "CS/2024/045", TopicName: "Compilers", CreatedAt: at.Add(time.Hour)},
		{ID: 3, FullName: "Yaa Asantewaa", IndexNumber: "A-100", TopicName: "Compilers", CreatedAt: at.Add(2 * time.Hour)},
	}
}

func openWorkbook(t *testing.T, data []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open generated workbook: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// The export exists because Excel eats leading zeroes. Prove it does not.
func TestXLSXPreservesLeadingZeroes(t *testing.T) {
	data, err := writeXLSX(sampleRows())
	if err != nil {
		t.Fatalf("writeXLSX: %v", err)
	}

	f := openWorkbook(t, data)

	got, err := f.GetCellValue(xlsxSheetName, "A2")
	if err != nil {
		t.Fatalf("read A2: %v", err)
	}
	if got != "00123456" {
		t.Fatalf("A2 = %q, want %q", got, "00123456")
	}

	// The value surviving is not enough: the cell must be *typed* as text so it
	// stays intact when the file is edited and saved again.
	styleID, err := f.GetCellStyle(xlsxSheetName, "A2")
	if err != nil {
		t.Fatalf("read style: %v", err)
	}
	style, err := f.GetStyle(styleID)
	if err != nil {
		t.Fatalf("resolve style: %v", err)
	}
	if style.NumFmt != numFmtText && style.CustomNumFmt == nil {
		t.Fatalf("A2 is not text-typed: numFmt=%d custom=%v", style.NumFmt, style.CustomNumFmt)
	}
}

func TestXLSXHeaderAndRows(t *testing.T) {
	rows := sampleRows()
	data, err := writeXLSX(rows)
	if err != nil {
		t.Fatalf("writeXLSX: %v", err)
	}

	f := openWorkbook(t, data)

	if sheets := f.GetSheetList(); len(sheets) != 1 || sheets[0] != xlsxSheetName {
		t.Fatalf("sheets = %v, want [%s]", sheets, xlsxSheetName)
	}

	all, err := f.GetRows(xlsxSheetName)
	if err != nil {
		t.Fatalf("get rows: %v", err)
	}
	if len(all) != len(rows)+1 {
		t.Fatalf("rows = %d, want %d (data + header)", len(all), len(rows)+1)
	}

	wantHeader := []string{"index_number", "full_name", "topic", "registered_at"}
	for i, want := range wantHeader {
		if all[0][i] != want {
			t.Fatalf("header[%d] = %q, want %q", i, all[0][i], want)
		}
	}

	// Order is preserved exactly as supplied by the service.
	if all[1][1] != "Ama Mensah" || all[2][1] != "Kofi Boateng" || all[3][1] != "Yaa Asantewaa" {
		t.Fatalf("row order not preserved: %v", all[1:])
	}
	if all[2][0] != "CS/2024/045" {
		t.Fatalf("index with slashes mangled: %q", all[2][0])
	}

	// Timestamp is a real date value, not a string blob.
	got, err := f.GetCellValue(xlsxSheetName, "D2")
	if err != nil {
		t.Fatalf("read D2: %v", err)
	}
	if !strings.HasPrefix(got, "2026-11-11") {
		t.Fatalf("D2 = %q, want a 2026-11-11 timestamp", got)
	}
}

func TestXLSXEmptyExportStillHasHeader(t *testing.T) {
	data, err := writeXLSX(nil)
	if err != nil {
		t.Fatalf("writeXLSX: %v", err)
	}

	f := openWorkbook(t, data)
	all, err := f.GetRows(xlsxSheetName)
	if err != nil {
		t.Fatalf("get rows: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("rows = %d, want 1 header row", len(all))
	}
}

func TestXLSXFilename(t *testing.T) {
	at := time.Date(2026, 11, 11, 9, 14, 2, 0, time.UTC)
	if got := ExcelExportFilename(at); got != "groupify-registrations-20261111-091402.xlsx" {
		t.Fatalf("filename = %q", got)
	}
}

func TestWriteCSVPreservesIdentifiers(t *testing.T) {
	data := writeCSV(sampleRows())

	if !bytes.HasPrefix(data, []byte("\xEF\xBB\xBF")) {
		t.Fatal("missing UTF-8 BOM")
	}

	records, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF")))).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("records = %d, want 4", len(records))
	}
	if records[1][0] != "00123456" {
		t.Fatalf("csv index = %q, want leading zeroes preserved in the file", records[1][0])
	}
}

func TestSanitiseCSVNeutralisesFormulas(t *testing.T) {
	cases := map[string]string{
		"=cmd|' /C calc'!A0": "'=cmd|' /C calc'!A0",
		"+1234":              "'+1234",
		"-1234":              "'-1234",
		"@SUM(A1)":           "'@SUM(A1)",
		"Ama Mensah":         "Ama Mensah",
		"":                   "",
	}
	for in, want := range cases {
		if got := sanitiseCSV(in); got != want {
			t.Fatalf("sanitiseCSV(%q) = %q, want %q", in, got, want)
		}
	}
}
