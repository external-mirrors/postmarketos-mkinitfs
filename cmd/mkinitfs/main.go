// Copyright 2022 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
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
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/hookdirs"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/hookfiles"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/hookscripts"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/initramfs"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/modules"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/osksdl"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/pkgs/deviceinfo"
)

// set at build time
var Version string

func main() {

	outDir := flag.String("d", "/boot", "Directory to output initfs(-extra) and other boot files")
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Print version and quit.")
	flag.Parse()

	if showVersion {
		fmt.Printf("%s - %s\n", filepath.Base(os.Args[0]), Version)
		os.Exit(0)
	}

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
	defer func() {
		e := os.RemoveAll(workDir)
		if e != nil && err == nil {
			err = e
		}
		if err != nil {
			log.Fatal(err)
		}
	}()

	log.Print("Generating for kernel version: ", kernVer)
	log.Print("Output directory: ", *outDir)

	log.Println("== Generating initramfs ==")
	if err := generateArchive("initramfs", workDir, []filelist.FileLister{
		hookdirs.New("/usr/share/mkinitfs/dirs"),
		hookdirs.New("/etc/mkinitfs/dirs"),
		hookfiles.New("/usr/share/mkinitfs/files"),
		hookfiles.New("/etc/mkinitfs/files"),
		hookscripts.New("/usr/share/mkinitfs/hooks"),
		hookscripts.New("/etc/mkinitfs/hooks"),
		modules.New(strings.Fields(devinfo.ModulesInitfs), "/usr/share/mkinitfs/modules"),
	}); err != nil {
		log.Fatalf("failed to generate %q: %s\n", "initramfs", err)
	}

	log.Println("== Generating initramfs-extra ==")
	if err := generateArchive("initramfs-extra", workDir, []filelist.FileLister{
		hookfiles.New("/usr/share/mkinitfs/files-extra"),
		hookfiles.New("/etc/mkinitfs/files-extra"),
		hookscripts.New("/usr/share/mkinitfs/hooks-extra"),
		hookscripts.New("/etc/mkinitfs/hooks-extra"),
		osksdl.New(devinfo.MesaDriver),
	}); err != nil {
		log.Fatalf("failed to generate %q: %s\n", "initramfs-extra", err)
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

func generateArchive(name string, path string, features []filelist.FileLister) error {
	a, err := archive.New()
	if err != nil {
		return err
	}

	fs := initramfs.New(features)
	if err := a.AddItems(fs); err != nil {
		return err
	}

	log.Println("- Writing and verifying archive: ", name)
	if err := a.Write(filepath.Join(path, name), os.FileMode(0644)); err != nil {
		return err
	}

	return nil
}
