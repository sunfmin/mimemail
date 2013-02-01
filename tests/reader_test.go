package mimemail

import (
	"bytes"
	// "fmt"
	"github.com/sunfmin/mimemail"
	// "io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

type Case struct {
	Input  string
	Output string
}

var cases = []Case{
	{"=?utf-8?B?SGVsbG8gUERGIOWKoOeCueS4reaWh+WSjOaXpeacrOiqnuOBig==?=\r\n=?utf-8?B?44Gv44KI44GG44GU44GW44GE44G+44GZ?=", `Hello PDF 加点中文和日本語おはようございます`},
	{"=?iso-8859-1?q?J=F6rg_Doe?= <joerg@example.com>", "Jörg Doe <joerg@example.com>"},
	{"=?ISO-8859-1?Q?Andr=E9?= Pirard <PIRARD@vm1.ulg.ac.be>", "André Pirard <PIRARD@vm1.ulg.ac.be>"},
}

func TestReader(t *testing.T) {
	for _, c := range cases {
		var err error
		var b []byte
		ir := strings.NewReader(c.Input)

		r := mimemail.NewRFC2047Reader(ir, nil)
		b, err = ioutil.ReadAll(r)
		if err != nil {
			t.Error(err)
		}
		if string(b) != c.Output {
			t.Errorf("expected: %s, but was: %s", c.Output, string(b))
		}
	}
}

func TestISO_8859_1(t *testing.T) {

	var b2 []byte

	f, err := os.Open("iso_8859_1_raw.txt")

	b2, err = ioutil.ReadAll(mimemail.NewISO_8859_1(f))
	if err != nil {
		panic(err)
	}

	expected := []byte("Jörg Doe <joerg@example.com>")

	if bytes.Compare(expected, b2) != 0 {
		t.Errorf("expected: %v, but was: %v", expected, b2)
	}

	// fmt.Println("ISO_8859_1", b)
	// fmt.Println("UTF8      ", []byte("Jörg Doe <joerg@example.com>"))
	// fmt.Println("CONVERTED ", b2)

}
