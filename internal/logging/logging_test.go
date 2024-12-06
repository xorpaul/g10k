package logging

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
)

func TestVerbosef(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(nil)
	}()

	tests := []struct {
		name     string
		debug    bool
		verbose  bool
		message  string
		expected string
	}{
		{"DebugTrue", true, false, "Debug message", "Debug message\n"},
		{"VerboseTrue", false, true, "Verbose message", "Verbose message\n"},
		{"BothFalse", false, false, "No message", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Debug = tt.debug
			Verbose = tt.verbose
			buf.Reset()

			Verbosef(tt.message)

			if tt.expected != "" {
				expectedPrefix := time.Now().Format("2006/01/02 15:04:05")
				if got := buf.String(); !bytes.HasPrefix([]byte(got), []byte(expectedPrefix)) || !bytes.HasSuffix([]byte(got), []byte(tt.expected)) {
					t.Errorf("Verbosef() = %v, want prefix %v and suffix %v", got, expectedPrefix, tt.expected)
				}
			} else {
				if got := buf.String(); got != tt.expected {
					t.Errorf("Verbosef() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}
func TestDebugf(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(nil)
	}()

	tests := []struct {
		name     string
		debug    bool
		message  string
		expected string
	}{
		{"DebugTrue", true, "Debug message", "DEBUG callDebugf(): Debug message\n"},
		{"DebugFalse", false, "No message", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Debug = tt.debug
			buf.Reset()

			callDebugf(tt.message)

			if tt.expected != "" {
				expectedPrefix := time.Now().Format("2006/01/02 15:04:05")
				if got := buf.String(); !bytes.HasPrefix([]byte(got), []byte(expectedPrefix)) || !strings.Contains(got, tt.expected) {
					t.Errorf("Debugf() = %v, want prefix %v and contains %v", got, expectedPrefix, tt.expected)
				}
			} else {
				if got := buf.String(); got != tt.expected {
					t.Errorf("Debugf() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}

func TestDebugfAnon(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(nil)
	}()

	tests := []struct {
		name     string
		debug    bool
		message  string
		expected string
	}{
		{"DebugTrue", true, "Debug message", "DEBUG Debug message\n"},
		{"DebugFalse", false, "No message", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Debug = tt.debug
			buf.Reset()

			Debugf(tt.message)

			if tt.expected != "" {
				expectedPrefix := time.Now().Format("2006/01/02 15:04:05")
				if got := buf.String(); !bytes.HasPrefix([]byte(got), []byte(expectedPrefix)) || !strings.Contains(got, tt.expected) {
					t.Errorf("Debugf() = %v, want prefix %v and contains %v", got, expectedPrefix, tt.expected)
				}
			} else {
				if got := buf.String(); got != tt.expected {
					t.Errorf("Debugf() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}

func callDebugf(message string) {
	Debugf(message)
}

func TestWarnf(t *testing.T) {
	r, w, _ := os.Pipe()
	originalStdout := os.Stdout
	os.Stdout = w
	color.Output = os.Stdout
	color.NoColor = false

	message := "Warning: something went wrong!"

	Warnf(message)

	w.Close()
	os.Stdout = originalStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	expected := "\x1b[33m" + message + "\n\x1b[0m"

	if buf.String() != expected {
		t.Errorf("Warnf() = %q, want %q", buf.String(), expected)
	}
}

func TestInfof(t *testing.T) {
	r, w, _ := os.Pipe()
	originalStdout := os.Stdout
	os.Stdout = w

	color.Output = os.Stdout
	color.NoColor = false
	message := "This is an info message"

	tests := []struct {
		name      string
		debug     bool
		verbose   bool
		info      bool
		expectOut bool
	}{
		{"All flags false", false, false, false, false},
		{"Only Debug true", true, false, false, true},
		{"Only Verbose true", false, true, false, true},
		{"Only Info true", false, false, true, true},
		{"All flags true", true, true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the global variables
			Debug = tt.debug
			Verbose = tt.verbose
			Info = tt.info

			// Call the function
			Infof(message)

			// Close the writer and restore stdout
			w.Close()
			os.Stdout = originalStdout

			// Read the captured output
			var buf bytes.Buffer
			buf.ReadFrom(r)

			// Check output based on the expectation
			got := buf.String()
			if tt.expectOut {
				// Check if the output contains the message
				expected := "\x1b[32m" + message + "\n\x1b[0m"
				if got != expected {
					t.Errorf("Infof() = %q, want %q", got, expected)
				}
			} else {
				// Ensure no output
				if got != "" {
					t.Errorf("Infof() = %q, want no output", got)
				}
			}

			// Reset stdout and recreate pipe for the next test
			r, w, _ = os.Pipe()
			os.Stdout = w
			color.Output = os.Stdout
		})
	}
}
