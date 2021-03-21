package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
	"unicode/utf8"
)

type errs []string

type varset map[string]string

type source struct {
	r           io.Reader
	pos0, pos   int
	eof         int
	line0, line int
	col0, col   int
	lit         int
	buf         [4096]byte
	errh        func(int, int, string)
}

type scanner struct {
	*source

	eof bool
	key string
	val string
}

func isLetter(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' || r == '_' || r == '-'
}

func newSource(r io.Reader, errh func(int, int, string)) *source {
	return &source{
		r:    r,
		pos:  0,
		line: 1,
		errh: errh,
	}
}

func newScanner(src *source) *scanner {
	sc := &scanner{
		source: src,
	}
	sc.next()
	return sc
}

func (s *source) err(msg string) {
	s.errh(s.line, s.col, msg)
}

func (s *source) get() rune {
redo:
	s.pos0, s.line0, s.col0 = s.pos, s.line, s.col

	if s.pos == 0 || s.pos >= len(s.buf) {
		n, err := s.r.Read(s.buf[0:])

		if err != nil {
			if err != io.EOF {
				s.err("io error: " + err.Error())
			}
			return -1
		}

		s.pos = 0
		s.eof = n
	}

	if s.pos == s.eof {
		return -1
	}

	b := s.buf[s.pos]

	if b >= utf8.RuneSelf {
		r, w := utf8.DecodeRune(s.buf[s.pos:])

		s.pos += w
		s.col += w

		return r
	}

	s.pos++
	s.col++

	if b == 0 {
		s.err("invalid NUL byte")
		goto redo
	}

	if b == '\n' {
		s.line++
		s.col = 0
	}
	return rune(b)
}

func (s *source) unget() { s.pos, s.line, s.col = s.pos0, s.line0, s.col0 }

func (s *source) startLit() { s.lit = s.pos0 }

func (s *source) stopLit() string {
	if s.lit < 0 {
		panic("negative literal position")
	}

	lit := s.buf[s.lit:s.pos]
	s.lit = -1
	return string(lit)
}

func (sc *scanner) skipline() {
	r := sc.get()

	for r != '\n' {
		r = sc.get()
	}
	sc.unget()
}

func (sc *scanner) scankey() {
	sc.startLit()

	r := sc.get()

	for isLetter(r) {
		r = sc.get()
	}
	sc.unget()

	sc.key = sc.stopLit()
}

func (sc *scanner) scanval() {
	r := sc.get()

	for r == ' ' || r == '\t' {
		r = sc.get()
	}

	sc.startLit()

	for r != '\n' {
		r = sc.get()
	}
	sc.unget()

	sc.val = sc.stopLit()
}

func (sc *scanner) next() {
redo:
	sc.key = sc.key[0:0]
	sc.val = sc.val[0:0]

	r := sc.get()

	for r == ' ' || r == '\t' || r == '\r' || r == '\n' {
		r = sc.get()
	}

	if r == -1 {
		sc.eof = true
		return
	}

	if r == '#' {
		sc.skipline()
		goto redo
	}

	if !isLetter(r) {
		goto err
	}

	sc.scankey()

	r = sc.get()

	for r == ' ' || r == '\t' {
		r = sc.get()
	}

	if r != '=' {
		goto err
	}

	sc.get()
	sc.scanval()
	return

err:
	sc.err(fmt.Sprintf("unexpected token %s", string(r)))
	sc.skipline()
	goto redo
}

func (e errs) err() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

func (e errs) Error() string {
	var buf bytes.Buffer

	for _, msg := range e {
		buf.WriteString(msg + "\n")
	}
	return buf.String()
}

func (v *varset) String() string { return "" }

func (v *varset) Set(s string) error {
	parts := strings.SplitN(s, "=", 2)

	if len(parts) < 2 {
		return errors.New("invalid variable, must be key=value")
	}

	if (*v) == nil {
		(*v) = make(map[string]string)
	}

	key := parts[0]
	val := parts[1]

	(*v)[key] = val
	return nil
}

func decodeVarfile(r io.Reader, errh func(int, int, string)) map[string]string {
	src := newSource(r, errh)
	sc := newScanner(src)

	m := make(map[string]string)

	for !sc.eof {
		m[sc.key] = sc.val
		sc.next()
	}
	return m
}

func loadVarfile(path string) (map[string]string, error) {
	f, err := os.Open(path)

	if err != nil {
		return nil, err
	}

	defer f.Close()

	errs := errs(make([]string, 0))

	m := decodeVarfile(f, func(line, col int, msg string) {
		errs = append(errs, fmt.Sprintf("%s,%d:%d - %s", path, line, col, msg))
	})
	return m, errs.err()
}

func main() {
	var (
		vars    varset
		varfile string
	)

	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.Var(&vars, "var","set a variable, value should be in format of key=value")
	fs.StringVar(&varfile, "file", "", "the file to read variables from")
	fs.Parse(os.Args[1:])

	args := fs.Args()

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: %s [-file file] [-var key=value] <template>\n", os.Args[0])
		os.Exit(1)
	}

	if varfile != "" {
		m, err := loadVarfile(varfile)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: failed to load variables from file: %s", os.Args[0], err)
			os.Exit(1)
		}

		if vars == nil {
			vars = make(map[string]string)
		}

		for k, v := range m {
			vars[k] = v
		}
	}

	var t *template.Template

	b, err := ioutil.ReadFile(args[0])

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to read template file: %s\n", os.Args[0], err)
		os.Exit(1)
	}

	t, err = template.New(args[0]).Parse(string(b))

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to parse template: %s\n", os.Args[0], err)
		os.Exit(1)
	}

	var buf bytes.Buffer

	if err := t.Execute(&buf, vars); err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to execute template: %s\n", os.Args[0], err)
		os.Exit(1)
	}
	io.Copy(os.Stdout, &buf)
}
