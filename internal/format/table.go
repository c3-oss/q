package format

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/c3-oss/q/internal/adapter"
)

type tableFormatter struct {
	tw     *tabwriter.Writer
	warn   io.Writer
	index  map[string]int
	header []string
	warned bool
}

func newTable(out, warn io.Writer) *tableFormatter {
	return &tableFormatter{
		tw:   tabwriter.NewWriter(out, 0, 0, 2, ' ', 0),
		warn: warn,
	}
}

func (f *tableFormatter) Write(rec adapter.Record) error {
	if f.header == nil {
		f.header = make([]string, len(rec))
		f.index = make(map[string]int, len(rec))
		for i, fld := range rec {
			f.header[i] = fld.Name
			f.index[fld.Name] = i
		}
		fmt.Fprintln(f.tw, strings.Join(f.header, "\t"))
	}

	row := make([]string, len(f.header))
	extra := false
	for _, fld := range rec {
		i, ok := f.index[fld.Name]
		if !ok {
			extra = true
			continue
		}
		s, err := cell(fld.Value)
		if err != nil {
			return err
		}
		row[i] = sanitizeCell(s)
	}
	if extra && !f.warned {
		fmt.Fprintln(f.warn, "q: warning: a record has fields absent from the first record; extra fields dropped")
		f.warned = true
	}
	fmt.Fprintln(f.tw, strings.Join(row, "\t"))
	return nil
}

func (f *tableFormatter) Close() error {
	return f.tw.Flush()
}

// sanitizeCell keeps tabs and newlines from breaking column alignment.
func sanitizeCell(s string) string {
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
}
