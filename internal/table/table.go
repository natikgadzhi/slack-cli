package table

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Table renders a bordered ASCII table to an io.Writer.
type Table struct {
	out     io.Writer
	headers []string
	rows    [][]string
	widths  []int
}

// New creates a new bordered table that writes to stdout.
func New() *Table {
	return &Table{out: os.Stdout}
}

// NewWriter creates a new bordered table with a custom destination.
func NewWriter(out io.Writer) *Table {
	return &Table{out: out}
}

// Header sets the column headers (uppercased for display).
func (t *Table) Header(columns ...string) {
	t.headers = make([]string, len(columns))
	t.widths = make([]int, len(columns))
	for i, c := range columns {
		t.headers[i] = strings.ToUpper(c)
		t.widths[i] = len(t.headers[i])
	}
}

// Row adds a data row.
func (t *Table) Row(values ...string) {
	t.rows = append(t.rows, values)
	for i, v := range values {
		if i < len(t.widths) && len(v) > t.widths[i] {
			t.widths[i] = len(v)
		}
	}
}

// Flush renders the table, shrinking columns to fit the terminal if needed.
func (t *Table) Flush() error {
	t.fitToTerminal()
	fmt.Fprintln(t.out, t.line("┌", "┬", "┐"))
	fmt.Fprintln(t.out, t.formatRow(t.headers))
	fmt.Fprintln(t.out, t.line("├", "┼", "┤"))
	for _, row := range t.rows {
		fmt.Fprintln(t.out, t.formatRow(row))
	}
	fmt.Fprintln(t.out, t.line("└", "┴", "┘"))
	return nil
}

// fitToTerminal shrinks the widest columns to fit within the terminal width.
func (t *Table) fitToTerminal() {
	termWidth := getTerminalWidth()
	if termWidth <= 0 {
		return
	}

	for t.tableWidth() > termWidth {
		// Find the widest column and shrink it by one.
		widest := 0
		for i := 1; i < len(t.widths); i++ {
			if t.widths[i] > t.widths[widest] {
				widest = i
			}
		}
		// Don't shrink below the header length or 4 chars (room for "x…").
		minWidth := len(t.headers[widest])
		if minWidth < 4 {
			minWidth = 4
		}
		if t.widths[widest] <= minWidth {
			break // can't shrink further
		}
		t.widths[widest]--
	}
}

// tableWidth returns the total rendered width of the table.
// Each column contributes: 1 (border) + 1 (pad) + width + 1 (pad), plus final border.
func (t *Table) tableWidth() int {
	// │ col1 │ col2 │ ... │ colN │
	// = len(widths) borders + len(widths) * (width + 2 padding) + 1 final border
	w := 1 // leading │
	for _, cw := range t.widths {
		w += cw + 2 + 1 // padding + content + trailing │
	}
	return w
}

func (t *Table) line(left, mid, right string) string {
	parts := make([]string, len(t.widths))
	for i, w := range t.widths {
		parts[i] = strings.Repeat("─", w+2)
	}
	return left + strings.Join(parts, mid) + right
}

func (t *Table) formatRow(values []string) string {
	parts := make([]string, len(t.widths))
	for i, w := range t.widths {
		val := ""
		if i < len(values) {
			val = values[i]
		}
		val = truncate(val, w)
		parts[i] = fmt.Sprintf(" %-*s ", w, val)
	}
	return "│" + strings.Join(parts, "│") + "│"
}

// truncate shortens s to maxLen, replacing the last char with "…" if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return s[:maxLen-1] + "…"
}

func getTerminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0
	}
	return w
}
