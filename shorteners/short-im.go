// Copyright (c) 2021 Andrew Archibald
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package shorteners

import (
	"net/url"
	"regexp"
	"strings"
)

// TODO RSS feed at http://short.im/feed.rss
// TODO shrink multiple URLs at http://short.im:80/multishrink.html
// TODO determine whether reboot shares links

// ShortIm describes the Short.i'm short.im link shortener.
var ShortIm = &Shortener{
	Name:     "short-im",
	Host:     "short.im",
	Prefix:   "http://short.im/", // 2018 redesign uses https
	Alphabet: "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz",
	// Underscore and dash are only allowed for vanity URLs.
	Pattern:   regexp.MustCompile(`^[0-9A-Za-z\-_]+$`),
	CleanFunc: cleanShortIm,
	HasVanity: true,
}

func cleanShortIm(shortcode string, u *url.URL) string {
	switch shortcode {
	// Get URL from API query string:
	//   http://short.im/api.php?short=http://short.im/<shortcode>
	//   http://short.im/api.php?url=http://short.im/<shortcode>
	case "api.php", "api":
		q := u.Query()
		short := q.Get("short")
		if short == "" {
			short = q.Get("url")
		}
		if short != "" {
			u2, err := url.Parse(short)
			if err == nil && getHostname(u2) == "short.im" {
				return cleanURL(u2, cleanShortIm)
			}
		}
		return ""
	// Exclude static files
	case "donate", "tos", "warn":
		return ""
	}
	// Exclude static files
	if strings.ContainsAny(shortcode, "./") {
		return ""
	}
	return shortcode
}
