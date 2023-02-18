package hookscripts

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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

func (h *HookScripts) List() ([]string, error) {
	files := []string{}
	log.Println("- Including hook scripts")

	fileInfo, err := os.ReadDir(h.scriptsDir)
	if err != nil {
		return nil, fmt.Errorf("getHookScripts: unable to read hook script dir: %w", err)
	}
	for _, file := range fileInfo {
		path := filepath.Join(h.scriptsDir, file.Name())
		files = append(files, path)
	}
	return files, nil
}
