package misc

import (
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"log"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/osutil"
)

func GetFiles(list []string, required bool) (files []string, err error) {
	for _, file := range list {
		filelist, err := getFile(file, required)
		if err != nil {
			return nil, err
		}
		files = append(files, filelist...)
	}

	files = RemoveDuplicates(files)
	return
}

// This function doesn't handle globs, use getFile() instead.
func getFileNormalized(file string, required bool) (files []string, err error) {
	fileInfo, err := os.Stat(file)

	// Trying some fallbacks...
	if err != nil {
		type triedResult struct {
			file string
			err error
		}

		triedFiles := make([]triedResult, 0, 1)

		// Temporary fallback until alpine/pmOS usr-merge happened
		// If a path starts with /bin or /sbin, also try /usr equivalent before giving up
		if strings.HasPrefix(file, "/bin/") || strings.HasPrefix(file, "/sbin/") {
			fileUsr := filepath.Join("/usr", file)
			_, err := os.Stat(fileUsr);
			if err == nil {
				log.Printf("getFile: failed to find %q, but found it in %q. Please adjust the path.", file, fileUsr)
				return getFileNormalized(fileUsr, required)
			} else {
				triedFiles = append(triedFiles, triedResult{fileUsr, err})
			}
		}

		{
			// Check if there is a Zstd-compressed version of the file
			fileZstd := file + ".zst" // .zst is the extension used by linux-firmware
			_, err := os.Stat(fileZstd);
			if err == nil {
				return getFileNormalized(fileZstd, required)
			} else {
				triedFiles = append(triedFiles, triedResult{fileZstd, err})
			}
		}
		
		// Failed to find anything
		if required {
			failStrings := make([]string, 0, 2)
			for _, result := range triedFiles {
				failStrings = append(failStrings, fmt.Sprintf("\n - also tried %q: %v", result.file, result.err))
			}
			return files, fmt.Errorf("getFile: failed to stat file %q: %v%q", file, err, strings.Join(failStrings, ""))
		} else {
			return files, nil
		}
	}

	if fileInfo.IsDir() {
		// Recurse over directory contents
		err := filepath.Walk(file, func(path string, f os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if f.IsDir() {
				return nil
			}
			newFiles, err := getFile(path, required)
			if err != nil {
				return err
			}
			files = append(files, newFiles...)
			return nil
		})
		if err != nil {
			return files, err
		}
	} else {
		files = append(files, file)

		// get dependencies for binaries
		if _, err := elf.Open(file); err == nil {
			if binaryDepFiles, err := getBinaryDeps(file); err != nil {
				return files, err
			} else {
				files = append(files, binaryDepFiles...)
			}
		}
	}

	files = RemoveDuplicates(files)
	return
}

func getFile(file string, required bool) (files []string, err error) {
	// Expand glob expression
	expanded, err := filepath.Glob(file)
	if err != nil {
		return
	}
	if len(expanded) > 0 && expanded[0] != file {
		for _, path := range expanded {
			if globFiles, err := getFile(path, required); err != nil {
				return files, err
			} else {
				files = append(files, globFiles...)
			}
		}
		return RemoveDuplicates(files), nil
	}

	return getFileNormalized(file, required)
}

func getDeps(file string, parents map[string]struct{}) (files []string, err error) {

	if _, found := parents[file]; found {
		return
	}

	// get dependencies for binaries
	fd, err := elf.Open(file)
	if err != nil {
		return nil, fmt.Errorf("getDeps: unable to open elf binary %q: %w", file, err)
	}
	libs, _ := fd.ImportedLibraries()
	fd.Close()
	files = append(files, file)
	parents[file] = struct{}{}

	if len(libs) == 0 {
		return
	}

	// we don't recursively search these paths for performance reasons
	libdirGlobs := []string{
		"/usr/lib",
		"/lib",
		"/usr/lib/expect*",
	}

	for _, lib := range libs {
		found := false
	findDepLoop:
		for _, libdirGlob := range libdirGlobs {
			libdirs, _ := filepath.Glob(libdirGlob)
			for _, libdir := range libdirs {
				path := filepath.Join(libdir, lib)
				if _, err := os.Stat(path); err == nil {
					binaryDepFiles, err := getDeps(path, parents)
					if err != nil {
						return nil, err
					}
					files = append(files, binaryDepFiles...)
					files = append(files, path)
					found = true
					break findDepLoop
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("getDeps: unable to locate dependency for %q: %s", file, lib)
		}
	}

	return
}

// Recursively list all dependencies for a given ELF binary
func getBinaryDeps(file string) ([]string, error) {
	// if file is a symlink, resolve dependencies for target
	fileStat, err := os.Lstat(file)
	if err != nil {
		return nil, fmt.Errorf("getBinaryDeps: failed to stat file %q: %w", file, err)
	}

	// Symlink: write symlink to archive then set 'file' to link target
	if fileStat.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(file)
		if err != nil {
			return nil, fmt.Errorf("getBinaryDeps: unable to read symlink %q: %w", file, err)
		}
		if !filepath.IsAbs(target) {
			target, err = osutil.RelativeSymlinkTargetToDir(target, filepath.Dir(file))
			if err != nil {
				return nil, err
			}
		}
		file = target
	}

	return getDeps(file, make(map[string]struct{}))

}
