package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/omicsnut/binest"
)

func main() {
	var mode string
	cmds := []string{"copy", "size", "sex"}

	if len(os.Args) == 1 {
		fmt.Fprintf(os.Stderr, "binest %s Available subcommands: %s\n", binest.Version, strings.Join(cmds, ", "))
		os.Exit(0)
	}

	mode = os.Args[1]
	os.Args[0] += fmt.Sprintf(" %s", mode)
	os.Args = append(os.Args[:1], os.Args[2:]...)

	switch mode {
	case "copy":
		runCopy()
	case "size":
		runSize()
	case "sex":
		runSex()
	default:
		msg := fmt.Sprintf("%s not a valid command!\n", strings.Join(os.Args, " "))
		msg += fmt.Sprintf("Available subcommands: %s\n", strings.Join(cmds, ", "))
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}
