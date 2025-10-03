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
	"slices"
	"strings"
	"time"

	"github.com/carabiner-dev/github"
	"github.com/nozzle/throttler"
)

const (
	releaseURLMask  = `repos/%s/%s/releases/tags/%s`
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

	return NewWithOptions(&opts)
}

// NewWithOptions takes an options set and return a new RFS
func NewWithOptions(opts *Options) (*ReleaseFileSystem, error) {
	c, err := github.NewClient()
	if err != nil {
		return nil, err
	}
	c.Options.Host = opts.Host

	rfs := &ReleaseFileSystem{
		Options: *opts,
		client:  c,
	}

	if err := rfs.LoadRelease(); err != nil {
		return nil, fmt.Errorf("loading release: %w", err)
	}

	return rfs, nil
}

// Ensure RFS implements fs.FS
var (
	_ fs.FS        = (*ReleaseFileSystem)(nil)
	_ fs.StatFS    = (*ReleaseFileSystem)(nil)
	_ fs.ReadDirFS = (*ReleaseFileSystem)(nil)
)

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
	// Use the stock release endpoint
	releaseURL := fmt.Sprintf(
		releaseURLMask, rfs.Options.Organization, rfs.Options.Repository, rfs.Options.Tag,
	)

	// ...unless we're targeting the latest one, which is different:
	if rfs.Options.Tag == "" || rfs.Options.Tag == "latest" {
		releaseURL = fmt.Sprintf(
			"repos/%s/%s/releases/latest", rfs.Options.Organization, rfs.Options.Repository,
		)
	}

	// Call the API to get the data
	resp, err := rfs.client.Call(
		context.Background(), "GET", releaseURL, nil,
	)
	if resp.StatusCode > 399 || resp.StatusCode < 200 {
		return fmt.Errorf("HTTP error %d when getting release data", resp.StatusCode)
	}
	if err != nil {
		return fmt.Errorf("loading release: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	data := ReleaseData{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&data); err != nil { //nolint:musttag
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

func (rfs *ReleaseFileSystem) Stat(name string) (fs.FileInfo, error) {
	if name == "." || name == "/" {
		return FileInfo{
			IName:  rfs.Release.Tag,
			ISize:  0,
			Ctime:  rfs.Release.PublishedAt,
			Mtime:  rfs.Release.PublishedAt,
			IIsDir: true,
		}, nil
	}
	i, ok := rfs.Release.fileIndex[name]
	if !ok {
		return nil, fmt.Errorf("opening %q: %w", name, fs.ErrNotExist)
	}

	return rfs.Release.Assets[i], nil
}

// ReadDir implements readddir fs
func (rfs *ReleaseFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	// The only "dir" we support is the root, which is the release itself
	if name != "." && name != "/" {
		return nil, fs.ErrNotExist
	}
	ret := []fs.DirEntry{}
	for _, f := range rfs.Release.Assets {
		ret = append(ret, f)
	}
	return ret, nil
}

// Open opens a file.
func (rfs *ReleaseFileSystem) Open(name string) (fs.File, error) {
	if name == "." {
		assets := []fs.DirEntry{}
		for _, f := range rfs.Release.Assets {
			assets = append(assets, f)
		}
		return &ReleaseDir{
			Tag:        rfs.Release.Tag,
			Ctime:      rfs.Release.PublishedAt,
			Mtime:      rfs.Release.PublishedAt,
			AssetFiles: assets,
		}, nil
	}

	// Check if the asset file has its data stream already open
	i, ok := rfs.Release.fileIndex[name]
	if !ok {
		return nil, fmt.Errorf("opening %q: %w", name, fs.ErrNotExist)
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
		return nil, fmt.Errorf("opening %q: %w", name, fs.ErrNotExist)
	}
	if !rfs.Options.Cache {
		return nil, fmt.Errorf("unable to open file, release is not cached")
	}

	if rfs.Options.CachePath == "" {
		return nil, fmt.Errorf("unable to open file, release cache path not set")
	}

	f, err := os.Open(filepath.Join(rfs.Options.CachePath, name))
	if err != nil {
		// If the file was not found, open the remote file
		if errors.Is(err, os.ErrNotExist) {
			return rfs.OpenRemoteFile(name)
		}
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
	c, err := github.NewClient(
		github.WithHost(u.Hostname()),
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// OpenRemoteFile returns the asset file connected to its data stream
func (rfs *ReleaseFileSystem) OpenRemoteFile(name string) (fs.File, error) {
	i, ok := rfs.Release.fileIndex[name]
	if !ok {
		return nil, fmt.Errorf("opening %q: %w", name, fs.ErrNotExist)
	}

	if rfs.Release.Assets[i].URL == "" {
		return nil, fmt.Errorf("no URL found in asset data")
	}

	// Assets are not downloaded from the API, we need a new client
	c, err := getClientForURL(rfs.Release.Assets[i].URL)
	if err != nil {
		return nil, err
	}

	// Send the request to the API
	resp, err := c.Call(
		context.Background(), "GET",
		rfs.Release.Assets[i].URL, nil,
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
	// If there is no cache path specified, create a temporary file
	if rfs.Options.CachePath == "" {
		path, err := os.MkdirTemp("", "github-release-fs-")
		if err != nil {
			return fmt.Errorf("creating temporary cache dir: %w", err)
		}
		rfs.Options.CachePath = path
	}

	// Cache the release data into a JSON file
	f, err := os.Create(filepath.Join(rfs.Options.CachePath, releaseDataFile))
	if err != nil {
		return fmt.Errorf("creating release data file: %w", err)
	}

	//nolint:musttag
	if err := json.NewEncoder(f).Encode(rfs.Release); err != nil {
		return fmt.Errorf("encoding release data: %w", err)
	}

	// Now copy the file data to the local cache
	t := throttler.New((rfs.Options.ParallelDownloads), len(rfs.Release.Assets))
	for _, a := range rfs.Release.Assets {
		go func() {
			// Check if the options have preferences for max size or extensions
			// to cache. If unmatched, the asset will not be cached but it will
			// be pulled remotely if needed.

			// Skip if over max size
			if rfs.Options.CacheMaxSize > 0 && rfs.Options.CacheMaxSize < a.Size() {
				t.Done(nil)
				return
			}

			// Skip if extensions are defined but the file ext is not one of them
			if len(rfs.Options.CacheExtensions) > 0 &&
				(strings.TrimPrefix(filepath.Ext(a.Name()), ".") == "" ||
					!slices.Contains(rfs.Options.CacheExtensions, strings.TrimPrefix(filepath.Ext(a.Name()), "."))) {
				t.Done(nil)
				return
			}

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
			a.cachePath = filepath.Join(rfs.Options.CachePath, a.Name())
			a.DataStream.Close() //nolint:errcheck,gosec
			a.DataStream = nil

			t.Done(nil)
		}()
		t.Throttle()
	}
	rfs.Options.Cache = true

	return nil
}
