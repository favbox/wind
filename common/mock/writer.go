package mock

import "bytes"

type ExtWriter struct {
	tmp     []byte
	Buf     *bytes.Buffer
	IsFinal *bool
}

func (m *ExtWriter) Write(p []byte) (n int, err error) {
	m.tmp = p
	return len(p), nil
}

func (m *ExtWriter) Flush() error {
	_, err := m.Buf.Write(m.tmp)
	return err
}

func (m *ExtWriter) Finalize() error {
	if !*m.IsFinal {
		*m.IsFinal = true
	}
	return nil
}

func (m *ExtWriter) SetBody(body []byte) {
	m.Buf.Reset()
	m.tmp = body
}
