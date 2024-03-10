package misc

import (
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"

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

	fileInfo, err := os.Stat(file)
	if err != nil {
		// Check if there is a Zstd-compressed version of the file
		fileZstd := file + ".zst" // .zst is the extension used by linux-firmware
		fileInfoZstd, errZstd := os.Stat(fileZstd)

		if errZstd == nil {
			file = fileZstd
			fileInfo = fileInfoZstd
			// Unset nil so we don't retain the error from the os.Stat call for the uncompressed version.
			err = nil
		} else {
			if required {
				return files, fmt.Errorf("getFile: failed to stat file %q: %w (also tried %q: %w)", file, err, fileZstd, errZstd)
			}

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
