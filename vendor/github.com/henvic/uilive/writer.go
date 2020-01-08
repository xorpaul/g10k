package uilive

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sync"
)

// ESC is the ASCII code for escape character
const ESC = 27

// Out is the default output writer for the Writer
var Out = os.Stdout

// ErrClosedPipe is the error returned when trying to writer is not listening
var ErrClosedPipe = errors.New("uilive: read/write on closed pipe")

// FdWriter is a writer with a file descriptor.
type FdWriter interface {
	io.Writer
	Fd() uintptr
}

// Writer is a buffered the writer that updates the terminal. The contents of writer will be flushed on a timed interval or when Flush is called.
type Writer struct {
	// Out is the writer to write to
	Out io.Writer

	buf       bytes.Buffer
	mtx       *sync.Mutex
	lineCount int
}

type bypass struct {
	writer *Writer
}

// New returns a new Writer with defaults
func New() *Writer {
	return &Writer{
		Out: Out,
		mtx: &sync.Mutex{},
	}
}

// Flush writes to the out and resets the buffer. It should be called after the last call to Write to ensure that any data buffered in the Writer is written to output.
// Any incomplete escape sequence at the end is considered complete for formatting purposes.
// An error is returned if the contents of the buffer cannot be written to the underlying output stream
func (w *Writer) Flush() error {
	w.mtx.Lock()
	defer w.mtx.Unlock()

	// do nothing is  buffer is empty
	if len(w.buf.Bytes()) == 0 {
		return nil
	}
	w.clearLines()

	lines := 0
	for _, b := range w.buf.Bytes() {
		if b == '\n' {
			lines++
		}
	}
	w.lineCount = lines
	_, err := w.Out.Write(w.buf.Bytes())
	w.buf.Reset()
	return err
}

// Write save the contents of b to its buffers. The only errors returned are ones encountered while writing to the underlying buffer.
func (w *Writer) Write(b []byte) (n int, err error) {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	return w.buf.Write(b)
}

// Bypass creates an io.Writer which allows non-buffered output to be written to the underlying output
func (w *Writer) Bypass() io.Writer {
	return &bypass{writer: w}
}

func (b *bypass) Write(p []byte) (n int, err error) {
	b.writer.mtx.Lock()
	defer b.writer.mtx.Unlock()

	b.writer.clearLines()
	b.writer.lineCount = 0
	return b.writer.Out.Write(p)
}
