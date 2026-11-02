package dxf

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
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

// groupReader reads group code/value pairs from an ASCII DXF stream.
type groupReader struct {
	r      *bufio.Reader
	limits Limits

	line        int
	lastLine    int
	pending     *Group
	pendingLine int
}

func newGroupReader(r io.Reader, limits Limits) *groupReader {
	return &groupReader{r: bufio.NewReader(r), limits: limits}
}

func (g *groupReader) readLine() (string, error) {
	s, err := g.r.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			if s == "" {
				return "", io.EOF
			}
			g.line++
			return trimEOL(s), nil
		}
		return "", err
	}
	g.line++
	return trimEOL(s), nil
}

func trimEOL(s string) string {
	s = strings.TrimSuffix(s, "\n")
	return strings.TrimSuffix(s, "\r")
}

func (g *groupReader) read() (Group, int, error) {
	for {
		codeLine, err := g.readLine()
		if err != nil {
			return Group{}, g.line, err
		}
		if strings.TrimSpace(codeLine) == "" {
			continue
		}
		line := g.line
		code, err := strconv.Atoi(strings.TrimSpace(codeLine))
		if err != nil {
			return Group{}, line, &SyntaxError{Line: line, Reason: fmt.Sprintf("invalid group code: %q", strings.TrimSpace(codeLine))}
		}
		if !IsValidCode(code) {
			return Group{}, line, &SyntaxError{Line: line, Code: code, Reason: "group code outside the valid range (-5..1071)"}
		}
		value, err := g.readLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return Group{}, line, &SyntaxError{Line: line, Code: code, Reason: "incomplete group code/value pair"}
			}
			return Group{}, line, err
		}
		if len(value) > g.limits.MaxValueBytes {
			return Group{}, line, &LimitError{Kind: "value", Limit: g.limits.MaxValueBytes, Line: line}
		}
		return Group{Code: code, Value: value}, line, nil
	}
}

// next returns the next group. It returns io.EOF at a clean end of stream.
func (g *groupReader) next() (Group, error) {
	if g.pending != nil {
		group := *g.pending
		g.lastLine = g.pendingLine
		g.pending = nil
		return group, nil
	}
	group, line, err := g.read()
	if err != nil {
		return Group{}, err
	}
	g.lastLine = line
	return group, nil
}

// peek returns the next group without consuming it. ok is false at end of stream.
func (g *groupReader) peek() (Group, bool, error) {
	if g.pending != nil {
		return *g.pending, true, nil
	}
	group, line, err := g.read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Group{}, false, nil
		}
		return Group{}, false, err
	}
	g.pending = &group
	g.pendingLine = line
	return group, true, nil
}
