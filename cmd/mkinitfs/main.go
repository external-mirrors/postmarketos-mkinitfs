// Copyright 2022 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/archive"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/bootdeploy"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/hookdirs"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/hookfiles"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/hookscripts"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/initramfs"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/modules"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist/osksdl"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/osutil"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/pkgs/deviceinfo"
)

// set at build time
var Version string

func main() {
	retCode := 0
	defer func() { os.Exit(retCode) }()

	outDir := flag.String("d", "/boot", "Directory to output initfs(-extra) and other boot files")

	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Print version and quit.")

	var disableBootDeploy bool
	flag.BoolVar(&disableBootDeploy, "no-bootdeploy", false, "Disable running 'boot-deploy' after generating archives.")
	flag.Parse()

	if showVersion {
		fmt.Printf("%s - %s\n", filepath.Base(os.Args[0]), Version)
		return
	}

	deviceinfoFile := "/etc/deviceinfo"
	if exists, err := misc.Exists(deviceinfoFile); !exists {
		log.Printf("NOTE: %q not found, this file is required by mkinitfs.\n", deviceinfoFile)
		return
	} else if err != nil {
		retCode = 1
		log.Printf("received unexpected error when getting status for %q: %s", deviceinfoFile, err)
	}

	devinfo, err := deviceinfo.ReadDeviceinfo(deviceinfoFile)
	if err != nil {
		log.Println(err)
		retCode = 1
		return
	}

	defer misc.TimeFunc(time.Now(), "mkinitfs")

	kernVer, err := osutil.GetKernelVersion()
	if err != nil {
		log.Println(err)
		retCode = 1
		return
	}

	// temporary working dir
	workDir, err := os.MkdirTemp("", "mkinitfs")
	if err != nil {
		log.Println(err)
		log.Println("unable to create temporary work directory")
		retCode = 1
		return
	}
	defer func() {
		e := os.RemoveAll(workDir)
		if e != nil && err == nil {
			err = e
			retCode = 1
		}
	}()

	log.Print("Generating for kernel version: ", kernVer)
	log.Print("Output directory: ", *outDir)

	if err := generateArchive("initramfs", workDir, []filelist.FileLister{
		hookdirs.New("/usr/share/mkinitfs/dirs"),
		hookdirs.New("/etc/mkinitfs/dirs"),
		hookfiles.New("/usr/share/mkinitfs/files"),
		hookfiles.New("/etc/mkinitfs/files"),
		hookscripts.New("/usr/share/mkinitfs/hooks"),
		hookscripts.New("/etc/mkinitfs/hooks"),
		modules.New(strings.Fields(devinfo.ModulesInitfs), "/usr/share/mkinitfs/modules"),
		modules.New([]string{}, "/etc/mkinitfs/modules"),
	}); err != nil {
		log.Println(err)
		log.Println("failed to generate: ", "initramfs")
		retCode = 1
		return
	}

	if err := generateArchive("initramfs-extra", workDir, []filelist.FileLister{
		hookfiles.New("/usr/share/mkinitfs/files-extra"),
		hookfiles.New("/etc/mkinitfs/files-extra"),
		hookscripts.New("/usr/share/mkinitfs/hooks-extra"),
		hookscripts.New("/etc/mkinitfs/hooks-extra"),
		modules.New([]string{}, "/usr/share/mkinitfs/modules-extra"),
		modules.New([]string{}, "/etc/mkinitfs/modules-extra"),
		osksdl.New(devinfo.MesaDriver),
	}); err != nil {
		log.Println(err)
		log.Println("failed to generate: ", "initramfs-extra")
		retCode = 1
		return
	}

	// Final processing of initramfs / kernel is done by boot-deploy
	if !disableBootDeploy {
		if err := bootDeploy(workDir, *outDir, devinfo.UbootBoardname); err != nil {
			log.Println(err)
			log.Println("boot-deploy failed")
			retCode = 1
			return
		}
	}
}

func bootDeploy(workDir, outDir, ubootBoardname string) error {
	log.Print("== Using boot-deploy to finalize/install files ==")
	defer misc.TimeFunc(time.Now(), "boot-deploy")

	bd := bootdeploy.New(workDir, outDir, ubootBoardname)
	return bd.Run()
}

func generateArchive(name string, path string, features []filelist.FileLister) error {
	log.Printf("== Generating %s ==\n", name)
	defer misc.TimeFunc(time.Now(), name)
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
