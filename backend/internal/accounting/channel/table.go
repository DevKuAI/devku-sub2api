package channel

import (
	"bytes"
	"encoding/csv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// table is a sheet read as text: the first row is the header, values keep their displayed form.
type table struct {
	header []string
	rows   [][]string
}

func readTable(f SourceFile) (*table, error) {
	var records [][]string
	switch f.Ext() {
	case ".csv":
		r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(f.Data, []byte("\xef\xbb\xbf"))))
		r.FieldsPerRecord = -1
		var err error
		if records, err = r.ReadAll(); err != nil {
			return nil, problem(f.Name, 0, "", "invalid CSV: %v", err)
		}
	case ".xlsx":
		book, err := excelize.OpenReader(bytes.NewReader(f.Data))
		if err != nil {
			return nil, problem(f.Name, 0, "", "invalid xlsx: %v", err)
		}
		defer func() { _ = book.Close() }()
		sheets := book.GetSheetList()
		if len(sheets) == 0 {
			return nil, problem(f.Name, 0, "", "workbook has no sheet")
		}
		if records, err = book.GetRows(sheets[0]); err != nil {
			return nil, problem(f.Name, 0, "", "invalid xlsx: %v", err)
		}
	default:
		return nil, problem(f.Name, 0, "", "unsupported file type %s", f.Ext())
	}
	if len(records) == 0 {
		return nil, problem(f.Name, 0, "", "file is empty")
	}
	t := &table{}
	for _, cell := range records[0] {
		t.header = append(t.header, strings.TrimSpace(cell))
	}
	for _, record := range records[1:] {
		blank := true
		for _, cell := range record {
			if strings.TrimSpace(cell) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		t.rows = append(t.rows, record)
	}
	return t, nil
}

// column returns the index of a header, or -1.
func (t *table) column(name string) int {
	for i, h := range t.header {
		if h == name {
			return i
		}
	}
	return -1
}

func (t *table) has(names ...string) bool {
	for _, name := range names {
		if t.column(name) < 0 {
			return false
		}
	}
	return true
}

// record maps header to trimmed value for one row.
func (t *table) record(row []string) map[string]string {
	out := make(map[string]string, len(t.header))
	for i, h := range t.header {
		if h == "" {
			continue
		}
		if i < len(row) {
			out[h] = strings.TrimSpace(row[i])
		} else {
			out[h] = ""
		}
	}
	return out
}
