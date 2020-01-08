# uilive [![GoDoc](https://godoc.org/github.com/henvic/uilive?status.svg)](https://godoc.org/github.com/henvic/uilive) [![Build Status](https://travis-ci.org/henvic/uilive.svg?branch=master)](https://travis-ci.org/henvic/uilive)

uilive is a go library for updating terminal output in realtime. It provides a buffered [io.Writer](https://golang.org/pkg/io/#Writer) that you can flush when it is time.

**This is an API-incompatible modified fork** ([original](https://github.com/gosuri/uilive)) that removes the timed interval ticker, so that you have full control of flushing lines to terminal.

## Usage Example

Calling `uilive.New()` will create a new writer. To start rendering, simply call `writer.Start()` and update the ui by writing to the `writer`. Full source for the below example is in [example/main.go](example/main.go).

```go
writer := uilive.New()

for _, f := range []string{"Foo.zip", "Bar.iso"} {
    for i := 0; i <= 50; i++ {
        fmt.Fprintf(writer, "Downloading %s.. (%d/%d) GB\n", f, i, 50)
        writer.Flush()
        time.Sleep(time.Millisecond * 70)
    }

    fmt.Fprintf(writer.Bypass(), "Downloaded %s\n", f)
}

fmt.Fprintln(writer, "Finished: Downloaded 100GB")
```

[![asciicast](https://asciinema.org/a/9lo78nlgj1q9jptd9ovbt5okw.png)](https://asciinema.org/a/9lo78nlgj1q9jptd9ovbt5okw)
