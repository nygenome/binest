package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/omicsnut/binest"
	"github.com/omicsnut/binest/copy"
	"github.com/omicsnut/binest/sex"
	"github.com/omicsnut/binest/size"
)

func main() {
	var mode string
	cmds := []string{"copy", "size", "sex"}

	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "binest %s Available subcommands: %s\n", binest.Version, strings.Join(cmds, ", "))
		os.Exit(0)
	} else {
		mode = os.Args[1]
		os.Args[0] += fmt.Sprintf(" %s", mode)
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}

	switch mode {
	case "copy":
		copy.Run()
	case "size":
		size.Run()
	case "sex":
		sex.Run()
	default:
		msg := fmt.Sprintf("%s not a valid command!\n", strings.Join(os.Args, " "))
		msg += fmt.Sprintf("Available subcommands: %s\n", strings.Join(cmds, ", "))
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}
