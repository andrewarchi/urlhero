// Copyright (c) 2020-2021 Andrew Archibald
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tinytown processes URLTeam's second generation Terror of Tiny
// Town releases.
package tinytown

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/storage"
	"github.com/andrewarchi/browser/jsonutil"
)

// DownloadTorrents downloads all terroroftinytown releases via torrent.
func DownloadTorrents(dir string) error {
	ids, err := GetReleaseIDs()
	if err != nil {
		return err
	}

	conf := torrent.NewDefaultClientConfig()
	conf.DataDir = dir
	conf.DefaultStorage = storage.NewMMap(dir)
	c, err := torrent.NewClient(conf)
	if err != nil {
		return err
	}

	for i, id := range ids {
		fmt.Printf("(%d/%d) Adding %s\n", i+1, len(ids), id)
		filename, err := saveTorrentFile(id, dir)
		if err != nil {
			return err
		}
		t, err := c.AddTorrentFromFile(filename)
		if err != nil {
			return err
		}
		t.DownloadAll()
		if i%15 == 14 {
			c.WaitAll()
		}
	}
	c.WaitAll()
	return nil
}

// GetReleaseIDs queries the Internet Archive for the identifiers of all
// incremental terroroftinytown releases.
func GetReleaseIDs() ([]string, error) {
	url := "https://archive.org/services/search/v1/scrape?q=subject:terroroftinytown&count=10000"
	resp, err := httpGet(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	type Scrape struct {
		Items []struct {
			Identifier string `json:"identifier"`
		} `json:"items"`
		Count int `json:"count"`
		Total int `json:"total"`
		// TODO fields for error response
	}
	var items Scrape
	if err := jsonutil.Decode(resp.Body, &items); err != nil {
		return nil, err
	}

	// TODO handle paging
	if items.Count != items.Total {
		return nil, fmt.Errorf("tinytown: queried %d of %d releases", items.Count, items.Total)
	}

	ids := make([]string, len(items.Items))
	for i, item := range items.Items {
		ids[i] = item.Identifier
	}
	return ids, nil
}

func saveTorrentFile(id, dir string) (string, error) {
	url := "https://archive.org/download/" + id + "/" + id + "_archive.torrent"
	filename := filepath.Join(dir, path.Base(url))
	return filename, saveFile(url, filename)
}

func saveFile(url, filename string) error {
	if _, err := os.Stat(filename); err == nil {
		return nil
	}

	resp, err := httpGet(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func httpGet(url string) (*http.Response, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("tinytown: http status %s", resp.Status)
	}
	return resp, nil
}
