// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

// Package ghrfs (short ffor GitHub Release File System) is an fs.FS implementation
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
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/carabiner-dev/github"
)

const releaseURLMask = `/repos/%s/%s/releases/tags/%ss`
const githubAPIURL = "api.github.com"

func New(optFns ...optFunc) (*ReleaseFileSystem, error) {
	opts := defaultOptions
	for _, fn := range optFns {
		fn(&opts)
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

// OpenRemoteFile returns the asset file connected to its data stream
func (rfs *ReleaseFileSystem) OpenRemoteFile(name string) (fs.File, error) {
	i, ok := rfs.Release.fileIndex[name]
	if !ok {
		return nil, fs.ErrNotExist
	}

	if rfs.Release.Assets[i].URL == "" {
		return nil, fmt.Errorf("no url found in asset data")
	}

	// The download URL from the assets is not on the same host as
	// the API, so we need a new client
	u, err := url.Parse(rfs.Release.Assets[i].URL)
	if err != nil {
		return nil, fmt.Errorf("parsing asset URL: %w", err)
	}

	// Request the file using a client with the asset URL
	c, err := github.NewClient()
	if err != nil {
		return nil, err
	}
	c.Options.Host = u.Hostname()

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

// CacheRelease caches
func (rfs *ReleaseFileSystem) CacheRelease() error {
	return errors.New("Not yet")
}
