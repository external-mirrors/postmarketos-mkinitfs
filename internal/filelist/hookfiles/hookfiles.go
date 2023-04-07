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
		src, dest, has_dest := strings.Cut(s.Text(), ":")

		if !has_dest {
			dest = src
		}

		err := files.AddGlobbed(src, dest)
		if err != nil {
			return nil, err
		}
	}

	return files, s.Err()
}
