// Copyright 2022 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/archive"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/hookfiles"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/hookscripts"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/modules"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/osksdl"
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

	kernVer, err := misc.GetKernelVersion()
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

func getInitfsExtraFiles(devinfo deviceinfo.DeviceInfo) (files []string, err error) {
	log.Println("== Generating initramfs extra ==")

	// Hook files & scripts
	if misc.Exists("/etc/postmarketos-mkinitfs/files-extra") {
		log.Println("- Including hook files")
		hookFiles := hookfiles.New("/etc/postmarketos-mkinitfs/files-extra")

		if list, err := hookFiles.List(); err != nil {
			return nil, err
		} else {
			files = append(files, list...)
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

	// Hook files & scripts
	if misc.Exists("/etc/postmarketos-mkinitfs/files") {
		log.Println("- Including hook files")
		hookFiles := hookfiles.New("/etc/postmarketos-mkinitfs/files")

		if list, err := hookFiles.List(); err != nil {
			return nil, err
		} else {
			files = append(files, list...)
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

	return
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
