// Copyright (c) 2020-2021 Andrew Archibald
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tinytown

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/andrewarchi/archive"
	"github.com/andrewarchi/browser/jsonutil"
	"github.com/andrewarchi/urlhero/beacon"
)

// Meta contains link dump metadata from a *.meta.json.xz file.
type Meta struct {
	Name              string  `json:"name"`
	MinVersion        int     `json:"min_version"`        // minimum library version
	MinClientVersion  int     `json:"min_client_version"` // minimum pipeline version
	Alphabet          string  `json:"alphabet"`
	URLTemplate       string  `json:"url_template"`
	RequestDelay      float64 `json:"request_delay"`     // e.g. 0.5
	RedirectCodes     []int   `json:"redirect_codes"`    // HTTP codes
	NoRedirectCodes   []int   `json:"no_redirect_codes"` // HTTP codes
	UnavailableCodes  []int   `json:"unavailable_codes"` // HTTP codes
	BannedCodes       []int   `json:"banned_codes"`      // HTTP codes
	BodyRegex         string  `json:"body_regex"`
	LocationAntiRegex string  `json:"location_anti_regex"`
	Method            string  `json:"method"` // HTTP method, e.g. "head"
	Enabled           bool    `json:"enabled"`
	Autoqueue         bool    `json:"autoqueue"`
	NumCountPerItem   int     `json:"num_count_per_item"`
	MaxNumItems       int     `json:"max_num_items"`
	LowerSequenceNum  int64   `json:"lower_sequence_num"`
	AutoreleaseTime   int     `json:"autorelease_time"`
}

// ProcessFunc is the type of function that is called for each link
// visited.
type ProcessFunc func(l *beacon.Link, m *Meta, shortcodeLen int, releaseFilename, dumpFilename string) error

// ProcessReleases processes every release in a directory by calling fn
// on every link.
func ProcessReleases(root string, fn ProcessFunc) error {
	// TODO allow user to skip releases or projects.
	rootContents, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, release := range rootContents {
		if !release.IsDir() {
			continue
		}
		dir := filepath.Join(root, release.Name())
		dirContents, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, file := range dirContents {
			filename := filepath.Join(dir, file.Name())
			if !strings.HasSuffix(filename, ".zip") {
				continue
			}
			if err := ProcessProject(filename, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

// ProcessProject processes every link dump in a project release by
// calling fn on every link.
func ProcessProject(filename string, fn ProcessFunc) error {
	zr, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer zr.Close()
	metaFile, dumps, err := classifyFiles(zr.File, filename)
	if err != nil {
		return err
	}
	meta, err := readMeta(metaFile)
	if err != nil {
		return err
	}
	for _, f := range dumps {
		if err := processLinkDump(f, filename, meta, fn); err != nil {
			return err
		}
	}
	return nil
}

func classifyFiles(files []*zip.File, filename string) (meta *zip.File, dumps []*zip.File, err error) {
	// Before 2015-07-29, project zip archives were sorted with meta
	// first, followed by dumps in increasing shortcode length. Later
	// archives do not sort files.
meta:
	switch {
	case len(files) == 0:
		return nil, nil, fmt.Errorf("tinytown: empty archive: %s", filename)
	// Meta is usually first or last; don't allocate for those.
	case strings.HasSuffix(files[0].Name, ".meta.json.xz"):
		meta = files[0]
		dumps = files[1:]
	case strings.HasSuffix(files[len(files)-1].Name, ".meta.json.xz"):
		meta = files[len(files)-1]
		dumps = files[:len(files)-1]
	default:
		for i, f := range files {
			if strings.HasSuffix(f.Name, ".meta.json.xz") {
				meta = f
				dumps = make([]*zip.File, 0, len(files)-1)
				dumps = append(append(dumps, files[:i]...), files[i+1:]...)
				break meta
			}
		}
		return nil, nil, fmt.Errorf("tinytown: no meta file in archive: %s", filename)
	}

	for _, f := range dumps {
		if !strings.HasSuffix(f.Name, ".txt.xz") {
			return nil, nil, fmt.Errorf("tinytown: not a link dump: %s", filename)
		}
	}
	return
}

func readMeta(f *zip.File) (*Meta, error) {
	fr, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer fr.Close()
	xr, err := archive.NewXZReader(fr)
	if err != nil {
		return nil, err
	}
	defer xr.Close()
	var m Meta
	if err := jsonutil.Decode(xr, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func processLinkDump(f *zip.File, filename string, meta *Meta, fn ProcessFunc) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	xr, err := archive.NewXZReader(r)
	if err != nil {
		return err
	}
	defer xr.Close()

	shortcodeLen := len(filepath.Base(f.Name)) - len(".txt.xz")
	br := beacon.NewURLTeamReader(xr, shortcodeLen)
	fmt.Fprintf(os.Stderr, "%s:%s ", filepath.Base(filename), f.Name)
	n := 0
	for {
		link, err := br.Read()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%d links]\n", n)
			if err == io.EOF {
				return nil
			}
			return err
		}
		n++
		if err := fn(link, meta, shortcodeLen, filename, f.Name); err != nil {
			return err
		}
	}
}
