package dxf

import (
	"io"

	"github.com/apantanella/dxf/tags"
)

// Limits bounds the resources used while decoding an untrusted file.
type Limits struct {
	// MaxEntities is the maximum number of entities in the ENTITIES and BLOCKS
	// sections.
	MaxEntities int
	// MaxBlocks is the maximum number of block definitions.
	MaxBlocks int
	// MaxValueBytes is the maximum length of a single group value.
	MaxValueBytes int
	// MaxNestingDepth is reserved for nested structures.
	MaxNestingDepth int
}

// DefaultLimits returns the limits documented in SECURITY.md.
func DefaultLimits() Limits {
	return Limits{
		MaxEntities:     1_000_000,
		MaxBlocks:       100_000,
		MaxValueBytes:   1 << 20,
		MaxNestingDepth: 64,
	}
}

// groupReader adapts tags.Reader to the parser: it keeps the line of the last
// group returned, which the parser uses in its diagnostics.
type groupReader struct {
	tr       *tags.Reader
	lastLine int
}

func newGroupReader(r io.Reader, limits Limits) *groupReader {
	return &groupReader{tr: tags.NewReader(r, tags.WithMaxValueBytes(limits.MaxValueBytes))}
}

// next returns the next group. It returns io.EOF at a clean end of stream.
func (g *groupReader) next() (Group, error) {
	group, err := g.tr.Next()
	if err != nil {
		return Group{}, err
	}
	g.lastLine = g.tr.Line()
	return group, nil
}

// peek returns the next group without consuming it. ok is false at end of stream.
func (g *groupReader) peek() (Group, bool, error) { return g.tr.Peek() }

// nonASCII returns the number of values with bytes of 0x80 or more and the
// line of the first one.
func (g *groupReader) nonASCII() (count, firstLine int) { return g.tr.NonASCII() }
