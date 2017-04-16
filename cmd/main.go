package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/omicsnut/binest"
	"github.com/omicsnut/binest/chunk"
	"github.com/omicsnut/binest/cnv"
	"github.com/omicsnut/binest/cov"
	"github.com/omicsnut/binest/sex"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Fprintf(os.Stderr, "binest %s Available subcommands: chunk, cnv, cov and sex\n", binest.Version)
		os.Exit(0)
	}

	var mode string
	if len(os.Args) > 1 {
		mode = os.Args[1]
		os.Args[0] += fmt.Sprintf(" %s", mode)
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}

	switch mode {
	case "chunk":
		chunk.Run()
	case "cnv":
		cnv.Run()
	case "cov":
		cov.Run()
	case "sex":
		sex.Run()
	default:
		msg := fmt.Sprintf("%s not a valid command!\n", strings.Join(os.Args, " "))
		msg += "Available subcommands: chunk, cnv, cov and sex"
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}
