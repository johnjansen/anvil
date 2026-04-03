package output

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// Manager provides synchronized output writing to prevent interleaved output
// from multiple goroutines writing to stdout/stderr simultaneously.
type Manager struct {
	mu       sync.Mutex
	stdout   io.Writer
	stderr   io.Writer
	rawMode  bool
}

// globalManager is the singleton instance used throughout the application
var globalManager = &Manager{
	stdout: os.Stdout,
	stderr: os.Stderr,
}

// SetRawMode toggles raw terminal mode output. When enabled, println writes
// \r\n instead of bare \n so that lines start at column 0 when OPOST is disabled.
func SetRawMode(enabled bool) {
	globalManager.mu.Lock()
	globalManager.rawMode = enabled
	globalManager.mu.Unlock()
}

// Println writes a line to stdout with proper synchronization.
func Println(a ...interface{}) {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	line := fmt.Sprintln(a...)
	if globalManager.rawMode {
		fmt.Fprint(globalManager.stdout, line[:len(line)-1]+"\r\n")
	} else {
		fmt.Fprint(globalManager.stdout, line)
	}
}

// Printf formats according to a format specifier and writes to stdout with proper synchronization.
func Printf(format string, a ...interface{}) {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	line := fmt.Sprintf(format, a...)
	if globalManager.rawMode {
		fmt.Fprint(globalManager.stdout, line+"\r\n")
	} else {
		fmt.Fprint(globalManager.stdout, line+"\n")
	}
}

// Print writes to stdout with proper synchronization.
func Print(a ...interface{}) {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	fmt.Fprint(globalManager.stdout, a...)
}

// Fprintln writes a line to the specified writer with proper synchronization.
func Fprintln(w io.Writer, a ...interface{}) {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	fmt.Fprintln(w, a...)
}

// Fprintf formats according to a format specifier and writes to the specified writer
// with proper synchronization.
func Fprintf(w io.Writer, format string, a ...interface{}) {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	fmt.Fprintf(w, format, a...)
}

// Fprint writes to the specified writer with proper synchronization.
func Fprint(w io.Writer, a ...interface{}) {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	fmt.Fprint(w, a...)
}

// Errorln writes a line to stderr with proper synchronization.
func Errorln(a ...interface{}) {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	fmt.Fprintln(globalManager.stderr, a...)
}

// Errorf formats according to a format specifier and writes to stderr with proper synchronization.
func Errorf(format string, a ...interface{}) {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	fmt.Fprintf(globalManager.stderr, format, a...)
	fmt.Fprint(globalManager.stderr, "\n")
}