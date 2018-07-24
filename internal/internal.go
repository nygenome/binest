package internal

import "github.com/biogo/hts/bgzf"

// TileWidth is the length of the interval tiling used in BAI and tabix indexes.
const TileWidth = 0x4000

// Bin is an index bin.
type Bin struct {
	Bin    uint32
	Chunks []bgzf.Chunk
}

// ReferenceStats holds mapping statistics for a genomic reference.
type ReferenceStats struct {
	Chunk    bgzf.Chunk
	Mapped   uint64
	Unmapped uint64
}

// RefIndex is the index of a single reference.
type RefIndex struct {
	Bins      []Bin
	Stats     *ReferenceStats
	Intervals []bgzf.Offset
}
