package output

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		if got := FormatBytes(tt.input); got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "00:00"},
		{30, "00:30"},
		{60, "01:00"},
		{90, "01:30"},
		{3600, "01:00:00"},
		{3661, "01:01:01"},
	}
	for _, tt := range tests {
		if got := FormatDuration(tt.input); got != tt.want {
			t.Errorf("FormatDuration(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"a", "*"},
		{"abcdefgh", "********"},
		{"sk_live_abcdefgh12345678", "sk_l****************5678"},
	}
	for _, tt := range tests {
		if got := MaskKey(tt.input); got != tt.want {
			t.Errorf("MaskKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncateUUID(t *testing.T) {
	uuid := "2f71a6cb-8c31-4c5b-9cb9-821d87d9a100"
	truncated := TruncateUUID(uuid)
	if len(truncated) != 8 {
		t.Errorf("expected 8 chars, got %d: %s", len(truncated), truncated)
	}
}

func TestPluralize(t *testing.T) {
	if Pluralize(1, "file", "files") != "1 file" {
		t.Errorf("unexpected singular: %s", Pluralize(1, "file", "files"))
	}
	if Pluralize(2, "file", "files") != "2 files" {
		t.Errorf("unexpected plural: %s", Pluralize(2, "file", "files"))
	}
}

func TestFormatBool(t *testing.T) {
	if FormatBool(true) != "Yes" {
		t.Error("expected Yes")
	}
	if FormatBool(false) != "No" {
		t.Error("expected No")
	}
}

func TestOutputJSONNoANSI(t *testing.T) {
	var buf bytes.Buffer
	out := New(true, false, false)
	out.stdout = &buf

	result := map[string]string{"key": "value"}
	out.PrintJSON(result)

	output := buf.String()
	if strings.Contains(output, "\x1b") {
		t.Error("JSON output should not contain ANSI escape codes")
	}

	var decoded map[string]string
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("JSON output should be valid JSON: %s", err)
	}
}

func TestOutputJSONContainsFullUUIDs(t *testing.T) {
	var buf bytes.Buffer
	out := New(true, false, false)
	out.stdout = &buf

	result := map[string]string{"uuid": "2f71a6cb-8c31-4c5b-9cb9-821d87d9a100"}
	out.PrintJSON(result)

	output := buf.String()
	if !strings.Contains(output, "2f71a6cb-8c31-4c5b-9cb9-821d87d9a100") {
		t.Error("JSON output should contain full UUIDs")
	}
}

func TestNoColorDisablesANSIDuringNOCOLOR(t *testing.T) {
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	out := New(false, false, false)
	if !out.NoColor() {
		t.Error("expected NoColor() to return true when NO_COLOR is set")
	}
}

func TestNoColorFlag(t *testing.T) {
	out := New(false, true, false)
	if !out.NoColor() {
		t.Error("expected NoColor() to return true when --no-color is set")
	}
}

func TestKVNotInJSON(t *testing.T) {
	var buf bytes.Buffer
	out := New(true, false, false)
	out.stdout = &buf

	out.PrintKV("Key", "Value")
	if buf.Len() > 0 {
		t.Error("KV output should be suppressed in JSON mode")
	}
}

func TestTableNotInJSON(t *testing.T) {
	var buf bytes.Buffer
	out := New(true, false, false)
	out.stdout = &buf

	out.Table([]string{"H1"}, [][]string{{"V1"}})
	if buf.Len() > 0 {
		t.Error("Table output should be suppressed in JSON mode")
	}
}

func TestTableAlignment(t *testing.T) {
	var buf bytes.Buffer
	out := New(false, true, false)
	out.stdout = &buf

	headers := []string{"Name", "UUID"}
	rows := [][]string{
		{"Accra Radio", "2f71a6cb"},
		{"Kumasi FM", "3a82b7dc"},
	}
	out.Table(headers, rows)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines (header, separator, 2 data rows), got %d", len(lines))
	}

	if !strings.HasPrefix(lines[2], "Accra Radio") {
		t.Errorf("expected row to start with Accra Radio, got %q", lines[2])
	}
}

func TestPrintStdErr(t *testing.T) {
	var buf bytes.Buffer
	out := New(false, false, false)
	out.stderr = &buf

	out.PrintStdErr("test error")
	if buf.String() != "test error\n" {
		t.Errorf("expected 'test error\\n', got %q", buf.String())
	}
}

func TestOutputDebug(t *testing.T) {
	var stdoutBuf, stderrBuf bytes.Buffer
	out := New(false, false, true)
	out.stdout = &stdoutBuf
	out.stderr = &stderrBuf

	out.PrintDebug("debug message")
	if stderrBuf.Len() == 0 {
		t.Error("expected debug output on stderr")
	}
}

func TestOutputDebugSuppressed(t *testing.T) {
	var stderrBuf bytes.Buffer
	out := New(false, false, false)
	out.stderr = &stderrBuf

	out.PrintDebug("debug message")
	if stderrBuf.Len() > 0 {
		t.Error("expected no debug output when debug is false")
	}
}
