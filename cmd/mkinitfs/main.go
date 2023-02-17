// Copyright 2022 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"debug/elf"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/archive"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/osksdl"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/hookscripts"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/modules"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/pkgs/deviceinfo"
)

func main() {
	deviceinfoFile := "/etc/deviceinfo"
	if !misc.Exists(deviceinfoFile) {
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

	defer misc.TimeFunc(time.Now(), "mkinitfs")

	kernVer, err := getKernelVersion()
	if err != nil {
		log.Fatal(err)
	}

	// temporary working dir
	workDir, err := os.MkdirTemp("", "mkinitfs")
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
	if !misc.Exists(cmd.Path) {
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

func getHookFiles(filesdir string) (files []string, err error) {
	fileInfo, err := os.ReadDir(filesdir)
	if err != nil {
		return nil, fmt.Errorf("getHookFiles: unable to read hook file dir: %w", err)
	}
	for _, file := range fileInfo {
		path := filepath.Join(filesdir, file.Name())
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("getHookFiles: unable to open hook file: %w", err)

		}
		defer f.Close()
		log.Printf("-- Including files from: %s\n", path)
		s := bufio.NewScanner(f)
		for s.Scan() {
			if filelist, err := getFiles([]string{s.Text()}, true); err != nil {
				return nil, fmt.Errorf("getHookFiles: unable to add file %q required by %q: %w", s.Text(), path, err)
			} else {
				files = append(files, filelist...)
			}
		}
		if err := s.Err(); err != nil {
			return nil, fmt.Errorf("getHookFiles: uname to process hook file %q: %w", path, err)
		}
	}
	return files, nil
}

func getDeps(file string, parents map[string]struct{}) (files []string, err error) {

	if _, found := parents[file]; found {
		return
	}

	// get dependencies for binaries
	fd, err := elf.Open(file)
	if err != nil {
		return nil, fmt.Errorf("getDeps: unable to open elf binary %q: %w", file, err)
	}
	libs, _ := fd.ImportedLibraries()
	fd.Close()
	files = append(files, file)
	parents[file] = struct{}{}

	if len(libs) == 0 {
		return
	}

	// we don't recursively search these paths for performance reasons
	libdirGlobs := []string{
		"/usr/lib",
		"/lib",
		"/usr/lib/expect*",
	}

	for _, lib := range libs {
		found := false
	findDepLoop:
		for _, libdirGlob := range libdirGlobs {
			libdirs, _ := filepath.Glob(libdirGlob)
			for _, libdir := range libdirs {
				path := filepath.Join(libdir, lib)
				if _, err := os.Stat(path); err == nil {
					binaryDepFiles, err := getDeps(path, parents)
					if err != nil {
						return nil, err
					}
					files = append(files, binaryDepFiles...)
					files = append(files, path)
					found = true
					break findDepLoop
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("getDeps: unable to locate dependency for %q: %s", file, lib)
		}
	}

	return
}

// Recursively list all dependencies for a given ELF binary
func getBinaryDeps(file string) ([]string, error) {
	// if file is a symlink, resolve dependencies for target
	fileStat, err := os.Lstat(file)
	if err != nil {
		return nil, fmt.Errorf("getBinaryDeps: failed to stat file %q: %w", file, err)
	}

	// Symlink: write symlink to archive then set 'file' to link target
	if fileStat.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(file)
		if err != nil {
			return nil, fmt.Errorf("getBinaryDeps: unable to read symlink %q: %w", file, err)
		}
		if !filepath.IsAbs(target) {
			target, err = misc.RelativeSymlinkTargetToDir(target, filepath.Dir(file))
			if err != nil {
				return nil, err
			}
		}
		file = target
	}

	return getDeps(file, make(map[string]struct{}))

}

func getFiles(list []string, required bool) (files []string, err error) {
	for _, file := range list {
		filelist, err := getFile(file, required)
		if err != nil {
			return nil, err
		}
		files = append(files, filelist...)
	}

	files = misc.RemoveDuplicates(files)
	return
}

func getFile(file string, required bool) (files []string, err error) {
	// Expand glob expression
	expanded, err := filepath.Glob(file)
	if err != nil {
		return
	}
	if len(expanded) > 0 && expanded[0] != file {
		for _, path := range expanded {
			if globFiles, err := getFile(path, required); err != nil {
				return files, err
			} else {
				files = append(files, globFiles...)
			}
		}
		return misc.RemoveDuplicates(files), nil
	}

	fileInfo, err := os.Stat(file)
	if err != nil {
		if required {
			return files, fmt.Errorf("getFile: failed to stat file %q: %w", file, err)
		}
		return files, nil
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
			newFiles, err := getFile(path, required)
			if err != nil {
				return err
			}
			files = append(files, newFiles...)
			return nil
		})
		if err != nil {
			return files, err
		}
	} else {
		files = append(files, file)

		// get dependencies for binaries
		if _, err := elf.Open(file); err == nil {
			if binaryDepFiles, err := getBinaryDeps(file); err != nil {
				return files, err
			} else {
				files = append(files, binaryDepFiles...)
			}
		}
	}

	files = misc.RemoveDuplicates(files)
	return
}

func getHookScripts(scriptsdir string) (files []string, err error) {
	fileInfo, err := os.ReadDir(scriptsdir)
	if err != nil {
		return nil, fmt.Errorf("getHookScripts: unable to read hook script dir: %w", err)
	}
	for _, file := range fileInfo {
		path := filepath.Join(scriptsdir, file.Name())
		files = append(files, path)
	}
	return files, nil
}

func getInitfsExtraFiles(devinfo deviceinfo.DeviceInfo) (files []string, err error) {
	log.Println("== Generating initramfs extra ==")
	binariesExtra := []string{
		"/lib/libz.so.1",
		"/sbin/btrfs",
		"/sbin/dmsetup",
		"/sbin/e2fsck",
		"/usr/sbin/parted",
		"/usr/sbin/resize2fs",
		"/usr/sbin/resize.f2fs",
	}
	log.Println("- Including extra binaries")
	if filelist, err := getFiles(binariesExtra, true); err != nil {
		return nil, err
	} else {
		files = append(files, filelist...)
	}

	// Hook files & scripts
	if misc.Exists("/etc/postmarketos-mkinitfs/files-extra") {
		log.Println("- Including hook files")
		var hookFiles []string
		hookFiles, err := getHookFiles("/etc/postmarketos-mkinitfs/files-extra")
		if err != nil {
			return nil, err
		}
		if filelist, err := getFiles(hookFiles, true); err != nil {
			return nil, err
		} else {
			files = append(files, filelist...)
		}
	}

	if misc.Exists("/etc/postmarketos-mkinitfs/hooks-extra") {
		log.Println("- Including extra hook scripts")
		hookScripts := hookscripts.New("/etc/postmarketos-mkinitfs/hooks-extra")

		if list, err := hookScripts.List(); err != nil {
			return nil, err
		} else {
			files = append(files, list...)
		}
	}

	osksdlFiles := osksdl.New(devinfo.MesaDriver)
	if filelist, err := osksdlFiles.List(); err != nil {
		return nil, err
	} else if len(filelist) > 0 {
		log.Println("- Including osk-sdl support")
		files = append(files, filelist...)
	}

	return
}

func getInitfsFiles(devinfo deviceinfo.DeviceInfo) (files []string, err error) {
	log.Println("== Generating initramfs ==")
	requiredFiles := []string{
		"/bin/busybox",
		"/bin/sh",
		"/bin/busybox-extras",
		"/usr/sbin/telnetd",
		"/usr/sbin/kpartx",
		"/etc/deviceinfo",
		"/usr/bin/unudhcpd",
	}

	// Hook files & scripts
	if misc.Exists("/etc/postmarketos-mkinitfs/files") {
		log.Println("- Including hook files")
		if hookFiles, err := getHookFiles("/etc/postmarketos-mkinitfs/files"); err != nil {
			return nil, err
		} else {
			if filelist, err := getFiles(hookFiles, true); err != nil {
				return nil, err
			} else {
				files = append(files, filelist...)
			}
		}
	}

	if misc.Exists("/etc/postmarketos-mkinitfs/hooks") {
		log.Println("- Including hook scripts")
		hookScripts := hookscripts.New("/etc/postmarketos-mkinitfs/hooks")

		if list, err := hookScripts.List(); err != nil {
			return nil, err
		} else {
			files = append(files, list...)
		}
	}

	log.Println("- Including required binaries")
	if filelist, err := getFiles(requiredFiles, true); err != nil {
		return nil, err
	} else {
		files = append(files, filelist...)
	}

	return
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
	entries, err := os.ReadDir(srcDir)
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
		if err := initfsArchive.AddItem(dir, dir); err != nil {
			return err
		}
	}

	if files, err := getInitfsFiles(devinfo); err != nil {
		return err
	} else {
		items := make(map[string]string)
		// copy files into a map, where the source(key) and dest(value) are the
		// same
		for _, f := range files {
			items[f] = f
		}
		if err := initfsArchive.AddItems(items); err != nil {
			return err
		}
	}

	log.Println("- Including kernel modules")
	modules := modules.New(strings.Fields(devinfo.ModulesInitfs))
	if list, err := modules.List(); err != nil {
		return err
	} else {
		items := make(map[string]string)
		// copy files into a map, where the source(key) and dest(value) are the
		// same
		for _, f := range list {
			items[f] = f
		}
		if err := initfsArchive.AddItems(items); err != nil {
			return err
		}
	}

	if err := initfsArchive.AddItem("/usr/share/postmarketos-mkinitfs/init.sh", "/init"); err != nil {
		return err
	}

	// splash images
	log.Println("- Including splash images")
	splashFiles, _ := filepath.Glob("/usr/share/postmarketos-splashes/*.ppm.gz")
	for _, file := range splashFiles {
		// splash images are expected at /<file>
		if err := initfsArchive.AddItem(file, filepath.Join("/", filepath.Base(file))); err != nil {
			return err
		}
	}

	// initfs_functions
	if err := initfsArchive.AddItem("/usr/share/postmarketos-mkinitfs/init_functions.sh", "/init_functions.sh"); err != nil {
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

	if files, err := getInitfsExtraFiles(devinfo); err != nil {
		return err
	} else {

		items := make(map[string]string)
		// copy files into a map, where the source(key) and dest(value) are the
		// same
		for _, f := range files {
			items[f] = f
		}
		if err := initfsExtraArchive.AddItems(items); err != nil {
			return err
		}
	}

	log.Println("- Writing and verifying initramfs-extra archive")
	if err := initfsExtraArchive.Write(filepath.Join(path, name), os.FileMode(0644)); err != nil {
		return err
	}

	return nil
}
