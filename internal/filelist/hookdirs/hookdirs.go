package hookdirs

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist"
)

type HookDirs struct {
	path string
}

// New returns a new HookDirs that will use the given path to provide a list
// of directories use.
func New(path string) *HookDirs {
	return &HookDirs{
		path: path,
	}
}

func (h *HookDirs) List() (*filelist.FileList, error) {
	log.Printf("- Creating directories specified in %s", h.path)

	files := filelist.NewFileList()
	fileInfo, err := os.ReadDir(h.path)
	if err != nil {
		log.Println("-- Unable to find dir, skipping...")
		return files, nil
	}
	for _, file := range fileInfo {
		path := filepath.Join(h.path, file.Name())
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("getHookDirs: unable to open hook file: %w", err)

		}
		defer f.Close()
		log.Printf("-- Creating directories from: %s\n", path)

		s := bufio.NewScanner(f)
		for s.Scan() {
			dir := s.Text()
			files.Add(dir, dir)
		}
	}
	return files, nil
}
