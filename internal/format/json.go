package format

import (
	"bytes"
	"io"

	"github.com/c3-oss/q/internal/adapter"
)

type jsonFormatter struct {
	w     io.Writer
	open  bool
	wrote bool
}

func newJSON(out io.Writer) *jsonFormatter {
	return &jsonFormatter{w: out}
}

// Write emits one object, building it field-by-field so field order is
// preserved (a map would not). Each element is assembled in a buffer and
// written once.
func (f *jsonFormatter) Write(rec adapter.Record) error {
	var b bytes.Buffer
	if !f.open {
		b.WriteByte('[')
		f.open = true
	}
	if f.wrote {
		b.WriteString(",\n")
	}

	b.WriteByte('{')
	for i, fld := range rec {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := marshalNoHTML(fld.Name)
		if err != nil {
			return err
		}
		b.Write(key)
		b.WriteByte(':')
		val, err := jsonValue(fld.Value)
		if err != nil {
			return err
		}
		b.Write(val)
	}
	b.WriteByte('}')

	_, err := f.w.Write(b.Bytes())
	f.wrote = true
	return err
}

func (f *jsonFormatter) Close() error {
	if !f.open {
		_, err := io.WriteString(f.w, "[]\n")
		return err
	}
	_, err := io.WriteString(f.w, "]\n")
	return err
}
