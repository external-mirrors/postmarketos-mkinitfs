package hookscripts

import (
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
	log.Printf("- Searching for hook scripts from %s", h.scriptsDir)

	files := filelist.NewFileList()

	fileInfo, err := os.ReadDir(h.scriptsDir)
	if err != nil {
		log.Println("-- Unable to find dir, skipping...")
		return files, nil
	}
	for _, file := range fileInfo {
		path := filepath.Join(h.scriptsDir, file.Name())
		files.Add(path, path)
	}
	return files, nil
}
