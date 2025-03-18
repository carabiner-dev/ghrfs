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
	fmt.Println("Test File Contents:")
	if _, err := io.Copy(os.Stdout, file); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
