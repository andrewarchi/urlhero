// Copyright (c) 2021 Andrew Archibald
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ia contains utilities for working with files from the
// Internet Archive.
package ia

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"

	"github.com/andrewarchi/browser/jsonutil"
	"github.com/andrewarchi/browser/jsonutil/timefmt"
)

func Validate(dir string) error {
	metaName := filepath.Base(dir) + "_files.xml"
	files, err := ReadFileMeta(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		fmt.Println(file.Name)
		if file.Name == metaName {
			continue // checksums of itself are inaccurate
		}
		fv, err := file.OpenValidator(dir)
		if err != nil {
			return err
		}
		_, err = io.Copy(io.Discard, fv)
		fv.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

type filesMeta struct {
	Files []FileMeta `xml:"file"`
}

// FileMeta contains file metadata listed in the *_files.xml file in the
// root of an item. This file is excluded for torrent downloads.
type FileMeta struct {
	Name     string          `xml:"name,attr"`   // filename, relative to root
	Source   string          `xml:"source,attr"` // "original", "metadata", or "derivative"
	Format   string          `xml:"format"`      // e.g. "Text", "Metadata", "Unknown"
	Original string          `xml:"original"`
	BTIH     jsonutil.Hex    `xml:"btih"` // BitTorrent info-hash
	ModTime  timefmt.UnixSec `xml:"mtime"`
	Size     int64           `xml:"size"`
	MD5      jsonutil.Hex    `xml:"md5"`
	CRC32    jsonutil.Hex    `xml:"crc32"`
	SHA1     jsonutil.Hex    `xml:"sha1"`
	Length   float64         `xml:"length"` // audio duration
	Height   int             `xml:"height"` // image height
	Width    int             `xml:"width"`  // image width
	Private  bool            `xml:"private"`
}

func ReadFileMeta(dir string) ([]FileMeta, error) {
	name := filepath.Base(dir) + "_files.xml"
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var meta filesMeta
	if err := xml.NewDecoder(f).Decode(&meta); err != nil {
		return nil, err
	}
	return meta.Files, nil
}

func (fm *FileMeta) OpenValidator(dir string) (io.ReadCloser, error) {
	f, err := os.Open(filepath.Join(dir, fm.Name))
	if err != nil {
		return nil, err
	}
	return &readValidateCloser{Reader: fm.Validator(f), rc: f}, nil
}

func (fm *FileMeta) Validator(r io.Reader) io.Reader {
	return NewReadValidator(r, fm.Name, fm.MD5, fm.SHA1, fm.CRC32)
}

func ValidateFile(filename string, md5Sum, sha1Sum, crc32Sum []byte) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	v := NewReadValidator(f, filename, md5Sum, sha1Sum, crc32Sum)
	_, err = io.Copy(io.Discard, v)
	return err
}

type readValidator struct {
	r         io.Reader
	name      string
	md5Hash   hash.Hash
	sha1Hash  hash.Hash
	crc32Hash hash.Hash32
	md5Sum    []byte
	sha1Sum   []byte
	crc32Sum  []byte
}

type readValidateCloser struct {
	io.Reader
	rc io.ReadCloser
}

func NewReadValidator(r io.Reader, name string, md5Sum, sha1Sum, crc32Sum []byte) io.Reader {
	rv := &readValidator{r: r, name: name, md5Sum: md5Sum, sha1Sum: sha1Sum, crc32Sum: crc32Sum}
	if len(md5Sum) != 0 {
		rv.md5Hash = md5.New()
		rv.r = io.TeeReader(rv.r, rv.md5Hash)
	}
	if len(sha1Sum) != 0 {
		rv.sha1Hash = sha1.New()
		rv.r = io.TeeReader(rv.r, rv.sha1Hash)
	}
	if len(crc32Sum) != 0 {
		rv.crc32Hash = crc32.NewIEEE()
		rv.r = io.TeeReader(rv.r, rv.crc32Hash)
	}
	return rv
}

func (rv *readValidator) Read(p []byte) (n int, err error) {
	n, err = rv.r.Read(p)
	if err == io.EOF {
		if err1 := rv.validate(); err1 != nil {
			return n, err1
		}
	}
	return n, err
}

func (rv *readValidator) validate() error {
	if err := rv.validateSum("MD5", rv.md5Hash, rv.md5Sum); err != nil {
		return err
	}
	if err := rv.validateSum("SHA-1", rv.sha1Hash, rv.sha1Sum); err != nil {
		return err
	}
	return rv.validateSum("CRC-32", rv.crc32Hash, rv.crc32Sum)
}

func (rv *readValidator) validateSum(kind string, hash hash.Hash, sum []byte) error {
	if hash != nil {
		s := hash.Sum(nil)
		if !bytes.Equal(s, sum) {
			return fmt.Errorf("ia: validate %s: %s sum is %x instead of %x", rv.name, kind, s, sum)
		}
	}
	return nil
}

func (rv *readValidateCloser) Close() error { return rv.rc.Close() }
