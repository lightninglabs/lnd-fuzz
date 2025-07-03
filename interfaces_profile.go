package lndfuzz

import "io"

// CoverageProfileReader reads and parses coverage profiles.
type CoverageProfileReader interface {
	// ReadProfile reads a coverage profile from the given reader.
	ReadProfile(r io.Reader) (map[string]int, error)
}