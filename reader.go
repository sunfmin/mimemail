package mimemail

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
)

const (
	raw int = iota
	quoteStarting
	quoteEnding
	quoteBody
)

// var (
// 	CorruptedEncodingError = errors.New("corrupted encoding error")
// )

var (
	encodedWordStart = []byte("=?")
	encodedWordEnd   = []byte("?=")
)

func DecodeText(text string, utf8ReaderFactory UTF8ReaderFactory) (decoded string, err error) {
	r := NewRFC2047Reader(strings.NewReader(text), utf8ReaderFactory)
	var b []byte
	b, err = ioutil.ReadAll(r)
	decoded = string(b)
	return
}

type RFC2047Reader struct {
	br                *bufio.Reader
	state             int
	utf8ReaderFactory UTF8ReaderFactory
	buf               *bytes.Buffer
	err               error
	bodyReader        io.Reader
	charsetBytes      []byte
	encodingBytes     []byte
}

func NewRFC2047Reader(r io.Reader, utf8ReaderFactory UTF8ReaderFactory) *RFC2047Reader {
	if utf8ReaderFactory == nil {
		utf8ReaderFactory = &DefaultUTF8ReaderFactory{}
	}

	return &RFC2047Reader{
		br:                bufio.NewReaderSize(r, 256),
		state:             raw,
		utf8ReaderFactory: utf8ReaderFactory,
		buf:               bytes.NewBuffer(nil),
	}
}

// =?utf-8?B?SGVsbG8gUERGIOWKoOeCueS4reaWh+WSjOaXpeacrOiqnuOBig==?=
// =?utf-8?B?44Gv44KI44GG44GU44GW44GE44G+44GZ?=
func (rr *RFC2047Reader) Read(p []byte) (n int, err error) {

	// fmt.Println(rr.state)

	l := len(p)

	if l == 0 {
		return 0, nil
	}

	if rr.buf.Len() > l {
		return rr.buf.Read(p)
	}

	// underline reader have error, but still have buffer
	if rr.err != nil && rr.buf.Len() < l {
		n, _ = rr.buf.Read(p)
		err = rr.err
		return
	}

	peek, _ := rr.br.Peek(256)

	if len(peek) == 0 {
		return rr.setErrAndReadLeft(io.EOF, p)
	}

	// spaces newlines between two encoded-word are discarded
	// but spaces and newlines between encoded-word and raw word are not

	if rr.state == raw {

		startIndex := bytes.Index(peek, encodedWordStart)
		nCopy := 0
		if startIndex == -1 {
			nCopy = len(peek)
		} else {
			nCopy = startIndex
			rr.state = quoteStarting
		}

		if strings.Trim(string(peek[0:nCopy]), "\r\n ") != "" {
			if _, err = io.CopyN(rr.buf, rr.br, int64(nCopy)); err != nil {
				return rr.setErrAndReadLeft(err, p)
			}
		} else {
			discard := make([]byte, nCopy)
			rr.br.Read(discard)
		}

		return rr.Read(p)

	}

	if rr.state == quoteEnding {
		drop := make([]byte, 2)
		// drop "?="
		if _, err = rr.br.Read(drop); err != nil {
			return rr.setErrAndReadLeft(err, p)
		}
		rr.state = raw
		return rr.Read(p)
	}

	if rr.state == quoteBody {
		endIndex := bytes.Index(peek, encodedWordEnd)
		nCopy := 0
		if endIndex == -1 {
			nCopy = len(peek)
		} else {
			nCopy = endIndex
			rr.state = quoteEnding
		}

		rr.bodyReader, err = bodyReader(rr.charsetBytes, rr.encodingBytes, io.LimitReader(rr.br, int64(nCopy)), rr.utf8ReaderFactory, true)
		if err != nil {
			return rr.setErrAndReadLeft(err, p)
		}

		if _, err = io.Copy(rr.buf, rr.bodyReader); err != nil && err != io.EOF {
			return rr.setErrAndReadLeft(err, p)
		}

		return rr.Read(p)
	}

	if rr.state == quoteStarting {
		// drop "=?"
		drop := make([]byte, 2)
		rr.br.Read(drop)

		rr.charsetBytes, err = rr.br.ReadBytes('?')
		if err != nil {
			return rr.setErrAndReadLeft(err, p)
		}
		rr.charsetBytes = rr.charsetBytes[0 : len(rr.charsetBytes)-1] // cut ?

		rr.encodingBytes, err = rr.br.ReadBytes('?')
		if err != nil {
			return rr.setErrAndReadLeft(err, p)
		}
		rr.encodingBytes = rr.encodingBytes[0 : len(rr.encodingBytes)-1] // cut ?

		rr.state = quoteBody
		return rr.Read(p)
	}
	return
}

func (rr *RFC2047Reader) setErrAndReadLeft(e error, p []byte) (n int, err error) {
	rr.err = e
	return rr.Read(p)
}

func BodyReader(charset string, encoding string, r io.Reader, utf8ReaderFactory UTF8ReaderFactory) (br io.Reader, err error) {
	if utf8ReaderFactory == nil {
		utf8ReaderFactory = &DefaultUTF8ReaderFactory{}
	}
	return bodyReader([]byte(charset), []byte(encoding), r, utf8ReaderFactory, false)
}

func bodyReader(charsetBytes []byte, encBytes []byte, r io.Reader, utf8ReaderFactory UTF8ReaderFactory, isHeader bool) (br io.Reader, err error) {

	encoding := strings.ToLower(string(encBytes))
	charset := strings.ToLower(string(charsetBytes))

	switch encoding {
	case "q", "quoted-printable":
		br = NewQDecoder(r, isHeader)
	case "b", "base64":
		br = base64.NewDecoder(base64.StdEncoding, NewLineLessReader(r))
	default:
		br = r
	}
	br, err = utf8ReaderFactory.UTF8Reader(charset, br)
	return
}

type QDecoder struct {
	r        *bufio.Reader
	buf      *bytes.Buffer
	err      error
	isEql    bool
	eqlCode  []byte
	IsHeader bool
}

func NewQDecoder(r io.Reader, isHeader bool) (rd *QDecoder) {
	rd = &QDecoder{r: bufio.NewReader(r), buf: bytes.NewBuffer(nil), IsHeader: isHeader}
	return
}

func (qd *QDecoder) reset() {
	qd.isEql = false
	qd.eqlCode = nil
}

func (qd *QDecoder) Read(p []byte) (n int, err error) {
	// This method writes at most one byte into p.
	if len(p) == 0 {
		return 0, nil
	}

	if qd.buf.Len() >= len(p) {
		return qd.buf.Read(p)
	}

	if qd.err != nil {
		n, _ = qd.buf.Read(p)
		err = qd.err
		return
	}

	readBytes := make([]byte, 512)
	n, qd.err = qd.r.Read(readBytes)
	for i := 0; i < n; i++ {
		c := readBytes[i]
		if qd.isEql {
			if c == '\n' || c == '\r' {
				qd.reset()
				continue
			}
			if len(qd.eqlCode) < 2 {
				qd.eqlCode = append(qd.eqlCode, c)
			}
			if len(qd.eqlCode) == 2 {
				x, err := strconv.ParseInt(string(qd.eqlCode), 16, 64)
				if err != nil {
					return 0, fmt.Errorf("mail: invalid RFC 2047 encoding: %q", qd.eqlCode)
				}
				qd.buf.WriteByte(byte(x))
				qd.reset()
			}
			continue
		}

		switch c {
		case '=':
			qd.isEql = true
		case '_':
			if qd.IsHeader {
				qd.buf.WriteByte(byte(' '))
			} else {
				qd.buf.WriteByte('_')
			}
			qd.reset()
		case '\n', '\r':
			if !qd.IsHeader {
				qd.buf.WriteByte(c)
			}
			qd.reset()
		default:
			qd.buf.WriteByte(c)
			qd.reset()
		}
	}

	return qd.Read(p)
}
