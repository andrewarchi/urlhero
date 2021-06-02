// Copyright (c) 2021 Andrew Archibald
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ia

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/andrewarchi/browser/jsonutil"
)

// TimemapOptions contains options for a timemap API call.
type TimemapOptions struct {
	MatchPrefix bool     // whether url is a prefix (* wildcard is appended)
	Collapse    string   // field to collapse by; earliest captures with unique field is kept
	Fields      []string // e.g. urlkey,timestamp,endtimestamp,original,mimetype,statuscode,digest,redirect,robotflags,length,offset,filename,groupcount,uniqcount
	Limit       int      // e.g. 100000
}

// GetTimemap gets a list of Internet Archive captures of the given URL.
func GetTimemap(pageURL string, options *TimemapOptions) ([][]string, error) {
	// Timemap API, as observed on
	// https://web.archive.org/web/*/https://dumps.wikimedia.org/other/shorturls/*

	q := make(url.Values)
	q.Set("url", pageURL)
	q.Set("output", "json") // other values: "csv" and omitted
	if options != nil {
		if options.MatchPrefix {
			q.Set("matchType", "prefix") // other values unknown
		}
		if options.Collapse != "" {
			q.Set("collapse", options.Collapse)
		}
		if len(options.Fields) != 0 {
			q.Set("fl", strings.Join(options.Fields, ","))
		}
		// TODO paging
		if options.Limit > 0 {
			q.Set("limit", strconv.Itoa(options.Limit))
		}
	}

	resp, err := checkResponse(http.Get("https://web.archive.org/web/timemap/?" + q.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var timemap [][]string
	if err := jsonutil.Decode(resp.Body, &timemap); err != nil {
		return nil, err
	}
	if len(timemap) >= 1 {
		timemap = timemap[1:] // Skip header row
	}
	return timemap, nil
}

// DecodeDigest decodes a base32-encoded SHA-1 digest.
func DecodeDigest(digest string) (*[20]byte, error) {
	if len(digest) != 32 {
		return nil, fmt.Errorf("ia: digest not 32 bytes: %q", digest)
	}
	var b [20]byte
	bit := 0
	for i := 0; i < 32; i++ {
		x, err := digestAlpha(digest[i], digest)
		if err != nil {
			return nil, err
		}
		// Fill 5-bit digits into 8-bit bytes
		x <<= 3
		b[bit/8] |= x >> (bit % 8)
		if bit/8 < 19 {
			b[bit/8+1] |= x << (8 - bit%8)
		}
		bit += 5
	}
	return &b, nil
}

// digestAlpha returns the base-32 value for the character in the
// alphabet "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567".
func digestAlpha(ch byte, digest string) (byte, error) {
	switch {
	case 'A' <= ch && ch <= 'Z':
		return ch - 'A', nil
	case '2' <= ch && ch <= '7':
		return ch - '2' + 26, nil
	default:
		return 0, fmt.Errorf("ia: illegal byte %q in digest: %q", ch, digest)
	}
}

func checkResponse(resp *http.Response, err error) (*http.Response, error) {
	if err == nil && resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("ia: http status %s", resp.Status)
	}
	return resp, err
}
