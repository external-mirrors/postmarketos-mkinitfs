// Copyright 2021 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"debug/elf"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/pkgs/archive"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/pkgs/deviceinfo"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/pkgs/misc"
)

func timeFunc(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s completed in: %s", name, elapsed)
}

func main() {
	deviceinfoFile := "/etc/deviceinfo"
	if !exists(deviceinfoFile) {
		log.Print("NOTE: deviceinfo (from device package) not installed yet, " +
			"not building the initramfs now (it should get built later " +
			"automatically.)")
		return
	}

	devinfo, err := deviceinfo.ReadDeviceinfo(deviceinfoFile)
	if err != nil {
		log.Fatal(err)
	}

	outDir := flag.String("d", "/boot", "Directory to output initfs(-extra) and other boot files")
	flag.Parse()

	defer timeFunc(time.Now(), "mkinitfs")

	kernVer, err := getKernelVersion()
	if err != nil {
		log.Fatal(err)
	}

	// temporary working dir
	workDir, err := ioutil.TempDir("", "mkinitfs")
	if err != nil {
		log.Fatal("Unable to create temporary work directory:", err)
	}
	defer os.RemoveAll(workDir)

	log.Print("Generating for kernel version: ", kernVer)
	log.Print("Output directory: ", *outDir)

	if err := generateInitfs("initramfs", workDir, kernVer, devinfo); err != nil {
		log.Fatal("generateInitfs: ", err)
	}

	if err := generateInitfsExtra("initramfs-extra", workDir, devinfo); err != nil {
		log.Fatal("generateInitfsExtra: ", err)
	}

	if err := copyUbootFiles(workDir, devinfo); errors.Is(err, os.ErrNotExist) {
		log.Println("u-boot files copying skipped: ", err)
	} else {
		if err != nil {
			log.Fatal("copyUbootFiles: ", err)
		}
	}

	// Final processing of initramfs / kernel is done by boot-deploy
	if err := bootDeploy(workDir, *outDir); err != nil {
		log.Fatal("bootDeploy: ", err)
	}

}

func bootDeploy(workDir string, outDir string) error {
	// boot-deploy expects the kernel to be in the same dir as initramfs.
	// Assume that the kernel is in the output dir...
	log.Print("== Using boot-deploy to finalize/install files ==")
	kernels, _ := filepath.Glob(filepath.Join(outDir, "vmlinuz*"))
	if len(kernels) == 0 {
		return errors.New("Unable to find any kernels at " + filepath.Join(outDir, "vmlinuz*"))
	}

	// Pick a kernel that does not have suffixes added by boot-deploy
	var kernFile string
	for _, f := range kernels {
		if strings.HasSuffix(f, "-dtb") || strings.HasSuffix(f, "-mtk") {
			continue
		}
		kernFile = f
		break
	}

	kernFd, err := os.Open(kernFile)
	if err != nil {
		return err
	}
	defer kernFd.Close()

	kernFileCopy, err := os.Create(filepath.Join(workDir, "vmlinuz"))
	if err != nil {
		return err
	}

	if _, err = io.Copy(kernFileCopy, kernFd); err != nil {
		return err
	}
	kernFileCopy.Close()

	// boot-deploy -i initramfs -k vmlinuz-postmarketos-rockchip -d /tmp/cpio -o /tmp/foo initramfs-extra
	cmd := exec.Command("boot-deploy",
		"-i", "initramfs",
		"-k", "vmlinuz",
		"-d", workDir,
		"-o", outDir,
		"initramfs-extra")
	if !exists(cmd.Path) {
		return errors.New("boot-deploy command not found")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Print("'boot-deploy' command failed")
		return err
	}

	return nil
}

func exists(file string) bool {
	if _, err := os.Stat(file); err == nil {
		return true
	}
	return false
}

func getHookFiles(filesdir string) misc.StringSet {
	fileInfo, err := ioutil.ReadDir(filesdir)
	if err != nil {
		log.Fatal(err)
	}
	files := make(misc.StringSet)
	for _, file := range fileInfo {
		path := filepath.Join(filesdir, file.Name())
		f, err := os.Open(path)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			if err := getFile(files, s.Text(), true); err != nil {
				log.Fatalf("Unable to find file %q required by %q", s.Text(), path)
			}
		}
		if err := s.Err(); err != nil {
			log.Fatal(err)
		}
	}
	return files
}

// Recursively list all dependencies for a given ELF binary
func getBinaryDeps(files misc.StringSet, file string) error {
	// if file is a symlink, resolve dependencies for target
	fileStat, err := os.Lstat(file)
	if err != nil {
		log.Print("getBinaryDeps: failed to stat file")
		return err
	}

	// Symlink: write symlink to archive then set 'file' to link target
	if fileStat.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(file)
		if err != nil {
			log.Print("getBinaryDeps: unable to read symlink: ", file)
			return err
		}
		if !filepath.IsAbs(target) {
			target, err = misc.RelativeSymlinkTargetToDir(target, filepath.Dir(file))
			if err != nil {
				return err
			}
		}
		if err := getBinaryDeps(files, target); err != nil {
			return err
		}
		return err
	}

	// get dependencies for binaries
	fd, err := elf.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	libs, _ := fd.ImportedLibraries()
	fd.Close()
	files[file] = false

	if len(libs) == 0 {
		return err
	}

	libdirs := []string{"/usr/lib", "/lib"}
	for _, lib := range libs {
		found := false
		for _, libdir := range libdirs {
			path := filepath.Join(libdir, lib)
			if _, err := os.Stat(path); err == nil {
				err := getBinaryDeps(files, path)
				if err != nil {
					return err
				}
				files[path] = false
				found = true
				break
			}
		}
		if !found {
			log.Fatalf("Unable to locate dependency for %q: %s", file, lib)
		}
	}

	return nil
}

func getFiles(files misc.StringSet, newFiles misc.StringSet, required bool) error {
	for file := range newFiles {
		err := getFile(files, file, required)
		if err != nil {
			return err
		}
	}
	return nil
}

func getFile(files misc.StringSet, file string, required bool) error {
	// Expand glob expression
	expanded, _ := filepath.Glob(file)
	if len(expanded) > 0 && expanded[0] != file {
		for _, path := range expanded {
			if err := getFile(files, path, required); err != nil {
				return err
			}
		}
		return nil
	}

	fileInfo, err := os.Stat(file)
	if err != nil {
		if required {
			return errors.New("getFile: File does not exist :" + file)
		}
		return nil
	}

	if fileInfo.IsDir() {
		// Recurse over directory contents
		err := filepath.Walk(file, func(path string, f os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if f.IsDir() {
				return nil
			}
			return getFile(files, path, required)
		})
		if err != nil {
			return err
		}
		return nil
	}

	files[file] = false

	// get dependencies for binaries
	if _, err := elf.Open(file); err != nil {
		// file is not an elf, so don't resolve lib dependencies
		return nil
	}

	if err := getBinaryDeps(files, file); err != nil {
		return err
	}

	return nil
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
	if !exists(path) {
		return path, errors.New("Unable to find font: " + path)
	}

	return path, nil
}

// Get a list of files and their dependencies related to supporting rootfs full
// disk (d)encryption
func getFdeFiles(files misc.StringSet, devinfo deviceinfo.DeviceInfo) error {
	confFiles := misc.StringSet{
		"/etc/osk.conf":   false,
		"/etc/ts.conf":    false,
		"/etc/pointercal": false,
		"/etc/fb.modes":   false,
		"/etc/directfbrc": false,
	}
	// TODO: this shouldn't be false? though some files (pointercal) don't always exist...
	if err := getFiles(files, confFiles, false); err != nil {
		return err
	}

	// osk-sdl
	oskFiles := misc.StringSet{
		"/usr/bin/osk-sdl":    false,
		"/sbin/cryptsetup":    false,
		"/usr/lib/libGL.so.1": false}
	if err := getFiles(files, oskFiles, true); err != nil {
		return err
	}

	fontFile, err := getOskConfFontPath("/etc/osk.conf")
	if err != nil {
		return err
	}
	files[fontFile] = false

	// Directfb
	dfbFiles := make(misc.StringSet)
	err = filepath.Walk("/usr/lib/directfb-1.7-7", func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == ".so" {
			dfbFiles[path] = false
		}
		return nil
	})
	if err != nil {
		log.Print("getBinaryDeps: failed to stat file")
		return err
	}
	if err := getFiles(files, dfbFiles, true); err != nil {
		return err
	}

	// tslib
	tslibFiles := make(misc.StringSet)
	err = filepath.Walk("/usr/lib/ts", func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == ".so" {
			tslibFiles[path] = false
		}
		return nil
	})
	if err != nil {
		log.Print("getBinaryDeps: failed to stat file")
		return err
	}
	libts, _ := filepath.Glob("/usr/lib/libts*")
	for _, file := range libts {
		tslibFiles[file] = false
	}
	if err = getFiles(files, tslibFiles, true); err != nil {
		return err
	}

	// mesa hw accel
	if devinfo.MesaDriver != "" {
		mesaFiles := misc.StringSet{
			"/usr/lib/libEGL.so.1":    false,
			"/usr/lib/libGLESv2.so.2": false,
			"/usr/lib/libgbm.so.1":    false,
			"/usr/lib/libudev.so.1":   false,
			"/usr/lib/xorg/modules/dri/" + devinfo.MesaDriver + "_dri.so": false,
		}
		if err := getFiles(files, mesaFiles, true); err != nil {
			return err
		}
	}

	return nil
}

func getHookScripts(files misc.StringSet) {
	scripts, _ := filepath.Glob("/etc/postmarketos-mkinitfs/hooks/*.sh")
	for _, script := range scripts {
		files[script] = false
	}
}

func getInitfsExtraFiles(files misc.StringSet, devinfo deviceinfo.DeviceInfo) error {
	log.Println("== Generating initramfs extra ==")
	binariesExtra := misc.StringSet{
		"/lib/libz.so.1":        false,
		"/sbin/btrfs":           false,
		"/sbin/dmsetup":         false,
		"/sbin/e2fsck":          false,
		"/usr/sbin/parted":      false,
		"/usr/sbin/resize2fs":   false,
		"/usr/sbin/resize.f2fs": false,
	}
	log.Println("- Including extra binaries")
	if err := getFiles(files, binariesExtra, true); err != nil {
		return err
	}

	if exists("/usr/bin/osk-sdl") {
		log.Println("- Including FDE support")
		if err := getFdeFiles(files, devinfo); err != nil {
			return err
		}
	} else {
		log.Println("- *NOT* including FDE support")
	}

	return nil
}

func getInitfsFiles(files misc.StringSet, devinfo deviceinfo.DeviceInfo) error {
	log.Println("== Generating initramfs ==")
	requiredFiles := misc.StringSet{
		"/bin/busybox":        false,
		"/bin/sh":             false,
		"/bin/busybox-extras": false,
		"/usr/sbin/telnetd":   false,
		"/sbin/kpartx":        false,
		"/etc/deviceinfo":     false,
		"/usr/bin/unudhcpd":   false,
	}

	// Hook files & scripts
	if exists("/etc/postmarketos-mkinitfs/files") {
		log.Println("- Including hook files")
		hookFiles := getHookFiles("/etc/postmarketos-mkinitfs/files")
		if err := getFiles(files, hookFiles, true); err != nil {
			return err
		}
	}
	log.Println("- Including hook scripts")
	getHookScripts(files)

	log.Println("- Including required binaries")
	if err := getFiles(files, requiredFiles, true); err != nil {
		return err
	}

	return nil
}

func getInitfsModules(files misc.StringSet, devinfo deviceinfo.DeviceInfo, kernelVer string) error {
	log.Println("- Including kernel modules")

	modDir := filepath.Join("/lib/modules", kernelVer)
	if !exists(modDir) {
		// dir /lib/modules/<kernel> if kernel built without module support, so just print a message
		log.Printf("-- kernel module directory not found: %q, not including modules", modDir)
		return nil
	}

	// modules.* required by modprobe
	modprobeFiles, _ := filepath.Glob(filepath.Join(modDir, "modules.*"))
	for _, file := range modprobeFiles {
		files[file] = false
	}

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
				if err := getModulesInDir(files, d); err != nil {
					log.Print("Unable to get modules in dir: ", d)
					return err
				}
			}
		} else if dir == "" {
			// item is a module name
			if err := getModule(files, file, modDir); err != nil {
				log.Print("Unable to get module: ", file)
				return err
			}
		} else {
			log.Printf("Unknown module entry: %q", item)
		}
	}

	// deviceinfo modules
	for _, module := range strings.Fields(devinfo.ModulesInitfs) {
		if err := getModule(files, module, modDir); err != nil {
			log.Print("Unable to get modules from deviceinfo")
			return err
		}
	}

	// /etc/postmarketos-mkinitfs/modules/*.modules
	initfsModFiles, _ := filepath.Glob("/etc/postmarketos-mkinitfs/modules/*.modules")
	for _, modFile := range initfsModFiles {
		f, err := os.Open(modFile)
		if err != nil {
			log.Print("getInitfsModules: unable to open mkinitfs modules file: ", modFile)
			return err
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			if err := getModule(files, s.Text(), modDir); err != nil {
				log.Print("getInitfsModules: unable to get module file: ", s.Text())
				return err
			}
		}
	}

	return nil
}

func getKernelReleaseFile() (string, error) {
	files, _ := filepath.Glob("/usr/share/kernel/*/kernel.release")
	// only one kernel flavor supported
	if len(files) != 1 {
		return "", fmt.Errorf("only one kernel release/flavor is supported, found: %q", files)
	}

	return files[0], nil
}

func getKernelVersion() (string, error) {
	var version string

	releaseFile, err := getKernelReleaseFile()
	if err != nil {
		return version, err
	}

	contents, err := os.ReadFile(releaseFile)
	if err != nil {
		return version, err
	}

	return strings.TrimSpace(string(contents)), nil
}

func Copy(srcFile, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}

	defer out.Close()

	in, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}

func copyUbootFiles(path string, devinfo deviceinfo.DeviceInfo) error {
	if devinfo.UbootBoardname == "" {
		return nil
	}

	srcDir := filepath.Join("/usr/share/u-boot", devinfo.UbootBoardname)
	entries, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(path, entry.Name())

		if err := Copy(sourcePath, destPath); err != nil {
			return err
		}
	}

	return nil
}

func generateInitfs(name string, path string, kernVer string, devinfo deviceinfo.DeviceInfo) error {
	initfsArchive, err := archive.New()
	if err != nil {
		return err
	}

	requiredDirs := []string{
		"/bin", "/sbin", "/usr/bin", "/usr/sbin", "/proc", "/sys",
		"/dev", "/tmp", "/lib", "/boot", "/sysroot", "/etc",
	}
	for _, dir := range requiredDirs {
		initfsArchive.Dirs[dir] = false
	}

	if err := getInitfsFiles(initfsArchive.Files, devinfo); err != nil {
		return err
	}

	if err := getInitfsModules(initfsArchive.Files, devinfo, kernVer); err != nil {
		return err
	}

	if err := initfsArchive.AddFile("/usr/share/postmarketos-mkinitfs/init.sh", "/init"); err != nil {
		return err
	}

	// splash images
	log.Println("- Including splash images")
	splashFiles, _ := filepath.Glob("/usr/share/postmarketos-splashes/*.ppm.gz")
	for _, file := range splashFiles {
		// splash images are expected at /<file>
		if err := initfsArchive.AddFile(file, filepath.Join("/", filepath.Base(file))); err != nil {
			return err
		}
	}

	// initfs_functions
	if err := initfsArchive.AddFile("/usr/share/postmarketos-mkinitfs/init_functions.sh", "/init_functions.sh"); err != nil {
		return err
	}

	log.Println("- Writing and verifying initramfs archive")
	if err := initfsArchive.Write(filepath.Join(path, name), os.FileMode(0644)); err != nil {
		return err
	}

	return nil
}

func generateInitfsExtra(name string, path string, devinfo deviceinfo.DeviceInfo) error {
	initfsExtraArchive, err := archive.New()
	if err != nil {
		return err
	}

	if err := getInitfsExtraFiles(initfsExtraArchive.Files, devinfo); err != nil {
		return err
	}

	log.Println("- Writing and verifying initramfs-extra archive")
	if err := initfsExtraArchive.Write(filepath.Join(path, name), os.FileMode(0644)); err != nil {
		return err
	}

	return nil
}

func stripExts(file string) string {
	return strings.Split(file, ".")[0]
}

func getModulesInDir(files misc.StringSet, modPath string) error {
	err := filepath.Walk(modPath, func(path string, f os.FileInfo, err error) error {
		// TODO: need to support more extensions?
		if filepath.Ext(path) != ".ko" && filepath.Ext(path) != ".xz" {
			return nil
		}
		files[path] = false
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// Given a module name, e.g. 'dwc_wdt', resolve the full path to the module
// file and all of its dependencies.
// Note: it's not necessarily fatal if the module is not found, since it may
// have been built into the kernel
// TODO: look for it in modules.builtin, and make it fatal if it can't be found
// anywhere
func getModule(files misc.StringSet, modName string, modDir string) error {

	modDep := filepath.Join(modDir, "modules.dep")
	if !exists(modDep) {
		log.Fatal("Kernel module.dep not found: ", modDir)
	}

	fd, err := os.Open(modDep)
	if err != nil {
		log.Print("Unable to open modules.dep: ", modDep)
		return err
	}
	defer fd.Close()

	deps, err := getModuleDeps(modName, fd)
	if err != nil {
		return err
	}

	for _, dep := range deps {
		p := filepath.Join(modDir, dep)
		if !exists(p) {
			log.Print(fmt.Sprintf("Tried to include a module that doesn't exist in the modules directory (%s): %s", modDir, p))
			return err
		}
		files[p] = false
	}

	return err
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
