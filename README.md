# ghrfs: The GitHub Release File System

A go fs.FS driver that reads from GitHub releases.

## Description

ghrfs (short for GitHub Release File System) is a Go [fs.FS](https://pkg.go.dev/io/fs)
implementation that reads data from a GitHub release.

The `ReleaseFileystem` object emulates a read-only filesystem by reading from 
GitHub's API or from a local cache for faster access with less bandwidth consumption
or to support switching to airgapped environments.

## Install 

```bash
go get github.com/carabiner-dev/ghrfs
```
## Usage

### Authentication

The filesystems requires GitHub credentials to access the GitHub API. By default
it will look for an API token in the `GITHUB_TOKEN` environment variable. `ghrfs`
is based on [carabiner-dev/github](https://github.com/carabiner-dev/github) which
means you should be able to use any token provider that the client supports.

### Example Use

To use the filesystem, simply initialize a new instance and use with anything that
takes a `fs.FS`:

```golang
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/carabiner-dev/ghrfs"
)

func main() {
	// Create a new GitHub Release File System:
	rfs, err := ghrfs.New(
		ghrfs.FromURL("https://github.com/carabiner-dev/ghrfs/releases/tag/v0.0.0"),
	)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// Open a file from the release:
	file, err := rfs.Open("about-this-release.txt")
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// Get information about the file:
	info, err := file.Stat()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// Do something with the file:
	fmt.Printf("%s atrributes: %+v\n", info.Name(), info)
	fmt.Println("File contents:")
	io.Copy(os.Stdout, file)
}
```

## Contribute!

This module is released under the Apache 2.0 license. Feel free to contribute
improvements and report any problems you find by creating issues.
