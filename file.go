// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package ghrfs

import (
	"io"
	"io/fs"
	"time"
)

var _ fs.File = (*AssetFile)(nil)

// AssetFile abstracts an asset stored in a GitHub release and
// implements fs.File by reading data from an io.ReadCloser
type AssetFile struct {
	DataStream io.ReadCloser
	URL        string `json:"browser_download_url"`
	ID         int64  `json:"id"`
	FileInfo
}

// Close implements the Close method for the file. After closing, the response
// stream is niled out to cause a re-fetch if there is another call to open/read.
func (af *AssetFile) Close() error {
	af.DataStream.Close() //nolint:errcheck,gosec
	af.DataStream = nil
	return nil
}

func (af *AssetFile) Read(p []byte) (int, error) {
	return af.DataStream.Read(p)
}

func (af *AssetFile) Stat() (fs.FileInfo, error) {
	return af.FileInfo, nil
}

func (af *AssetFile) Info() (fs.FileInfo, error) {
	return af.FileInfo, nil
}

func (af *AssetFile) Type() fs.FileMode {
	return af.Mode()
}

// FileInfo captures the asset information and implements fs.FileInfo
type FileInfo struct {
	IName  string    `json:"name"` // base name of the file
	ISize  int64     `json:"size"` // length in bytes for regular files; system-dependent for others
	Ctime  time.Time `json:"created_at"`
	Mtime  time.Time `json:"updated_at"`
	IIsDir bool      `json:"isdir"`
}

// Name base name of the file
func (afd FileInfo) Name() string {
	return afd.IName
}

// Size length in bytes for regular files; system-dependent for others
func (afd FileInfo) Size() int64 {
	return afd.ISize
}

// Mode file mode bits
func (afd FileInfo) Mode() fs.FileMode {
	if afd.IIsDir {
		return fs.ModeDir
	}
	return fs.FileMode(0o0400)
}

// ModTime modification time
func (afd FileInfo) ModTime() time.Time {
	return afd.Mtime
}

// IsDir: abbreviation for Mode().IsDir()
func (afd FileInfo) IsDir() bool {
	return afd.IIsDir
}

func (afd FileInfo) Sys() any {
	return nil
}
