package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripTerminalCodes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "plain text unchanged",
			in:   "hello world\n",
			want: "hello world\n",
		},
		{
			name: "strips ANSI color codes",
			in:   "\x1b[31mred\x1b[0m\n",
			want: "red\n",
		},
		{
			name: "strips ANSI cursor movement",
			in:   "\x1b[2Jcleared\n",
			want: "cleared\n",
		},
		{
			name: "collapses carriage-return overwrite",
			in:   "progress 10%\rprogress 50%\rprogress 100%\n",
			want: "progress 100%\n",
		},
		{
			name: "skips whitespace-only lines from overwrite",
			in:   "real line\n   \r   \n",
			want: "real line\n",
		},
		{
			name: "mixed ANSI and CR",
			in:   "\x1b[33mloading...\r\x1b[32mdone!\x1b[0m\n",
			want: "done!\n",
		},
		{
			name: "multiple lines with CR",
			in:   "line1\rx\nline2\ry\n",
			want: "x\ny\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripTerminalCodes(tt.in)
			if got != tt.want {
				t.Errorf("stripTerminalCodes(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestYesNo(t *testing.T) {
	if got := yesNo(true); got != "yes" {
		t.Errorf("yesNo(true) = %q, want %q", got, "yes")
	}
	if got := yesNo(false); got != "no" {
		t.Errorf("yesNo(false) = %q, want %q", got, "no")
	}
}

func TestReadWriteIterationFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "iteration.txt")

	// Read non-existent file
	if _, err := readIterationFile(path); err == nil {
		t.Error("expected error reading non-existent file")
	}

	// Write and read back
	if err := writeIterationFile(path, 7); err != nil {
		t.Fatalf("writeIterationFile: %v", err)
	}
	got, err := readIterationFile(path)
	if err != nil {
		t.Fatalf("readIterationFile: %v", err)
	}
	if got != 7 {
		t.Errorf("readIterationFile = %d, want 7", got)
	}

	// Overwrite and read back
	if err := writeIterationFile(path, 42); err != nil {
		t.Fatalf("writeIterationFile: %v", err)
	}
	got, err = readIterationFile(path)
	if err != nil {
		t.Fatalf("readIterationFile: %v", err)
	}
	if got != 42 {
		t.Errorf("readIterationFile = %d, want 42", got)
	}
}

func TestReadIterationFileWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "iteration.txt")

	// File with trailing newline and spaces
	if err := os.WriteFile(path, []byte("  99  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readIterationFile(path)
	if err != nil {
		t.Fatalf("readIterationFile: %v", err)
	}
	if got != 99 {
		t.Errorf("readIterationFile = %d, want 99", got)
	}
}

func TestReadIterationFileInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "iteration.txt")

	if err := os.WriteFile(path, []byte("not-a-number"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readIterationFile(path); err == nil {
		t.Error("expected error reading non-numeric file")
	}
}

func TestNewLoggerCreatesDir(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sub", "ralph.log")

	l := newLogger(logPath)
	defer l.close()

	// Write something
	l.info("test message")
	l.write("some output\n")
	l.warn("warning message")

	// Verify file exists and has content
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if len(data) == 0 {
		t.Error("log file is empty")
	}
}

func TestNewLoggerNoopWhenCantCreate(t *testing.T) {
	// Use a path that can't be created (file where dir should be)
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(blocker, "ralph.log") // blocker is a file, not a dir

	l := newLogger(logPath)
	defer l.close()

	// Should not panic
	l.info("test")
	l.write("test")
	l.warn("test")
}
