package model

import (
	"bufio"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/biogo/hts/bgzf"
	"github.com/gonum/stat/distuv"
	"github.com/pkg/errors"
)

// RefBlock represents the reference block for a bin
type RefBlock struct {
	Ref   string
	Start uint32
	End   uint32
}

// models holds all the available models
var models = loadModels()

// loadModels loads all available bin models
func loadModels() map[string]map[RefBlock]distuv.Gamma {
	matches, _ := filepath.Glob("*.model.gz")

	builtModels := make(map[string]map[RefBlock]distuv.Gamma, len(matches))

	for _, match := range matches {
		fh, err := os.Open(match)
		if err != nil {
			panic(err)
		}

		modelName := strings.TrimSuffix(filepath.Base(match), ".model.gz")
		modelMap := make(map[RefBlock]distuv.Gamma, 200000)

		bgzfRdr, err := bgzf.NewReader(fh, runtime.GOMAXPROCS(0))
		if err != nil {
			panic(err)
		}

		scanner := bufio.NewScanner(bgzfRdr)

		for scanner.Scan() {
			parseModelLine(scanner.Text(), &modelMap)
		}
		if err = scanner.Err(); err != nil {
			panic(err)
		}

		builtModels[modelName] = modelMap
		bgzfRdr.Close()
		fh.Close()
	}

	return builtModels
}

// parseModelLine parses a single line in a bin model
func parseModelLine(line string, modelMap *map[RefBlock]distuv.Gamma) {
	items := strings.Split(line, "\t")
	chrom, startStr, endStr, alphaStr, betaStr := items[0], items[1], items[2], items[3], items[4]

	start, err := strconv.Atoi(startStr)
	if err != nil {
		panic(errors.Wrapf(err, "Error parsing model start: %s", line))
	}

	end, err := strconv.Atoi(endStr)
	if err != nil {
		panic(errors.Wrapf(err, "Error parsing model end: %s", line))
	}

	alpha, err := strconv.ParseFloat(alphaStr, 64)
	if err != nil {
		panic(errors.Wrapf(err, "Error parsing model alpha: %s", line))
	}

	beta, err := strconv.ParseFloat(betaStr, 64)
	if err != nil {
		panic(errors.Wrapf(err, "Error parsing model beta: %s", line))
	}

	block := RefBlock{Ref: chrom, Start: uint32(start), End: uint32(end)}
	model := distuv.Gamma{
		Alpha:  alpha,
		Beta:   beta,
		Source: rand.New(rand.NewSource(int64(rand.Int()))),
	}

	(*modelMap)[block] = model
}
