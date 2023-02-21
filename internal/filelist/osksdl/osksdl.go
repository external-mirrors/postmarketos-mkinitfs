package osksdl

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
)

type OskSdl struct {
	mesaDriver string
}

// New returns a new HookScripts that will use the given path to provide a list
// of script files.
func New(mesaDriverName string) *OskSdl {
	return &OskSdl{
		mesaDriver: mesaDriverName,
	}
}

// Get a list of files and their dependencies related to supporting rootfs full
// disk (d)encryption
func (s *OskSdl) List() (*filelist.FileList, error) {
	files := filelist.NewFileList()

	if exists, err := misc.Exists("/usr/bin/osk-sdl"); !exists {
		return files, nil
	} else if err != nil {
		return files, fmt.Errorf("received unexpected error when getting status for %q: %w", "/usr/bin/osk-sdl", err)
	}

	log.Println("- Including osk-sdl support")

	confFiles := []string{
		"/etc/osk.conf",
		"/etc/ts.conf",
		"/etc/pointercal",
		"/etc/fb.modes",
		"/etc/directfbrc",
	}
	confFileList, err := misc.GetFiles(confFiles, false)
	if err != nil {
		return nil, fmt.Errorf("getFdeFiles: failed to add files: %w", err)
	}
	for _, file := range confFileList {
		files.Add(file, file)
	}

	// osk-sdl
	oskFiles := []string{
		"/usr/bin/osk-sdl",
		"/sbin/cryptsetup",
		"/usr/lib/libGL.so.1",
	}
	if oskFileList, err := misc.GetFiles(oskFiles, true); err != nil {
		return nil, fmt.Errorf("getFdeFiles: failed to add files: %w", err)
	} else {
		for _, file := range oskFileList {
			files.Add(file, file)
		}
	}

	fontFile, err := getOskConfFontPath("/etc/osk.conf")
	if err != nil {
		return nil, fmt.Errorf("getFdeFiles: failed to add file %q: %w", fontFile, err)
	}
	files.Add(fontFile, fontFile)

	// Directfb
	dfbFiles := []string{}
	err = filepath.Walk("/usr/lib/directfb-1.7-7", func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == ".so" {
			dfbFiles = append(dfbFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("getFdeFiles: failed to add file %w", err)
	}
	if dfbFileList, err := misc.GetFiles(dfbFiles, true); err != nil {
		return nil, fmt.Errorf("getFdeFiles: failed to add files: %w", err)
	} else {
		for _, file := range dfbFileList {
			files.Add(file, file)
		}
	}

	// tslib
	tslibFiles := []string{}
	err = filepath.Walk("/usr/lib/ts", func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == ".so" {
			tslibFiles = append(tslibFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("getFdeFiles: failed to add file: %w", err)
	}
	libts, _ := filepath.Glob("/usr/lib/libts*")
	tslibFiles = append(tslibFiles, libts...)
	if tslibFileList, err := misc.GetFiles(tslibFiles, true); err != nil {
		return nil, fmt.Errorf("getFdeFiles: failed to add files: %w", err)
	} else {
		for _, file := range tslibFileList {
			files.Add(file, file)
		}
	}

	// mesa hw accel
	if s.mesaDriver != "" {
		mesaFiles := []string{
			"/usr/lib/libEGL.so.1",
			"/usr/lib/libGLESv2.so.2",
			"/usr/lib/libgbm.so.1",
			"/usr/lib/libudev.so.1",
			"/usr/lib/xorg/modules/dri/" + s.mesaDriver + "_dri.so",
		}
		if mesaFileList, err := misc.GetFiles(mesaFiles, true); err != nil {
			return nil, fmt.Errorf("getFdeFiles: failed to add files: %w", err)
		} else {
			for _, file := range mesaFileList {
				files.Add(file, file)
			}
		}
	}

	return files, nil
}

func getOskConfFontPath(oskConfPath string) (string, error) {
	var path string
	f, err := os.Open(oskConfPath)
	if err != nil {
		return path, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		// "key = val" is 3 fields
		if len(fields) > 2 && fields[0] == "keyboard-font" {
			path = fields[2]
		}
	}
	if exists, err := misc.Exists(path); !exists {
		return path, fmt.Errorf("unable to find font: %s", path)
	} else if err != nil {
		return path, fmt.Errorf("received unexpected error when getting status for %q: %w", path, err)
	}

	return path, nil
}
