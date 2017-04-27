package main

import (
	"bufio"
	"fmt"
	"os"
)

// readIndexes merges all indexes if any, from both cli args and stdin
func readIndexes(allArgs [3][]string) ([]string, bool) {
	indexes := make([]string, 0, 100)

	for _, cmdArgs := range allArgs {
		indexes = append(indexes, cmdArgs...)
	}

	if hasStdin() {
		bufScanner := bufio.NewScanner(os.Stdin)
		for bufScanner.Scan() {
			indexes = append(indexes, bufScanner.Text())
		}
		if err := bufScanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "Error reading data from stdin")
			panic(err)
		}
	}

	return indexes, len(indexes) > 0
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
