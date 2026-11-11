package registrations

import (
	"bytes"
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

// xlsxSheetName is the single worksheet in the export.
const xlsxSheetName = "Registrations"

// Spreadsheet number formats.
//
// numFmtText is the built-in "@" (Text) format. It is the whole reason this
// export exists: index numbers carry leading zeroes ("00123456") and a plain
// CSV opened in Excel silently converts them to numbers. Writing those cells as
// explicitly text-typed keeps the value intact if the admin saves the workbook
// or re-imports it.
const (
	numFmtText     = 49
	numFmtDateTime = "yyyy-mm-dd hh:mm"
)

// writeXLSX renders registrations as a single-sheet .xlsx workbook.
//
// Layout: bold header row, frozen top row, autofilter, fixed column widths.
// Identifiers are text cells; the timestamp is a real datetime cell so Excel
// sorts chronologically instead of lexically.
func writeXLSX(rows []AdminRegistration) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if err := f.SetSheetName("Sheet1", xlsxSheetName); err != nil {
		return nil, fmt.Errorf("rename sheet: %w", err)
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FF1F2937"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFE8EEF7"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border: []excelize.Border{
			{Type: "bottom", Style: 1, Color: "FF9CA3AF"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("header style: %w", err)
	}

	textStyle, err := f.NewStyle(&excelize.Style{NumFmt: numFmtText})
	if err != nil {
		return nil, fmt.Errorf("text style: %w", err)
	}

	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: ptr(numFmtDateTime)})
	if err != nil {
		return nil, fmt.Errorf("date style: %w", err)
	}

	sw, err := f.NewStreamWriter(xlsxSheetName)
	if err != nil {
		return nil, fmt.Errorf("stream writer: %w", err)
	}

	headers := []any{
		excelize.Cell{StyleID: headerStyle, Value: "index_number"},
		excelize.Cell{StyleID: headerStyle, Value: "full_name"},
		excelize.Cell{StyleID: headerStyle, Value: "topic"},
		excelize.Cell{StyleID: headerStyle, Value: "registered_at"},
	}
	if err := sw.SetRow("A1", headers); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}

	for i, reg := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return nil, fmt.Errorf("row %d coordinate: %w", i+2, err)
		}
		row := []any{
			excelize.Cell{StyleID: textStyle, Value: reg.IndexNumber},
			excelize.Cell{StyleID: textStyle, Value: reg.FullName},
			excelize.Cell{StyleID: textStyle, Value: reg.TopicName},
			excelize.Cell{StyleID: dateStyle, Value: reg.CreatedAt.UTC()},
		}
		if err := sw.SetRow(cell, row); err != nil {
			return nil, fmt.Errorf("write row %d: %w", i+2, err)
		}
	}

	// The stream writer must be flushed before any other workbook operation.
	if err := sw.Flush(); err != nil {
		return nil, fmt.Errorf("flush rows: %w", err)
	}

	lastRow := len(rows) + 1
	lastCell, err := excelize.CoordinatesToCellName(4, lastRow)
	if err != nil {
		return nil, err
	}
	if err := f.AutoFilter(xlsxSheetName, "A1:"+lastCell, nil); err != nil {
		return nil, fmt.Errorf("autofilter: %w", err)
	}
	if err := f.SetPanes(xlsxSheetName, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return nil, fmt.Errorf("freeze header: %w", err)
	}

	widths := map[string]float64{"A": 20, "B": 30, "C": 30, "D": 20}
	for col, width := range widths {
		if err := f.SetColWidth(xlsxSheetName, col, col, width); err != nil {
			return nil, fmt.Errorf("column width %s: %w", col, err)
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("serialise workbook: %w", err)
	}
	return buf.Bytes(), nil
}

func ptr[T any](v T) *T { return &v }

// ExcelExportFilename builds the download name, e.g.
// groupify-registrations-20261111-091402.xlsx
func ExcelExportFilename(now time.Time) string {
	return fmt.Sprintf("groupify-registrations-%s.xlsx", now.UTC().Format("20060102-150405"))
}
