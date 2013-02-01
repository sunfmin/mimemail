package mimemail

import (
	"bytes"
	"fmt"
	"io"
)

type UTF8ReaderFactory interface {
	UTF8Reader(charset string, body io.Reader) (r io.Reader, err error)
}

type DefaultUTF8ReaderFactory struct {
}

func (dc *DefaultUTF8ReaderFactory) UTF8Reader(charset string, body io.Reader) (r io.Reader, err error) {
	switch charset {
	case "iso-8859-1":
		r = NewISO_8859_1(body)
	case "utf-8", "us-ascii", "ascii", "":
		r = body
	default:
		err = fmt.Errorf("charset %s not supported", charset)
	}
	return
}

type ISO_8859_1 struct {
	br  io.Reader
	buf *bytes.Buffer
	err error
}

func NewISO_8859_1(r io.Reader) *ISO_8859_1 {
	return &ISO_8859_1{br: r, buf: bytes.NewBuffer(nil)}
}

func (i8859 *ISO_8859_1) Read(p []byte) (n int, err error) {
	l := len(p)

	if i8859.buf.Len() >= l {
		return i8859.buf.Read(p)
	}

	if i8859.err != nil {
		err = i8859.err
		n, _ = i8859.buf.Read(p)
		return
	}

	peek := make([]byte, 256)
	var pn int
	pn, err = i8859.br.Read(peek)
	if err != nil {
		i8859.err = err
	}

	for i := 0; i < pn; i++ {
		c := peek[i]
		i8859.buf.WriteRune(rune(c))
	}

	return i8859.Read(p)
}
