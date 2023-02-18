package hookfiles

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
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

func (h *HookFiles) List() ([]string, error) {
	log.Println("- Including files")
	fileInfo, err := os.ReadDir(h.filePath)
	if err != nil {
		return nil, fmt.Errorf("getHookFiles: unable to read hook file dir: %w", err)
	}
	files := []string{}
	for _, file := range fileInfo {
		path := filepath.Join(h.filePath, file.Name())
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("getHookFiles: unable to open hook file: %w", err)

		}
		defer f.Close()
		log.Printf("-- Including files from: %s\n", path)
		s := bufio.NewScanner(f)
		for s.Scan() {
			if filelist, err := misc.GetFiles([]string{s.Text()}, true); err != nil {
				return nil, fmt.Errorf("getHookFiles: unable to add file %q required by %q: %w", s.Text(), path, err)
			} else {
				files = append(files, filelist...)
			}
		}
		if err := s.Err(); err != nil {
			return nil, fmt.Errorf("getHookFiles: uname to process hook file %q: %w", path, err)
		}
	}
	return files, nil
}
