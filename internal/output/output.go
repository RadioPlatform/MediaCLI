package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	ColorTitle   = color.New(color.FgHiWhite, color.Bold)
	ColorOK      = color.New(color.FgGreen, color.Bold)
	ColorError   = color.New(color.FgRed, color.Bold)
	ColorInfo    = color.New(color.FgCyan)
	ColorLabel   = color.New(color.FgHiWhite)
	ColorValue   = color.New(color.FgWhite)
	ColorMuted   = color.New(color.FgHiBlack)
	ColorWarning = color.New(color.FgYellow)
)

type Output struct {
	stdout  io.Writer
	stderr  io.Writer
	json    bool
	noColor bool
	debug   bool
}

func New(jsonMode, noColor, debug bool) *Output {
	return &Output{
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		json:    jsonMode,
		noColor: noColor,
		debug:   debug,
	}
}

func (o *Output) IsJSON() bool {
	return o.json
}

func (o *Output) IsDebug() bool {
	return o.debug
}

func (o *Output) NoColor() bool {
	if o.noColor {
		return true
	}
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	return false
}

func (o *Output) Print(v ...interface{}) {
	if o.json {
		return
	}
	fmt.Fprint(o.stdout, v...)
}

func (o *Output) Println(v ...interface{}) {
	if o.json {
		return
	}
	fmt.Fprintln(o.stdout, v...)
}

func (o *Output) Printf(format string, v ...interface{}) {
	if o.json {
		return
	}
	fmt.Fprintf(o.stdout, format, v...)
}

func (o *Output) PrintTitle(v string) {
	if o.json {
		return
	}
	if o.NoColor() {
		fmt.Fprintln(o.stdout, v)
		return
	}
	ColorTitle.Fprintln(o.stdout, v)
}

func (o *Output) PrintOK(v string) {
	if o.json {
		return
	}
	if o.NoColor() {
		fmt.Fprintln(o.stdout, v)
		return
	}
	ColorOK.Fprintln(o.stdout, v)
}

func (o *Output) PrintError(v string) {
	if o.NoColor() {
		fmt.Fprintln(o.stderr, v)
		return
	}
	ColorError.Fprintln(o.stderr, v)
}

func (o *Output) PrintWarning(v string) {
	if o.NoColor() {
		fmt.Fprintln(o.stderr, v)
		return
	}
	ColorWarning.Fprintln(o.stderr, v)
}

func (o *Output) PrintInfo(v string) {
	if o.json {
		return
	}
	if o.NoColor() {
		fmt.Fprintln(o.stdout, v)
		return
	}
	ColorInfo.Fprintln(o.stdout, v)
}

func (o *Output) PrintKV(label, value string) {
	if o.json {
		return
	}
	if o.NoColor() {
		fmt.Fprintf(o.stdout, "%-20s %s\n", label+":", value)
		return
	}
	ColorLabel.Fprintf(o.stdout, "%-20s ", label+":")
	ColorValue.Fprintln(o.stdout, value)
}

func (o *Output) PrintDebug(v string) {
	if !o.debug {
		return
	}
	if o.NoColor() {
		fmt.Fprintln(o.stderr, "[debug] "+v)
		return
	}
	ColorMuted.Fprintln(o.stderr, "[debug] "+v)
}

func (o *Output) PrintJSON(v interface{}) {
	enc := json.NewEncoder(o.stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		o.PrintError("failed to encode JSON output")
	}
}

func (o *Output) PrintJSONError(v interface{}) {
	enc := json.NewEncoder(o.stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(o.stderr, "failed to encode JSON error output")
	}
}

func (o *Output) PrintStdErr(v string) {
	fmt.Fprintln(o.stderr, v)
}

func (o *Output) Table(headers []string, rows [][]string) {
	if o.json {
		return
	}

	if len(rows) == 0 {
		return
	}

	cols := len(headers)
	if cols == 0 {
		return
	}

	widths := make([]int, cols)
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < cols && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	sep := make([]string, cols)
	for i, w := range widths {
		sep[i] = strings.Repeat("-", w)
	}

	format := ""
	for i, w := range widths {
		if i > 0 {
			format += "  "
		}
		format += fmt.Sprintf("%%-%ds", w)
	}
	format += "\n"

	headerStrs := make([]interface{}, cols)
	for i, h := range headers {
		headerStrs[i] = h
	}

	if o.NoColor() {
		fmt.Fprintf(o.stdout, format, headerStrs...)
		fmt.Fprintf(o.stdout, format, toInterfaceSlice(sep)...)
	} else {
		ColorLabel.Fprintf(o.stdout, format, headerStrs...)
		ColorMuted.Fprintf(o.stdout, format, toInterfaceSlice(sep)...)
	}

	for _, row := range rows {
		rowStrs := make([]interface{}, cols)
		for i, cell := range row {
			rowStrs[i] = cell
		}
		if o.NoColor() {
			fmt.Fprintf(o.stdout, format, rowStrs...)
		} else {
			ColorValue.Fprintf(o.stdout, format, rowStrs...)
		}
	}
}

func toInterfaceSlice(s []string) []interface{} {
	r := make([]interface{}, len(s))
	for i, v := range s {
		r[i] = v
	}
	return r
}
