// Copyright (c) 2021 Andrew Archibald
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tinytown

import (
	"encoding/hex"

	"github.com/andrewarchi/browser/jsonutil"
)

// Tracker is the base URL of the Terror of Tiny Town tracker instance.
// This can be changed to use an alternate tracker.
var Tracker = "https://tracker.archiveteam.org:1338"

type Health struct {
	HTTPStatusCode    int                     // e.g. 200
	HTTPStatusMessage string                  // e.g. "OK"
	GitHash           []byte                  // e.g. 80ffc526a8b3fd188e6f73fab7b425af61f45d28
	Projects          []string                // IDs of all projects, including disabled ones, e.g. "bitly_6"
	ProjectStats      map[string]ProjectStats // key: project ID
}

type ProjectStats struct {
	Found   int64
	Scanned int64
}

func GetHealth() (*Health, error) {
	resp, err := httpGet(Tracker + "/api/health")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var health struct {
		HTTPStatusCode    int                 `json:"http_status_code"`
		HTTPStatusMessage string              `json:"http_status_message"`
		GitHash           string              `json:"git_hash"` // e.g. "b'80ffc526a8b3fd188e6f73fab7b425af61f45d28'"
		Projects          []string            `json:"projects"`
		ProjectStats      map[string][2]int64 `json:"project_stats"`
	}
	if err := jsonutil.Decode(resp.Body, &health); err != nil {
		return nil, err
	}

	h := health.GitHash
	if len(h) >= 3 && h[0] == 'b' && h[1] == '\'' && h[len(h)-1] == '\'' {
		h = h[2 : len(h)-1]
	}
	hash, err := hex.DecodeString(h)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]ProjectStats, len(health.ProjectStats))
	for project, s := range health.ProjectStats {
		stats[project] = ProjectStats{s[0], s[1]}
	}

	return &Health{
		HTTPStatusCode:    health.HTTPStatusCode,
		HTTPStatusMessage: health.HTTPStatusMessage,
		GitHash:           hash,
		Projects:          health.Projects,
		ProjectStats:      stats,
	}, nil
}
