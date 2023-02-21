// Copyright 2022 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package misc

import (
	"errors"
	"log"
	"os"
	"time"
)

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

// Prints the execution time of a function, not meant to be very
// sensitive/accurate, but good enough to gauge rough run times.
// Meant to be called as:
//
//	defer misc.TimeFunc(time.Now(), "foo")
func TimeFunc(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s completed in: %s", name, elapsed)
}

// Exists tests if the given file/dir exists or not. Returns any errors related
// to os.Stat if the type is *not* ErrNotExist. If an error is returned, then
// the value of the returned boolean cannot be trusted.
func Exists(file string) (bool, error) {
	_, err := os.Stat(file)
	if err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		// Don't return the error, the file doesn't exist which is OK
		return false, nil
	}

	// Other errors from os.Stat returned here
	return false, err
}
