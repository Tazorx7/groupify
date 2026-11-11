package registrations

import (
	"bytes"
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	// xlsxSummarySheet answers "how is the class distributed?" at a glance.
	xlsxSummarySheet = "Summary"
	// xlsxRosterSheet is the flat roster, grouped by topic.
	xlsxRosterSheet = "Registrations"
)

// Spreadsheet number formats.
//
// numFmtText is the built-in "@" (Text) format. It is the whole reason the XLSX
// export exists: index numbers carry leading zeroes ("00123456") and a plain
// CSV opened in Excel silently converts them to numbers. Writing those cells as
// explicitly text-typed keeps the value intact if the admin saves the workbook
// or re-imports it.
const (
	numFmtText     = 49
	numFmtDateTime = "yyyy-mm-dd hh:mm"
)

type sheetStyles struct {
	header int
	text   int
	date   int
	total  int
}

// writeXLSX renders a two-sheet workbook:
//
//	Summary       — one row per topic: capacity, registered, remaining, status,
//	                then a totals row. Built from the topics table, so topics
//	                nobody picked still appear.
//	Registrations — the roster ordered by topic (then student), with a frozen
//	                header and autofilter so it stays usable as raw data.
//
// Identifiers are text cells; the timestamp is a real datetime cell so Excel
// sorts chronologically instead of lexically.
func writeXLSX(data *ExportData) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if err := f.SetSheetName("Sheet1", xlsxSummarySheet); err != nil {
		return nil, fmt.Errorf("rename summary sheet: %w", err)
	}
	if _, err := f.NewSheet(xlsxRosterSheet); err != nil {
		return nil, fmt.Errorf("create roster sheet: %w", err)
	}

	styles, err := newStyles(f)
	if err != nil {
		return nil, err
	}

	if err := writeSummarySheet(f, styles, data.Topics); err != nil {
		return nil, err
	}
	if err := writeRosterSheet(f, styles, data.Rows); err != nil {
		return nil, err
	}

	f.SetActiveSheet(0)

	// Freeze headers and size columns for both sheets.
	if err := f.SetPanes(xlsxSummarySheet, frozenHeader()); err != nil {
		return nil, fmt.Errorf("freeze summary header: %w", err)
	}
	if err := f.SetPanes(xlsxRosterSheet, frozenHeader()); err != nil {
		return nil, fmt.Errorf("freeze roster header: %w", err)
	}

	widths := map[string]map[string]float64{
		xlsxSummarySheet: {"A": 32, "B": 12, "C": 14, "D": 12, "E": 14},
		xlsxRosterSheet:  {"A": 32, "B": 20, "C": 30, "D": 20},
	}
	for sheet, cols := range widths {
		for col, width := range cols {
			if err := f.SetColWidth(sheet, col, col, width); err != nil {
				return nil, fmt.Errorf("column width %s!%s: %w", sheet, col, err)
			}
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("serialise workbook: %w", err)
	}
	return buf.Bytes(), nil
}

func newStyles(f *excelize.File) (sheetStyles, error) {
	var s sheetStyles
	var err error

	s.header, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FF1F2937"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFE8EEF7"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    []excelize.Border{{Type: "bottom", Style: 1, Color: "FF9CA3AF"}},
	})
	if err != nil {
		return s, fmt.Errorf("header style: %w", err)
	}

	s.text, err = f.NewStyle(&excelize.Style{NumFmt: numFmtText})
	if err != nil {
		return s, fmt.Errorf("text style: %w", err)
	}

	s.date, err = f.NewStyle(&excelize.Style{CustomNumFmt: ptr(numFmtDateTime)})
	if err != nil {
		return s, fmt.Errorf("date style: %w", err)
	}

	s.total, err = f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true},
		Border: []excelize.Border{{Type: "top", Style: 1, Color: "FF9CA3AF"}},
	})
	if err != nil {
		return s, fmt.Errorf("total style: %w", err)
	}

	return s, nil
}

func writeSummarySheet(f *excelize.File, styles sheetStyles, topics []TopicSummary) error {
	sw, err := f.NewStreamWriter(xlsxSummarySheet)
	if err != nil {
		return fmt.Errorf("summary stream writer: %w", err)
	}

	header := make([]any, 0, 5)
	for _, label := range []string{"Topic", "Capacity", "Registered", "Remaining", "Status"} {
		header = append(header, excelize.Cell{StyleID: styles.header, Value: label})
	}
	if err := sw.SetRow("A1", header); err != nil {
		return fmt.Errorf("summary header: %w", err)
	}

	var totalCapacity, totalMembers int
	for i, t := range topics {
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return err
		}
		totalCapacity += t.Capacity
		totalMembers += t.Members

		row := []any{
			excelize.Cell{StyleID: styles.text, Value: t.Name},
			t.Capacity,
			t.Members,
			max0(t.Capacity - t.Members),
			excelize.Cell{StyleID: styles.text, Value: summaryStatus(t)},
		}
		if err := sw.SetRow(cell, row); err != nil {
			return fmt.Errorf("summary row %d: %w", i+2, err)
		}
	}

	totalCell, err := excelize.CoordinatesToCellName(1, len(topics)+2)
	if err != nil {
		return err
	}
	totalRow := []any{
		excelize.Cell{StyleID: styles.total, Value: "TOTAL"},
		excelize.Cell{StyleID: styles.total, Value: totalCapacity},
		excelize.Cell{StyleID: styles.total, Value: totalMembers},
		excelize.Cell{StyleID: styles.total, Value: max0(totalCapacity - totalMembers)},
		excelize.Cell{StyleID: styles.total, Value: ""},
	}
	if err := sw.SetRow(totalCell, totalRow); err != nil {
		return fmt.Errorf("summary total row: %w", err)
	}

	if err := sw.Flush(); err != nil {
		return fmt.Errorf("flush summary: %w", err)
	}

	lastCell, err := excelize.CoordinatesToCellName(5, len(topics)+2)
	if err != nil {
		return err
	}
	if err := f.AutoFilter(xlsxSummarySheet, "A1:"+lastCell, nil); err != nil {
		return fmt.Errorf("summary autofilter: %w", err)
	}
	return nil
}

func writeRosterSheet(f *excelize.File, styles sheetStyles, rows []AdminRegistration) error {
	sw, err := f.NewStreamWriter(xlsxRosterSheet)
	if err != nil {
		return fmt.Errorf("roster stream writer: %w", err)
	}

	header := make([]any, 0, 4)
	for _, label := range []string{"Topic", "Index number", "Full name", "Registered at"} {
		header = append(header, excelize.Cell{StyleID: styles.header, Value: label})
	}
	if err := sw.SetRow("A1", header); err != nil {
		return fmt.Errorf("roster header: %w", err)
	}

	for i, reg := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return err
		}
		row := []any{
			excelize.Cell{StyleID: styles.text, Value: reg.TopicName},
			excelize.Cell{StyleID: styles.text, Value: reg.IndexNumber},
			excelize.Cell{StyleID: styles.text, Value: reg.FullName},
			excelize.Cell{StyleID: styles.date, Value: reg.CreatedAt.UTC()},
		}
		if err := sw.SetRow(cell, row); err != nil {
			return fmt.Errorf("roster row %d: %w", i+2, err)
		}
	}

	if err := sw.Flush(); err != nil {
		return fmt.Errorf("flush roster: %w", err)
	}

	lastCell, err := excelize.CoordinatesToCellName(4, len(rows)+1)
	if err != nil {
		return err
	}
	if err := f.AutoFilter(xlsxRosterSheet, "A1:"+lastCell, nil); err != nil {
		return fmt.Errorf("roster autofilter: %w", err)
	}
	return nil
}

func summaryStatus(t TopicSummary) string {
	switch {
	case !t.IsActive:
		return "inactive"
	case t.Members >= t.Capacity:
		return "full"
	default:
		return "available"
	}
}

func frozenHeader() *excelize.Panes {
	return &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func ptr[T any](v T) *T { return &v }

// ExcelExportFilename builds the download name, e.g.
// groupify-registrations-20261111-091402.xlsx
func ExcelExportFilename(now time.Time) string {
	return fmt.Sprintf("groupify-registrations-%s.xlsx", now.UTC().Format("20060102-150405"))
}
