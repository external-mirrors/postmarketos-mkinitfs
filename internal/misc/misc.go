// Copyright 2022 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package misc

import (
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

// Exists tests if the given file/dir exists or not
func Exists(file string) bool {
	if _, err := os.Stat(file); err == nil {
		return true
	}
	return false
}
