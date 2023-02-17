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

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
)

type Modules struct {
	modules []string
}

// New returns a new HookScripts that will use the given path to provide a list
// of script files.
func New(modules []string) *Modules {
	return &Modules{
		modules: modules,
	}
}

func (m *Modules) List() ([]string, error) {
	kernVer, err := misc.GetKernelVersion()
	if err != nil {
		return nil, err
	}

	files := []string{}

	modDir := filepath.Join("/lib/modules", kernVer)
	if !misc.Exists(modDir) {
		// dir /lib/modules/<kernel> if kernel built without module support, so just print a message
		log.Printf("-- kernel module directory not found: %q, not including modules", modDir)
		return files, nil
	}

	// modules.* required by modprobe
	modprobeFiles, _ := filepath.Glob(filepath.Join(modDir, "modules.*"))
	files = append(files, modprobeFiles...)

	// module name (without extension), or directory (trailing slash is important! globs OK)
	requiredModules := []string{
		"loop",
		"dm-crypt",
		"kernel/fs/overlayfs/",
		"kernel/crypto/",
		"kernel/arch/*/crypto/",
	}

	for _, item := range requiredModules {
		dir, file := filepath.Split(item)
		if file == "" {
			// item is a directory
			dir = filepath.Join(modDir, dir)
			dirs, _ := filepath.Glob(dir)
			for _, d := range dirs {
				if filelist, err := getModulesInDir(d); err != nil {
					return nil, fmt.Errorf("getInitfsModules: unable to get modules dir %q: %w", d, err)
				} else {
					files = append(files, filelist...)
				}
			}
		} else if dir == "" {
			// item is a module name
			if filelist, err := getModule(file, modDir); err != nil {
				return nil, fmt.Errorf("getInitfsModules: unable to get module %q: %w", file, err)
			} else {
				files = append(files, filelist...)
			}
		} else {
			log.Printf("Unknown module entry: %q", item)
		}
	}

	// deviceinfo modules
	for _, module := range m.modules {
		if filelist, err := getModule(module, modDir); err != nil {
			return nil, fmt.Errorf("getInitfsModules: unable to get modules from deviceinfo: %w", err)
		} else {
			files = append(files, filelist...)
		}
	}

	// /etc/postmarketos-mkinitfs/modules/*.modules
	initfsModFiles, _ := filepath.Glob("/etc/postmarketos-mkinitfs/modules/*.modules")
	for _, modFile := range initfsModFiles {
		f, err := os.Open(modFile)
		if err != nil {
			return nil, fmt.Errorf("getInitfsModules: unable to open mkinitfs modules file %q: %w", modFile, err)
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			if filelist, err := getModule(s.Text(), modDir); err != nil {
				return nil, fmt.Errorf("getInitfsModules: unable to get module file %q: %w", s.Text(), err)
			} else {
				files = append(files, filelist...)
			}
		}
	}

	return files, nil
}

func getModulesInDir(modPath string) (files []string, err error) {
	err = filepath.Walk(modPath, func(path string, f os.FileInfo, err error) error {
		// TODO: need to support more extensions?
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
// TODO: look for it in modules.builtin, and make it fatal if it can't be found
// anywhere
func getModule(modName string, modDir string) (files []string, err error) {

	modDep := filepath.Join(modDir, "modules.dep")
	if !misc.Exists(modDep) {
		return nil, fmt.Errorf("kernel module.dep not found: %s", modDir)
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
		if !misc.Exists(p) {
			return nil, fmt.Errorf("tried to include a module that doesn't exist in the modules directory (%s): %s", modDir, p)
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
