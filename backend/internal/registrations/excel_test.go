package registrations

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func sampleExport() *ExportData {
	at := time.Date(2026, 11, 11, 9, 14, 2, 0, time.UTC)
	return &ExportData{
		Rows: []AdminRegistration{
			{ID: 1, FullName: "Ama Mensah", IndexNumber: "00123456", TopicName: "Compilers", CreatedAt: at},
			{ID: 2, FullName: "Kofi Boateng", IndexNumber: "CS/2024/045", TopicName: "Compilers", CreatedAt: at.Add(time.Hour)},
			{ID: 3, FullName: "Yaa Asantewaa", IndexNumber: "A-100", TopicName: "Distributed Systems", CreatedAt: at.Add(2 * time.Hour)},
		},
		Topics: []TopicSummary{
			{ID: 1, Name: "Compilers", Capacity: 10, Members: 2, IsActive: true},
			{ID: 2, Name: "Distributed Systems", Capacity: 30, Members: 1, IsActive: true},
			{ID: 3, Name: "Full Topic", Capacity: 2, Members: 2, IsActive: true},
			{ID: 4, Name: "Inactive Topic", Capacity: 5, Members: 0, IsActive: false},
			{ID: 5, Name: "Zero Picks", Capacity: 8, Members: 0, IsActive: true},
		},
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
	data, err := writeXLSX(sampleExport())
	if err != nil {
		t.Fatalf("writeXLSX: %v", err)
	}

	f := openWorkbook(t, data)

	got, err := f.GetCellValue(xlsxRosterSheet, "B2")
	if err != nil {
		t.Fatalf("read B2: %v", err)
	}
	if got != "00123456" {
		t.Fatalf("B2 = %q, want %q", got, "00123456")
	}

	// The value surviving is not enough: the cell must be *typed* as text so it
	// stays intact when the file is edited and saved again.
	styleID, err := f.GetCellStyle(xlsxRosterSheet, "B2")
	if err != nil {
		t.Fatalf("read style: %v", err)
	}
	style, err := f.GetStyle(styleID)
	if err != nil {
		t.Fatalf("resolve style: %v", err)
	}
	if style.NumFmt != numFmtText && style.CustomNumFmt == nil {
		t.Fatalf("B2 is not text-typed: numFmt=%d custom=%v", style.NumFmt, style.CustomNumFmt)
	}
}

func TestXLSXHasSummaryGroupsAndRosterSheets(t *testing.T) {
	data, err := writeXLSX(sampleExport())
	if err != nil {
		t.Fatalf("writeXLSX: %v", err)
	}

	f := openWorkbook(t, data)

	sheets := f.GetSheetList()
	want := []string{xlsxSummarySheet, xlsxGroupsSheet, xlsxRosterSheet}
	if len(sheets) != len(want) {
		t.Fatalf("sheets = %v, want %v", sheets, want)
	}
	for i := range want {
		if sheets[i] != want[i] {
			t.Fatalf("sheets = %v, want %v", sheets, want)
		}
	}
}

// The Groups sheet answers "group 1 is these people" directly: a band per
// topic followed by its students, with occupancy in the band.
func TestXLSXGroupsSheetBandsEachTopic(t *testing.T) {
	data, err := writeXLSX(sampleExport())
	if err != nil {
		t.Fatalf("writeXLSX: %v", err)
	}

	f := openWorkbook(t, data)
	all, err := f.GetRows(xlsxGroupsSheet)
	if err != nil {
		t.Fatalf("get groups rows: %v", err)
	}

	// Drop spacer rows so the assertions are about content, not layout gaps.
	rows := make([][]string, 0, len(all))
	for _, row := range all {
		if len(row) > 0 && row[0] != "" {
			rows = append(rows, row)
		}
	}

	// header, 2 bands, 3 members.
	if len(rows) != 6 {
		t.Fatalf("groups rows = %d, want 6: %v", len(rows), rows)
	}
	if rows[0][0] != "Full name" || rows[0][1] != "Index number" || rows[0][2] != "Registered at" {
		t.Fatalf("groups header = %v", rows[0])
	}

	wantBand1 := "GROUP 1 · Compilers — 2/10 filled · 8 places left"
	if rows[1][0] != wantBand1 {
		t.Fatalf("band 1 = %q, want %q", rows[1][0], wantBand1)
	}
	// Members sit under their band, in name order.
	if rows[2][0] != "Ama Mensah" || rows[2][1] != "00123456" {
		t.Fatalf("member 1 = %v", rows[2])
	}
	if rows[3][0] != "Kofi Boateng" || rows[3][1] != "CS/2024/045" {
		t.Fatalf("member 2 = %v", rows[3])
	}

	wantBand2 := "GROUP 2 · Distributed Systems — 1/30 filled · 29 places left"
	if rows[4][0] != wantBand2 {
		t.Fatalf("band 2 = %q, want %q", rows[4][0], wantBand2)
	}
	if rows[5][0] != "Yaa Asantewaa" {
		t.Fatalf("member 3 = %v", rows[5])
	}

	// Each band spans the three data columns (rows 2 and 6: band, four rows,
	// spacer, band).
	merges, err := f.GetMergeCells(xlsxGroupsSheet)
	if err != nil {
		t.Fatalf("get merges: %v", err)
	}
	if len(merges) != 2 {
		t.Fatalf("merges = %d, want 2", len(merges))
	}
	for i, want := range []string{"A2", "A6"} {
		if merges[i].GetStartAxis() != want || merges[i].GetEndAxis() != strings.Replace(want, "A", "C", 1) {
			t.Fatalf("merge %d = %s:%s", i, merges[i].GetStartAxis(), merges[i].GetEndAxis())
		}
	}
}

func TestXLSXRosterHeaderAndRows(t *testing.T) {
	rows := sampleExport().Rows
	data, err := writeXLSX(sampleExport())
	if err != nil {
		t.Fatalf("writeXLSX: %v", err)
	}

	f := openWorkbook(t, data)
	all, err := f.GetRows(xlsxRosterSheet)
	if err != nil {
		t.Fatalf("get rows: %v", err)
	}
	if len(all) != len(rows)+1 {
		t.Fatalf("rows = %d, want %d (data + header)", len(all), len(rows)+1)
	}

	wantHeader := []string{"Topic", "Index number", "Full name", "Registered at"}
	for i, want := range wantHeader {
		if all[0][i] != want {
			t.Fatalf("header[%d] = %q, want %q", i, all[0][i], want)
		}
	}

	// Order is preserved exactly as supplied by the service (topic-ordered).
	if all[1][2] != "Ama Mensah" || all[2][2] != "Kofi Boateng" || all[3][2] != "Yaa Asantewaa" {
		t.Fatalf("row order not preserved: %v", all[1:])
	}
	// Topic grouping: the two Compilers students are adjacent.
	if all[1][0] != "Compilers" || all[2][0] != "Compilers" || all[3][0] != "Distributed Systems" {
		t.Fatalf("rows are not grouped by topic: %v", all[1:])
	}
	if all[2][1] != "CS/2024/045" {
		t.Fatalf("index with slashes mangled: %q", all[2][1])
	}

	got, err := f.GetCellValue(xlsxRosterSheet, "D2")
	if err != nil {
		t.Fatalf("read D2: %v", err)
	}
	if !strings.HasPrefix(got, "2026-11-11") {
		t.Fatalf("D2 = %q, want a 2026-11-11 timestamp", got)
	}
}

// The grouped view is the point: one row per topic with occupancy, including
// topics nobody chose, plus a totals row.
func TestXLSXSummaryGroupsByTopic(t *testing.T) {
	data, err := writeXLSX(sampleExport())
	if err != nil {
		t.Fatalf("writeXLSX: %v", err)
	}

	f := openWorkbook(t, data)
	all, err := f.GetRows(xlsxSummarySheet)
	if err != nil {
		t.Fatalf("get rows: %v", err)
	}

	// 5 topics + header + totals row.
	if len(all) != 7 {
		t.Fatalf("summary rows = %d, want 7: %v", len(all), all)
	}

	wantHeader := []string{"Topic", "Capacity", "Registered", "Remaining", "Status"}
	for i, want := range wantHeader {
		if all[0][i] != want {
			t.Fatalf("header[%d] = %q, want %q", i, all[0][i], want)
		}
	}

	// Compilers: 10 capacity, 2 registered, 8 remaining, available.
	if all[1][0] != "Compilers" || all[1][1] != "10" || all[1][2] != "2" || all[1][3] != "8" || all[1][4] != "available" {
		t.Fatalf("Compilers row = %v", all[1])
	}
	// A full topic reports zero remaining and status "full".
	if all[3][0] != "Full Topic" || all[3][3] != "0" || all[3][4] != "full" {
		t.Fatalf("Full Topic row = %v", all[3])
	}
	// An inactive topic is still listed.
	if all[4][0] != "Inactive Topic" || all[4][4] != "inactive" {
		t.Fatalf("Inactive Topic row = %v", all[4])
	}
	// A topic nobody picked appears with zeroes.
	if all[5][0] != "Zero Picks" || all[5][2] != "0" || all[5][3] != "8" {
		t.Fatalf("Zero Picks row = %v", all[5])
	}

	// Totals: 10+30+2+5+8=55 capacity, 2+1+2+0+0=5 registered, 50 remaining.
	total := all[6]
	if total[0] != "TOTAL" || total[1] != "55" || total[2] != "5" || total[3] != "50" {
		t.Fatalf("totals row = %v", total)
	}
}

func TestXLSXGroupsSheetEmptyTakesTopicOrder(t *testing.T) {
	at := time.Date(2026, 11, 11, 9, 14, 2, 0, time.UTC)
	data := &ExportData{
		// Deliberately interleaved, as a date-sorted export would be.
		Rows: []AdminRegistration{
			{FullName: "Zoe Mensah", IndexNumber: "3", TopicName: "Compilers", CreatedAt: at},
			{FullName: "Ama Boateng", IndexNumber: "1", TopicName: "Distributed Systems", CreatedAt: at},
			{FullName: "Kofi Adjei", IndexNumber: "2", TopicName: "Compilers", CreatedAt: at},
		},
		Topics: []TopicSummary{
			{Name: "Compilers", Capacity: 5},
			{Name: "Distributed Systems", Capacity: 5},
		},
	}

	raw, err := writeXLSX(data)
	if err != nil {
		t.Fatalf("writeXLSX: %v", err)
	}
	f := openWorkbook(t, raw)
	all, err := f.GetRows(xlsxGroupsSheet)
	if err != nil {
		t.Fatalf("get groups rows: %v", err)
	}

	rows := make([][]string, 0, len(all))
	for _, row := range all {
		if len(row) > 0 && row[0] != "" {
			rows = append(rows, row)
		}
	}

	want := []string{
		"Full name",
		"GROUP 1 · Compilers — 2/5 filled · 3 places left",
		"Kofi Adjei",
		"Zoe Mensah",
		"GROUP 2 · Distributed Systems — 1/5 filled · 4 places left",
		"Ama Boateng",
	}
	if len(rows) != len(want) {
		t.Fatalf("groups rows = %v, want %v", rows, want)
	}
	for i := range want {
		if rows[i][0] != want[i] {
			t.Fatalf("groups row %d = %q, want %q", i, rows[i][0], want[i])
		}
	}
}

func TestXLSXEmptyExportStillHasHeader(t *testing.T) {
	data, err := writeXLSX(&ExportData{})
	if err != nil {
		t.Fatalf("writeXLSX: %v", err)
	}

	f := openWorkbook(t, data)

	roster, err := f.GetRows(xlsxRosterSheet)
	if err != nil {
		t.Fatalf("get roster rows: %v", err)
	}
	if len(roster) != 1 {
		t.Fatalf("roster rows = %d, want 1 header row", len(roster))
	}

	summary, err := f.GetRows(xlsxSummarySheet)
	if err != nil {
		t.Fatalf("get summary rows: %v", err)
	}
	// Header + a zeroed totals row.
	if len(summary) != 2 || summary[1][0] != "TOTAL" || summary[1][1] != "0" {
		t.Fatalf("summary rows = %v", summary)
	}

	groups, err := f.GetRows(xlsxGroupsSheet)
	if err != nil {
		t.Fatalf("get groups rows: %v", err)
	}
	if len(groups) != 1 || groups[0][0] != "Full name" {
		t.Fatalf("groups rows = %v, want just the header", groups)
	}
}

func TestXLSXFilename(t *testing.T) {
	at := time.Date(2026, 11, 11, 9, 14, 2, 0, time.UTC)
	if got := ExcelExportFilename(at); got != "groupify-registrations-20261111-091402.xlsx" {
		t.Fatalf("filename = %q", got)
	}
}

func TestWriteCSVPreservesIdentifiers(t *testing.T) {
	data := writeCSV(sampleExport().Rows)

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
