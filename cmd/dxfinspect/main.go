// Command dxfinspect prints a summary of DXF R12 files: version, sections,
// blocks, layers, entity counts per type and diagnostics.
//
// Usage:
//
//	dxfinspect [-lenient] file.dxf [file.dxf...]
//
// The exit status is 0 when every file is decoded, 1 when a file fails and 2 on
// invalid arguments.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/apantanella/dxf"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("dxfinspect", flag.ContinueOnError)
	flags.SetOutput(stderr)
	lenient := flags.Bool("lenient", false, "collect recoverable anomalies as diagnostics instead of failing")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: dxfinspect [-lenient] file.dxf [file.dxf...]")
		return 2
	}

	status := 0
	for _, path := range flags.Args() {
		if err := inspect(path, *lenient, stdout); err != nil {
			fmt.Fprintf(stderr, "dxfinspect: %s: %v\n", path, err)
			status = 1
		}
	}
	return status
}

func inspect(path string, lenient bool, out io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var doc *dxf.Document
	if lenient {
		doc, err = dxf.Decode(file, dxf.WithLenient())
	} else {
		doc, err = dxf.Decode(file)
	}
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "%s\n", path)
	fmt.Fprintf(out, "  version: %s\n", doc.Version)
	fmt.Fprintf(out, "  blocks: %d\n", len(doc.Blocks))
	fmt.Fprintf(out, "  layers: %d\n", len(doc.Tables.LayerNames()))
	fmt.Fprintf(out, "  entities: %d\n", len(doc.Entities))
	if len(doc.UnknownSections) > 0 {
		names := make([]string, 0, len(doc.UnknownSections))
		for _, section := range doc.UnknownSections {
			names = append(names, section.Name)
		}
		fmt.Fprintf(out, "  unknown sections: %s\n", strings.Join(names, ", "))
	}

	counts := make(map[string]int, len(doc.Entities))
	for _, entity := range doc.Entities {
		counts[entity.Type()]++
	}
	for _, entityType := range sortedKeys(counts) {
		fmt.Fprintf(out, "    %-10s %d\n", entityType, counts[entityType])
	}

	if len(doc.Diagnostics) > 0 {
		fmt.Fprintf(out, "  diagnostics: %d\n", len(doc.Diagnostics))
		for _, diagnostic := range doc.Diagnostics {
			fmt.Fprintf(out, "    %s\n", diagnostic)
		}
	}
	return nil
}

func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
