package hookfiles

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/osutil"
)

type HookFiles struct {
	filePath string
}

// New returns a new HookFiles that will use the given path to provide a list
// of files + any binary dependencies they might have.
func New(filePath string) *HookFiles {
	return &HookFiles{
		filePath: filePath,
	}
}

func (h *HookFiles) List() (*filelist.FileList, error) {
	log.Printf("- Searching for file lists from %s", h.filePath)

	files := filelist.NewFileList()
	fileInfo, err := os.ReadDir(h.filePath)
	if err != nil {
		log.Println("-- Unable to find dir, skipping...")
		return files, nil
	}
	for _, file := range fileInfo {
		path := filepath.Join(h.filePath, file.Name())
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("getHookFiles: unable to open hook file: %w", err)

		}
		defer f.Close()
		log.Printf("-- Including files from: %s\n", path)

		if list, err := slurpFiles(f); err != nil {
			return nil, fmt.Errorf("hookfiles: unable to process hook file %q: %w", path, err)
		} else {
			files.Import(list)
		}
	}
	return files, nil
}

func slurpFiles(fd io.Reader) (*filelist.FileList, error) {
	files := filelist.NewFileList()

	s := bufio.NewScanner(fd)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		src, dest, has_dest, is_optional := stripSuffix(line)
		if osutil.HasMergedUsr() {
			src = osutil.MergeUsr(src)
		}

		fFiles, err := misc.GetFiles([]string{src}, true)
		if err != nil {
			// Ignore missing optional files, otherwise fail
			if is_optional {
				log.Println("--- Unable to find optional path, skipping...")
			} else {
				return nil, fmt.Errorf("unable to add %q: %w", src, err)
			}
		}
		// loop over all returned files from GetFile
		for _, file := range fFiles {
			if !has_dest {
				files.Add(file, file)
			} else if len(fFiles) > 1 {
				// Don't support specifying dest if src was a glob
				// NOTE: this could support this later...
				files.Add(file, file)
			} else {
				// dest path specified, and only 1 file
				files.Add(file, dest)
			}
		}
	}

	return files, s.Err()
}

func stripSuffix(line string) (string, string, bool, bool) {
	// Ignore the second return as it will always be "option" or nothing
	option_src, _, is_optional := strings.Cut(line, "!")
	src, dest, has_dest := strings.Cut(option_src, ":")
	return src, dest, has_dest, is_optional
}
