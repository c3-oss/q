package format

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/c3-oss/q/internal/adapter"
)

type csvFormatter struct {
	w      *csv.Writer
	warn   io.Writer
	index  map[string]int
	header []string
	warned bool
}

func newCSV(out, warn io.Writer) *csvFormatter {
	return &csvFormatter{w: csv.NewWriter(out), warn: warn}
}

func (f *csvFormatter) Write(rec adapter.Record) error {
	if f.header == nil {
		f.header = make([]string, len(rec))
		f.index = make(map[string]int, len(rec))
		for i, fld := range rec {
			f.header[i] = fld.Name
			f.index[fld.Name] = i
		}
		if err := f.w.Write(f.header); err != nil {
			return err
		}
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
		row[i] = s
	}
	f.maybeWarn(extra)
	return f.w.Write(row)
}

func (f *csvFormatter) maybeWarn(extra bool) {
	if extra && !f.warned {
		fmt.Fprintln(f.warn, "q: warning: a record has fields absent from the first record; extra fields dropped")
		f.warned = true
	}
}

func (f *csvFormatter) Close() error {
	f.w.Flush()
	return f.w.Error()
}
