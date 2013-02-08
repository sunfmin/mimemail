package mimemail

import (
	"bufio"
	"bytes"
	"io"
)

type LineLessReader struct {
	r   *bufio.Reader
	buf *bytes.Buffer
	err error
}

func NewLineLessReader(r io.Reader) (reader *LineLessReader) {
	reader = &LineLessReader{
		r:   bufio.NewReaderSize(r, 4096),
		buf: bytes.NewBuffer(nil),
	}
	return
}

func (nlr *LineLessReader) Read(p []byte) (n int, err error) {
	if nlr.buf.Len() >= len(p) {
		return nlr.buf.Read(p)
	}
	if nlr.err != nil {
		n, _ = nlr.buf.Read(p)
		err = nlr.err
		return
	}

	pb := make([]byte, len(p))
	_, nlr.err = nlr.r.Read(pb)
	for _, b := range pb {
		if b == '\n' || b == '\r' || b == 0 {
			continue
		}
		nlr.buf.WriteByte(b)
	}
	return nlr.Read(p)
}
