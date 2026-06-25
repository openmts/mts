package main

import (
	"context"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(runMain(os.Args, os.Stdout, os.Stderr))
}

func runMain(args []string, stdout io.Writer, stderr io.Writer) int {
	app := newApp(stdout, stderr)
	if err := app.RunContext(context.Background(), args); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
