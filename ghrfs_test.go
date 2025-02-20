// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package ghrfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCacheRelease(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		release  *ReleaseData
		mustErr  bool
		prepare  func(*testing.T, *Options, *ReleaseData)
		validate func(*testing.T, *Options, *ReleaseData)
	}{
		{
			"normal", &ReleaseData{}, false,
			func(t *testing.T, o *Options, rd *ReleaseData) {
				t.Helper()

				f1, err := os.Create(filepath.Join(o.CachePath, "src-test1.txt"))
				require.NoError(t, err)
				_, err = f1.Write([]byte("test1"))
				require.NoError(t, err)
				_, err = f1.Seek(0, 0)
				require.NoError(t, err)

				rd.Assets = append(rd.Assets, &AssetFile{
					FileInfo:   FileInfo{IName: "test1.txt", ISize: int64(len("test1"))},
					DataStream: f1,
				})

				f2, err := os.Create(filepath.Join(o.CachePath, "src-test2.txt"))
				require.NoError(t, err)
				_, err = f2.Write([]byte("test2"))
				require.NoError(t, err)
				_, err = f2.Seek(0, 0)
				require.NoError(t, err)

				rd.Assets = append(rd.Assets, &AssetFile{
					FileInfo:   FileInfo{IName: "test2.txt", ISize: int64(len("test2"))},
					DataStream: f2,
				})
			},
			func(t *testing.T, o *Options, rd *ReleaseData) {
				t.Helper()
				require.FileExists(t, filepath.Join(o.CachePath, releaseDataFile))
			},
		},
		// {"normal", &ReleaseData{}, func(t *testing.T, o *Options, rd *ReleaseData) {}, func(t *testing.T, o *Options, rd *ReleaseData) {}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmp := t.TempDir()
			rfs := &ReleaseFileSystem{
				Options: Options{
					Cache:             true,
					CachePath:         tmp,
					ParallelDownloads: defaultOptions.ParallelDownloads,
				},
				Release: *tc.release,
			}

			tc.prepare(t, &rfs.Options, &rfs.Release)

			// Build the cache
			err := rfs.CacheRelease()
			if tc.mustErr {
				require.Error(t, err)
				return
			}

			tc.validate(t, &rfs.Options, &rfs.Release)
			for _, a := range rfs.Release.Assets {
				require.NotEmpty(t, a.Name())
				require.FileExists(t, filepath.Join(rfs.Options.CachePath, a.Name()))
				info, err := os.Stat(filepath.Join(rfs.Options.CachePath, a.Name()))
				require.NoError(t, err)
				require.Equal(t, info.Size(), a.Size())
			}
		})
	}
}
