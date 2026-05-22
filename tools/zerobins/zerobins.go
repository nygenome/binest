package main

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"errors"
	"os"

	"github.com/nygenome/binest"
)

func main() {
	fh, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}

	var d binest.ZeroBins
	dec := json.NewDecoder(bufio.NewReader(fh))
	err = dec.Decode(&d)
	closeErr := fh.Close()
	if err != nil {
		panic(errors.Join(err, closeErr))
	}
	if closeErr != nil {
		panic(closeErr)
	}

	outFh, err := os.OpenFile("refbins.zeros", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}

	enc := gob.NewEncoder(outFh)
	err = enc.Encode(d)
	closeErr = outFh.Close()
	if err != nil {
		panic(errors.Join(err, closeErr))
	}
	if closeErr != nil {
		panic(closeErr)
	}
}
