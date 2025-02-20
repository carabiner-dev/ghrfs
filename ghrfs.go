// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

// Package ghrfs (short for GitHub Release File System) is an fs.FS implementation
// that reads data from a GitHub release.
//
// ghrfs can read directly from GitHub's API or cache a release locally for
// faster access with less bandwisth consumption or to support switching to
// airgapped environments.
package ghrfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/carabiner-dev/github"
	"github.com/nozzle/throttler"
)

const (
	releaseURLMask  = `/repos/%s/%s/releases/tags/%ss`
	githubAPIURL    = "api.github.com"
	releaseDataFile = ".release-data.json"
)

func New(optFns ...optFunc) (*ReleaseFileSystem, error) {
	opts := defaultOptions
	for _, fn := range optFns {
		if err := fn(&opts); err != nil {
			return nil, err
		}
	}

	return NewWithOptions(opts)
}

// NewWithOptions takes an options set and return a new RFS
func NewWithOptions(opts Options) (*ReleaseFileSystem, error) {
	c, err := github.NewClient()
	if err != nil {
		return nil, err
	}
	c.Options.Host = opts.Host
	return &ReleaseFileSystem{
		Options: opts,
		client:  c,
	}, nil
}

// Ensure RFS implements fs.FS
var _ fs.FS = (*ReleaseFileSystem)(nil)

// ReleaseFileSystem implements fs.FS by reading data a GitHub release.
type ReleaseFileSystem struct {
	Options Options
	Release ReleaseData
	client  *github.Client
}

// ReleaseData captures the release information from github
type ReleaseData struct {
	ID          int64        `json:"id"`
	URL         string       `json:"url"`
	Tag         string       `json:"tag_name"`
	Draft       bool         `json:"draft"`
	PublishedAt time.Time    `json:"published_at"`
	CreatedAt   time.Time    `json:"created_at"`
	Assets      []*AssetFile `json:"assets"`
	fileIndex   map[string]int
}

// LoadRelease queries the GitHub API and loads the release data,
// optionally catching the assets
func (rfs *ReleaseFileSystem) LoadRelease() error {
	resp, err := rfs.client.Call(
		context.Background(), "GET",
		fmt.Sprintf(
			releaseURLMask, rfs.Options.Organization, rfs.Options.Repository, rfs.Options.Tag,
		), nil,
	)

	if err != nil {
		return fmt.Errorf("loading release: %w", err)
	}
	defer resp.Body.Close()

	data := ReleaseData{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&data); err != nil {
		return fmt.Errorf("unmarshaling release data: %w", err)
	}
	rfs.Release = data

	// Index files
	rfs.Release.fileIndex = map[string]int{}
	for i, f := range rfs.Release.Assets {
		if f.Name() == "" {
			continue // Not sure if this can happen
		}
		rfs.Release.fileIndex[f.Name()] = i
	}

	if rfs.Options.Cache {
		if err := rfs.CacheRelease(); err != nil {
			return fmt.Errorf("caching release: %w", err)
		}
	}

	return nil
}

// Open opens a file.
func (rfs *ReleaseFileSystem) Open(name string) (fs.File, error) {
	// Check if the asset file has its data stream already open
	i, ok := rfs.Release.fileIndex[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if rfs.Release.Assets[i].DataStream != nil {
		return rfs.Release.Assets[i], nil
	}

	// Otherwise open it
	if rfs.Options.Cache {
		return rfs.OpenCachedFile(name)
	}
	return rfs.OpenRemoteFile(name)
}

// OpenCachedFile returns an asset file with its data source connected to
// a local cached file
func (rfs *ReleaseFileSystem) OpenCachedFile(name string) (fs.File, error) {
	i, ok := rfs.Release.fileIndex[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if !rfs.Options.Cache {
		return nil, fmt.Errorf("unable to open file, release is not cached")
	}

	if rfs.Options.CachePath == "" {
		return nil, fmt.Errorf("unable to open file, release cache path not set")
	}

	f, err := os.Open(filepath.Join(rfs.Options.CachePath, name))
	if err != nil {
		return nil, fmt.Errorf("opening cached file: %w", err)
	}

	rfs.Release.Assets[i].DataStream = f
	return rfs.Release.Assets[i], nil
}

// getClientForURL returns a github client configured for the hostname
// of a URL.
func getClientForURL(urlString string) (*github.Client, error) {
	// The download URL from the assets is not on the same host as
	// the API, so we need a new client
	u, err := url.Parse(urlString)
	if err != nil {
		return nil, fmt.Errorf("parsing asset URL: %w", err)
	}

	// Request the file using a client with the asset URL
	c, err := github.NewClient()
	if err != nil {
		return nil, err
	}
	c.Options.Host = u.Hostname()
	return c, nil
}

// OpenRemoteFile returns the asset file connected to its data stream
func (rfs *ReleaseFileSystem) OpenRemoteFile(name string) (fs.File, error) {
	i, ok := rfs.Release.fileIndex[name]
	if !ok {
		return nil, fs.ErrNotExist
	}

	if rfs.Release.Assets[i].URL == "" {
		return nil, fmt.Errorf("no url found in asset data")
	}

	// Assets are not downloaded from the API, we need a new client
	c, err := getClientForURL(rfs.Release.Assets[i].URL)
	if err != nil {
		return nil, err
	}

	// Send the request to the API
	resp, err := rfs.client.Call(
		context.Background(), "GET",
		strings.TrimPrefix(rfs.Release.Assets[i].URL, "https://"+c.Options.Host), nil,
	)
	if err != nil {
		return nil, fmt.Errorf("requesting file from API: %w", err)
	}
	rfs.Release.Assets[i].DataStream = resp.Body
	return rfs.Release.Assets[i], nil
}

// CacheRelease downloads `ParallelDownloads` assets at a time and caches them
// in `Options.CachePath`. Each asset file's data stream is copied to a local
// file. If assets already have a DataStream defined, it is reused for copying
// and it will be closed to be replaced by the new local file when it is used.
func (rfs *ReleaseFileSystem) CacheRelease() error {
	if rfs.Options.CachePath == "" {
		return errors.New("release cache path not specified")
	}
	// Cache the release data into a JSON file
	f, err := os.Create(filepath.Join(rfs.Options.CachePath, releaseDataFile))
	if err != nil {
		return fmt.Errorf("creating release data file: %w", err)
	}

	if err := json.NewEncoder(f).Encode(rfs.Release); err != nil {
		return fmt.Errorf("encoding release data: %w", err)
	}

	// Now copy the file data to the local cache
	t := throttler.New((rfs.Options.ParallelDownloads), len(rfs.Release.Assets))
	for _, a := range rfs.Release.Assets {
		go func() {
			var src fs.File
			var err error
			if a.DataStream != nil {
				src = a
			} else {
				src, err = rfs.OpenRemoteFile(a.Name())
				if err != nil {
					t.Done(err)
					return
				}
			}

			dst, err := os.Create(filepath.Join(rfs.Options.CachePath, a.Name()))
			if err != nil {
				t.Done(err)
				return
			}

			if _, err := io.Copy(dst, src); err != nil {
				t.Done(err)
				return
			}
			a.DataStream.Close()
			a.DataStream = nil

			t.Done(nil)
		}()
		t.Throttle()
	}
	rfs.Options.Cache = true

	return nil
}
