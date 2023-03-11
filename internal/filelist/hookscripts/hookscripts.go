package hookscripts

import (
	"log"
	"os"
	"path/filepath"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist"
)

type HookScripts struct {
	destPath   string
	scriptsDir string
}

// New returns a new HookScripts that will use the given path to provide a list
// of script files. The destination for each script it set to destPath, using
// the original file name.
func New(scriptsDir string, destPath string) *HookScripts {
	return &HookScripts{
		destPath:   destPath,
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
		log.Printf("-- Including script: %s\n", path)
		files.Add(path, filepath.Join(h.destPath, file.Name()))
	}
	return files, nil
}
