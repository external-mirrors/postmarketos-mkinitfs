// Copyright 2022 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package misc

import (
	"golang.org/x/sys/unix"
	"log"
	"os"
	"path/filepath"
)

// Converts a relative symlink target path (e.g. ../../lib/foo.so), that is
// absolute path
func RelativeSymlinkTargetToDir(symPath string, dir string) (string, error) {
	var path string

	oldWd, err := os.Getwd()
	if err != nil {
		log.Print("Unable to get current working dir")
		return path, err
	}

	if err := os.Chdir(dir); err != nil {
		log.Print("Unable to change to working dir: ", dir)
		return path, err
	}

	path, err = filepath.Abs(symPath)
	if err != nil {
		log.Print("Unable to resolve abs path to: ", symPath)
		return path, err
	}

	if err := os.Chdir(oldWd); err != nil {
		log.Print("Unable to change to old working dir")
		return path, err
	}

	return path, nil
}

func FreeSpace(path string) (uint64, error) {
	var stat unix.Statfs_t
	unix.Statfs(path, &stat)
	size := stat.Bavail * uint64(stat.Bsize)
	return size, nil
}

// Merge the contents of "b" into "a", overwriting any previously existing keys
// in "a"
func Merge(a map[string]string, b map[string]string) {
	for k, v := range b {
		a[k] = v
	}
}

// Removes duplicate entries from the given string slice and returns a slice
// with the unique values
func RemoveDuplicates(in []string) (out []string) {
	// use a map to "remove" duplicates. the value in the map is totally
	// irrelevant
	outMap := make(map[string]bool)
	for _, s := range in {
		if ok := outMap[s]; !ok {
			outMap[s] = true
		}
	}

	out = make([]string, 0, len(outMap))
	for k := range outMap {
		out = append(out, k)
	}

	return
}
