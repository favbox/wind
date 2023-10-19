package sse

import (
	"io"
	"strconv"
	"strings"

	"github.com/favbox/wind/internal/bytesconv"
)

var fieldReplacer = strings.NewReplacer(
	"\n", "\\n",
	"\r", "\\r")

var dataReplacer = strings.NewReplacer(
	"\n", "\ndata:",
	"\r", "\\r")

func Encode(w io.Writer, e *Event) (err error) {
	err = writeID(w, e.ID)
	if err != nil {
		return
	}
	err = writeEvent(w, e.Event)
	if err != nil {
		return
	}
	err = writeRetry(w, e.Retry)
	if err != nil {
		return
	}
	err = writeData(w, e.Data)
	if err != nil {
		return
	}
	return nil
}

func writeID(w io.Writer, id string) (err error) {
	if len(id) > 0 {
		_, err = w.Write([]byte("id:"))
		if err != nil {
			return
		}
		_, err = fieldReplacer.WriteString(w, id)
		if err != nil {
			return
		}
		_, err = w.Write([]byte("\n"))
		if err != nil {
			return
		}
	}

	return
}

func writeEvent(w io.Writer, event string) (err error) {
	if len(event) > 0 {
		_, err = w.Write([]byte("event:"))
		if err != nil {
			return
		}
		_, err = fieldReplacer.WriteString(w, event)
		if err != nil {
			return
		}
		_, err = w.Write([]byte("\n"))
		if err != nil {
			return
		}
	}

	return
}

func writeRetry(w io.Writer, retry uint64) (err error) {
	if retry > 0 {
		_, err = w.Write([]byte("retry:"))
		if err != nil {
			return
		}
		_, err = w.Write(bytesconv.S2b(strconv.FormatUint(retry, 10)))
		if err != nil {
			return
		}
		_, err = w.Write([]byte("\n"))
		if err != nil {
			return
		}
	}

	return
}

func writeData(w io.Writer, data []byte) (err error) {
	_, err = w.Write([]byte("data:"))
	if err != nil {
		return
	}
	_, err = dataReplacer.WriteString(w, bytesconv.B2s(data))
	if err != nil {
		return
	}
	_, err = w.Write([]byte("\n\n"))
	if err != nil {
		return
	}

	return nil
}
