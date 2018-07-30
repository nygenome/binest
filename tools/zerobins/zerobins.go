package main

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"os"

	"git.nygenome.org/rmusunuri/binest"
)

func main() {
	fh, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer fh.Close()

	var d binest.ZeroBins
	dec := json.NewDecoder(bufio.NewReader(fh))
	err = dec.Decode(&d)
	if err != nil {
		panic(err)
	}

	outFh, err := os.OpenFile("refbins.zeros", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer outFh.Close()
	enc := gob.NewEncoder(outFh)
	err = enc.Encode(d)
	if err != nil {
		panic(err)
	}
}
