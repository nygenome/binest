package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
)

// putIndexes merges all indexes if any, from both cli args and stdin and writes to chan
func putIndexes(args []string, results chan<- string) error {
	gotInput := len(args) > 0

	for _, arg := range args {
		results <- arg
	}

	if hasStdin() {
		bufScanner := bufio.NewScanner(os.Stdin)
		gotInput = true
		for bufScanner.Scan() {
			results <- bufScanner.Text()
		}
		if err := bufScanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "Error reading data from stdin")
			return err
		}
	}

	if !gotInput {
		return errors.New("No input obtained from stdin and cli args")
	}

	return nil
}

// hasStdin checks if process can read from /dev/stdin
func hasStdin() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error checking for data from stdin")
		panic(err)
	}
	if stat.Mode()&os.ModeCharDevice == 0 {
		return true
	}
	return false
}
