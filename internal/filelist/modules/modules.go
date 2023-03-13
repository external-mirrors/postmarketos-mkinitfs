package modules

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/osutil"
)

type Modules struct {
	modulesList     []string
	modulesListPath string
}

// New returns a new Modules that will use the given moduleto provide a list
// of script files.
func New(modulesList []string, modulesListPath string) *Modules {
	return &Modules{
		modulesList:     modulesList,
		modulesListPath: modulesListPath,
	}
}

func (m *Modules) List() (*filelist.FileList, error) {
	kernVer, err := osutil.GetKernelVersion()
	if err != nil {
		return nil, err
	}

	files := filelist.NewFileList()

	modDir := filepath.Join("/lib/modules", kernVer)
	if exists, err := misc.Exists(modDir); !exists {
		// dir /lib/modules/<kernel> if kernel built without module support, so just print a message
		log.Printf("-- kernel module directory not found: %q, not including modules", modDir)
		return files, nil
	} else if err != nil {
		return nil, fmt.Errorf("received unexpected error when getting status for %q: %w", modDir, err)
	}

	// modules.* required by modprobe
	modprobeFiles, _ := filepath.Glob(filepath.Join(modDir, "modules.*"))
	for _, file := range modprobeFiles {
		files.Add(file, file)
	}

	// slurp up given list of modules
	for _, module := range m.modulesList {
		if modFilelist, err := getModule(module, modDir); err != nil {
			return nil, fmt.Errorf("unable to get modules from deviceinfo: %w", err)
		} else {
			for _, file := range modFilelist {
				files.Add(file, file)
			}
		}
	}

	// slurp up modules from lists in modulesListPath
	log.Printf("- Searching for kernel modules from %s", m.modulesListPath)
	fileInfo, err := os.ReadDir(m.modulesListPath)
	if err != nil {
		log.Println("-- Unable to find dir, skipping...")
		return files, nil
	}
	for _, file := range fileInfo {
		path := filepath.Join(m.modulesListPath, file.Name())
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("unable to open module list file %q: %w", path, err)
		}
		defer f.Close()
		log.Printf("-- Including modules from: %s\n", path)

		if list, err := slurpModules(f, modDir); err != nil {
			return nil, fmt.Errorf("unable to process module list file %q: %w", path, err)
		} else {
			files.Import(list)
		}
	}
	return files, nil
}

func slurpModules(fd io.Reader, modDir string) (*filelist.FileList, error) {
	files := filelist.NewFileList()
	s := bufio.NewScanner(fd)
	for s.Scan() {
		line := s.Text()
		dir, file := filepath.Split(line)
		if file == "" {
			// item is a directory
			dir = filepath.Join(modDir, dir)
			dirs, _ := filepath.Glob(dir)
			for _, d := range dirs {
				if modFilelist, err := getModulesInDir(d); err != nil {
					return nil, fmt.Errorf("unable to get modules dir %q: %w", d, err)
				} else {
					for _, file := range modFilelist {
						files.Add(file, file)
					}
				}
			}
		} else if dir == "" {
			// item is a module name
			if modFilelist, err := getModule(s.Text(), modDir); err != nil {
				return nil, fmt.Errorf("unable to get module file %q: %w", s.Text(), err)
			} else {
				for _, file := range modFilelist {
					files.Add(file, file)
				}
			}
		} else {
			log.Printf("Unknown module entry: %q", line)
		}
	}

	return files, s.Err()
}

func getModulesInDir(modPath string) (files []string, err error) {
	err = filepath.Walk(modPath, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			// Unable to walk path
			return err
		}
		if filepath.Ext(path) != ".ko" && filepath.Ext(path) != ".xz" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return
}

// Given a module name, e.g. 'dwc_wdt', resolve the full path to the module
// file and all of its dependencies.
// Note: it's not necessarily fatal if the module is not found, since it may
// have been built into the kernel
func getModule(modName string, modDir string) (files []string, err error) {

	modDep := filepath.Join(modDir, "modules.dep")
	if exists, err := misc.Exists(modDep); !exists {
		return nil, fmt.Errorf("kernel module.dep not found: %s", modDir)
	} else if err != nil {
		return nil, fmt.Errorf("received unexpected error when getting module.dep status: %w", err)
	}

	fd, err := os.Open(modDep)
	if err != nil {
		return nil, fmt.Errorf("unable to open modules.dep: %w", err)
	}
	defer fd.Close()

	deps, err := getModuleDeps(modName, fd)
	if err != nil {
		return nil, err
	}

	for _, dep := range deps {
		p := filepath.Join(modDir, dep)
		if exists, err := misc.Exists(p); !exists {
			return nil, fmt.Errorf("tried to include a module that doesn't exist in the modules directory (%s): %s", modDir, p)
		} else if err != nil {
			return nil, fmt.Errorf("received unexpected error when getting status for %q: %w", p, err)
		}

		files = append(files, p)
	}

	return
}

// Get the canonicalized name for the module as represented in the given modules.dep io.reader
func getModuleDeps(modName string, modulesDep io.Reader) ([]string, error) {
	var deps []string

	// split the module name on - and/or _, build a regex for matching
	splitRe := regexp.MustCompile("[-_]+")
	modNameReStr := splitRe.ReplaceAllString(modName, "[-_]+")
	re := regexp.MustCompile("^" + modNameReStr + "$")

	s := bufio.NewScanner(modulesDep)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) == 0 {
			continue
		}
		fields[0] = strings.TrimSuffix(fields[0], ":")

		found := re.FindAll([]byte(filepath.Base(stripExts(fields[0]))), -1)
		if len(found) > 0 {
			deps = append(deps, fields...)
			break
		}
	}
	if err := s.Err(); err != nil {
		log.Print("Unable to get module + dependencies: ", modName)
		return deps, err
	}

	return deps, nil
}

func stripExts(file string) string {
	return strings.Split(file, ".")[0]
}
