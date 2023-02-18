package hookscripts

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist"
)

type HookScripts struct {
	scriptsDir string
}

// New returns a new HookScripts that will use the given path to provide a list
// of script files.
func New(scriptsDir string) *HookScripts {
	return &HookScripts{
		scriptsDir: scriptsDir,
	}
}

func (h *HookScripts) List() (*filelist.FileList, error) {
	files := filelist.NewFileList()
	log.Println("- Including hook scripts")

	fileInfo, err := os.ReadDir(h.scriptsDir)
	if err != nil {
		return nil, fmt.Errorf("getHookScripts: unable to read hook script dir: %w", err)
	}
	for _, file := range fileInfo {
		path := filepath.Join(h.scriptsDir, file.Name())
		files.Add(path, path)
	}
	return files, nil
}
