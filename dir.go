// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package ghrfs

import (
	"io/fs"
	"time"
)

// ReleaseDir implements the DirEntry interface abstracting the release as
// as directory.
type ReleaseDir struct {
	Tag        string
	Ctime      time.Time
	Mtime      time.Time
	AssetFiles []fs.DirEntry
	//FileInfo
}

func (rd *ReleaseDir) Close() error {
	return nil
}

func (rd *ReleaseDir) Read([]byte) (int, error) {
	return 0, nil
}

func (rd *ReleaseDir) Stat() (fs.FileInfo, error) {
	return rd.Info()
}

func (rd *ReleaseDir) Name() string {
	return rd.Tag
}

func (*ReleaseDir) IsDir() bool {
	return true
}

func Type() fs.FileMode {
	return fs.ModeDir
}

func (rd *ReleaseDir) Info() (FileInfo, error) {
	return FileInfo{
		IName: rd.Tag,
		ISize: 0,
		Ctime: rd.Ctime,
		Mtime: rd.Mtime,
	}, nil
}

func (rd *ReleaseDir) ReadDir(n int) ([]fs.DirEntry, error) {
	return rd.AssetFiles, nil
}
