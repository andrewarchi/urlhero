// Copyright (c) 2021 Andrew Archibald
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ia

import (
	"encoding/xml"
	"os"
	"path/filepath"
)

// ItemMeta contains item metadata in the *_meta.xml file in the root of
// an item.
type ItemMeta struct {
	Identifier     string   `xml:"identifier"`
	Collections    []string `xml:"collection"`
	Description    string   `xml:"description"`
	Mediatype      string   `xml:"mediatype"` // e.g. "software"
	Subject        string   `xml:"subject"`
	Title          string   `xml:"title"`
	Uploader       string   `xml:"uploader"`
	Publicdate     string   `xml:"publicdate"` // "2006-01-02 15:04:05" format
	Addeddate      string   `xml:"addeddate"`  // "2006-01-02 15:04:05" format
	Curation       string   `xml:"curation"`
	BackupLocation string   `xml:"backup_location"` // removed from meta in April 2020
}

func ReadItemMeta(dir string) (*ItemMeta, error) {
	name := filepath.Base(dir) + "_meta.xml"
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var meta ItemMeta
	if err := xml.NewDecoder(f).Decode(&meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

const TimestampFormat = "20060102150405"

func PageURL(url, timestamp string) string {
	return "https://web.archive.org/web/" + timestamp + "id_/" + url
}
