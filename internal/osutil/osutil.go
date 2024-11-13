package osutil

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Try to guess whether the system has merged dirs under /usr
func HasMergedUsr() bool {
	for _, dir := range []string{"/bin", "/lib"} {
		stat, err := os.Lstat(dir)
		if err != nil {
			// TODO: probably because the dir doesn't exist... so
			// should we assume that it's because the system has some weird
			// implementation of "merge /usr"?
			return true
		} else if stat.Mode()&os.ModeSymlink == 0 {
			// Not a symlink, so must not be merged /usr
			return false
		}
	}
	return true
}

// Converts given path to one supported by a merged /usr config.
// E.g., /bin/foo becomes /usr/bin/foo, /lib/bar becomes /usr/lib/bar
// See: https://www.freedesktop.org/wiki/Software/systemd/TheCaseForTheUsrMerge
func MergeUsr(file string) string {

	// Prepend /usr to supported paths
	for _, prefix := range []string{"/bin", "/sbin", "/lib", "/lib64"} {
		if strings.HasPrefix(file, prefix) {
			file = filepath.Join("/usr", file)
			break
		}
	}

	return file
}

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

func getKernelReleaseFile() (string, error) {
	files, _ := filepath.Glob("/usr/share/kernel/*/kernel.release")
	// only one kernel flavor supported
	if len(files) != 1 {
		return "", fmt.Errorf("only one kernel release/flavor is supported, found: %q", files)
	}

	return files[0], nil
}

func GetKernelVersion() (string, error) {
	var version string

	releaseFile, err := getKernelReleaseFile()
	if err != nil {
		return version, err
	}

	contents, err := os.ReadFile(releaseFile)
	if err != nil {
		return version, err
	}

	return strings.TrimSpace(string(contents)), nil
}
