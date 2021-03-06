// Copyright (c) 2020-2021 Andrew Archibald
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package beacon processes BEACON-format link dumps as defined by GBV
// and used by URLTeam.
package beacon

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Reader struct {
	r         *bufio.Reader
	meta      []MetaField
	metaRead  bool
	peekLine  string
	line      int
	format    Format
	sourceLen int
}

type MetaField struct {
	Name, Value string
}

type Link struct {
	Source, Target, Annotation string
}

// Format defines the format of the BEACON link dump.
type Format uint8

const (
	// RFC link dumps follow draft-003 of the BEACON format RFC submitted
	// December 2017 at https://gbv.github.io/beaconspec/beacon.html.
	RFC Format = iota

	// URLTeam link dumps resolve all links as SOURCE|TARGET. Any further
	// '|' characters on a line are considered to be part of TARGET.
	URLTeam
)

// NewReader constructs a reader that reads RFC-format BEACON link
// dumps.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReader(r)}
}

// NewURLTeamReader constructs a reader that reads URLTeam-format BEACON
// link dumps. Links always omit the annotation field.
func NewURLTeamReader(r io.Reader, shortcodeLen int) *Reader {
	return &Reader{r: bufio.NewReader(r), format: URLTeam, sourceLen: shortcodeLen}
}

// Meta returns the meta fields in the header.
func (r *Reader) Meta() ([]MetaField, error) {
	if r.metaRead {
		return r.meta, nil
	}
	r.metaRead = true
	meta, err := r.readMeta()
	if err == nil || err == io.EOF {
		return meta, nil
	}
	return nil, r.err(err)
}

func (r *Reader) readMeta() ([]MetaField, error) {
	if err := r.consumeBOM(); err != nil {
		return nil, err
	}
	// Allow omitted header section
	if b, err := r.r.Peek(1); err != nil || b[0] != '#' {
		return nil, err
	}

	// Read meta lines until the first blank line or non-#-prefixed line
	for {
		line, err := r.readLine()
		if err != nil {
			return r.meta, err
		}
		if trimLeftSpace(line) == "" {
			break
		}
		if line[0] != '#' {
			r.peekLine = line
			return r.meta, nil
		}
		meta, err := splitMeta(line[1:])
		if err != nil {
			return nil, err
		}
		r.meta = append(r.meta, meta)
	}

	// Consume empty lines
	for {
		line, err := r.readLine()
		if err != nil {
			return r.meta, err
		}
		if trimLeftSpace(line) != "" {
			r.peekLine = line
			return r.meta, nil
		}
	}
}

// consumeBOM skips a UTF-8 byte order mark as permitted by section 3.1.
func (r *Reader) consumeBOM() error {
	ch, _, err := r.r.ReadRune()
	if err != nil {
		return err
	}
	if ch == '\uFEFF' {
		return nil
	}
	return r.r.UnreadRune()
}

func splitMeta(meta string) (MetaField, error) {
	for i, ch := range meta {
		switch {
		case 'A' <= ch && ch <= 'Z':
		case ch == ':' || ch == ' ' || ch == '\t':
			return MetaField{meta[:i], trimLeftSpace(meta[i+1:])}, nil
		default:
			return MetaField{}, fmt.Errorf("invalid character %q in meta field: %q", ch, meta)
		}
	}
	return MetaField{}, fmt.Errorf("meta line missing value: %q", meta)
}

func (r *Reader) Read() (link *Link, err error) {
	if !r.metaRead {
		if _, err := r.Meta(); err != nil {
			return nil, err
		}
	}
	if r.format == URLTeam {
		link, err = r.readLinkURLTeam()
	} else {
		link, err = r.readLinkRFC()
	}
	return link, r.err(err)
}

func (r *Reader) readLinkRFC() (*Link, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	var link Link
	tokens := strings.SplitN(line, "|", 4)
	switch len(tokens) {
	case 1:
		link.Source = tokens[0]
	case 2:
		link.Source, link.Target = tokens[0], tokens[1]
		// TODO:
		// link.Source, link.Annotation = tokens[0], tokens[1]
	case 3:
		link.Source, link.Annotation, link.Target = tokens[0], tokens[1], tokens[2]
	case 4:
		return nil, fmt.Errorf("link line has too many bar separators: %q", line)
	}
	return &link, nil
}

func (r *Reader) readLinkURLTeam() (*Link, error) {
	line, err := r.readLineRaw()
	if err != nil {
		return nil, err
	}

	// Variable shortcode length
	if r.sourceLen <= 0 {
		i := strings.IndexByte(line, '|')
		if i == -1 {
			return nil, fmt.Errorf("link line missing bar separator: %q", line)
		}
		return &Link{line[:i], dropLineBreak(line[i:]), ""}, nil
	}

	// Fixed shortcode length
	if len(line) < r.sourceLen || line[r.sourceLen] != '|' {
		if i := strings.IndexByte(line, '|'); i != -1 {
			return nil, fmt.Errorf("shortcode not %d characters: %q", r.sourceLen, line)
		}
		return nil, fmt.Errorf("link line missing bar separator: %q", line)
	}
	shortcode, target := line[:r.sourceLen], line[r.sourceLen+1:]
	// Append successive lines in multi-line link
	for {
		line, err := r.readLineRaw()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(line) > r.sourceLen && line[r.sourceLen] == '|' {
			r.peekLine = line
			break
		}
		target += line
	}
	return &Link{shortcode, dropLineBreak(target), ""}, nil
}

func (r *Reader) readLine() (string, error) {
	line, err := r.readLineRaw()
	if err != nil {
		return "", err
	}
	return dropLineBreak(line), nil
}

func (r *Reader) readLineRaw() (string, error) {
	if l := r.peekLine; l != "" {
		r.peekLine = ""
		return l, nil
	}
	r.line++
	line, err := r.r.ReadString('\n')
	if err != nil && !(err == io.EOF && line != "") {
		return "", err
	}
	return line, nil
}

func (r *Reader) err(err error) error {
	if err == io.EOF || err == nil {
		return err
	}
	return fmt.Errorf("beacon: line %d: %w", r.line, err)
}

func dropLineBreak(line string) string {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		drop := 1
		if len(line) > 1 && line[len(line)-2] == '\r' {
			drop = 2
		}
		line = line[:len(line)-drop]
	}
	return line
}

func trimLeftSpace(s string) string {
	for s != "" && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}

func (m MetaField) String() string {
	return fmt.Sprintf("#%s: %s", m.Name, m.Value)
}

func (l Link) String() string {
	if l.Annotation != "" {
		return fmt.Sprintf("%s|%s|%s", l.Source, l.Annotation, l.Target)
	}
	return fmt.Sprintf("%s|%s", l.Source, l.Target)
}
