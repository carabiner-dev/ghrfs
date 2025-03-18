package main

import (
	"fmt"
	"io"
	"io/fs"
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

	fmt.Print("Walking filesystem:\n")
	// Walk the filesystem
	if err := fs.WalkDir(rfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("scanning at %s: %w", path, err)
		}

		if d.IsDir() {
			fmt.Printf("%s/ (%s)\n", path, d.Name())
			return nil
		}

		// Read the file data from the filesystem
		info, err := fs.Stat(rfs, path)
		if err != nil {
			return fmt.Errorf("reading file from fs: %w", err)
		}

		fmt.Printf("  %s (%d bytes)\n", path, info.Size())

		return nil
	}); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Print("\nOpening Test File:\n")
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
	fmt.Printf("  %s atrributes:\n  %+v\n", info.Name(), info)
	fmt.Println("\nTest File Contents:\n")
	if _, err := io.Copy(os.Stdout, file); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
