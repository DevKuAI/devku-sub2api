package channel

import (
	"bytes"
	"math"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"
)

// pdfCell is a run of glyphs on one text line; X is where the run starts.
type pdfCell struct {
	X    float64
	Text string
}

type pdfRow struct {
	Page  int
	Y     float64
	Cells []pdfCell
}

func (r pdfRow) texts() []string {
	out := make([]string, len(r.Cells))
	for i, c := range r.Cells {
		out[i] = c.Text
	}
	return out
}

// pdfRows rebuilds text lines from glyph positions. Statement PDFs place every glyph absolutely, so grouping
// by baseline and splitting on wide horizontal gaps recovers table rows and cells reliably.
func pdfRows(data []byte) (rows []pdfRow, err error) {
	defer func() {
		// The PDF reader panics on some malformed content streams; treat that as an unreadable file.
		if recovered := recover(); recovered != nil {
			rows, err = nil, errUnreadablePDF
		}
	}()
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errUnreadablePDF
	}
	const sameLine, cellGap = 2.0, 8.0
	for page := 1; page <= reader.NumPage(); page++ {
		var glyphs []pdf.Text
		for _, t := range reader.Page(page).Content().Text {
			if t.S != "\n" && t.S != "" {
				glyphs = append(glyphs, t)
			}
		}
		sort.SliceStable(glyphs, func(i, j int) bool {
			if math.Abs(glyphs[i].Y-glyphs[j].Y) > sameLine {
				return glyphs[i].Y > glyphs[j].Y
			}
			return glyphs[i].X < glyphs[j].X
		})
		var lines [][]pdf.Text
		for _, g := range glyphs {
			if n := len(lines); n > 0 && math.Abs(lines[n-1][0].Y-g.Y) <= sameLine {
				lines[n-1] = append(lines[n-1], g)
				continue
			}
			lines = append(lines, []pdf.Text{g})
		}
		for _, line := range lines {
			sort.SliceStable(line, func(i, j int) bool { return line[i].X < line[j].X })
			row := pdfRow{Page: page, Y: line[0].Y}
			var text strings.Builder
			start, end := line[0].X, -1.0
			flush := func() {
				if s := strings.TrimSpace(text.String()); s != "" {
					row.Cells = append(row.Cells, pdfCell{X: start, Text: s})
				}
				text.Reset()
			}
			for _, g := range line {
				if end >= 0 && g.X-end > cellGap {
					flush()
					start = g.X
				}
				text.WriteString(g.S)
				end = g.X + g.W
			}
			flush()
			if len(row.Cells) > 0 {
				rows = append(rows, row)
			}
		}
	}
	return rows, nil
}

type pdfError string

func (e pdfError) Error() string { return string(e) }

const errUnreadablePDF = pdfError("unreadable PDF")
