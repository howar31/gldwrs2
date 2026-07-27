package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type Mode int

const (
	ModeConcise Mode = iota
	ModeRaw
	ModeJSON
)

// Render writes raw JSON in the chosen mode. In ModeConcise, the pre-rendered
// concise string is used; an empty concise falls back to pretty JSON.
func Render(w io.Writer, raw json.RawMessage, mode Mode, concise string) error {
	switch mode {
	case ModeRaw:
		_, err := w.Write(raw)
		return err
	case ModeJSON:
		return writePretty(w, raw)
	default:
		if concise != "" {
			_, err := fmt.Fprintln(w, concise)
			return err
		}
		return writePretty(w, raw)
	}
}

func writePretty(w io.Writer, raw json.RawMessage) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		// not valid JSON object/array; emit as-is
		_, err2 := w.Write(raw)
		return err2
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}
