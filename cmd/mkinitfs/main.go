// Copyright 2022 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
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
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/osutil"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/pkgs/deviceinfo"
)

// set at build time
var Version string
var DisableGC string

func main() {
	// To allow working around silly GC-related issues, like https://gitlab.com/qemu-project/qemu/-/issues/2560
	if strings.ToLower(DisableGC) == "true" {
		debug.SetGCPercent(-1)
	}

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

	log.Default().SetFlags(log.Lmicroseconds)

	var devinfo deviceinfo.DeviceInfo
	deverr_usr := devinfo.ReadDeviceinfo("/usr/share/deviceinfo/deviceinfo")
	deverr_etc := devinfo.ReadDeviceinfo("/etc/deviceinfo")
	if deverr_etc != nil && deverr_usr != nil {
		log.Println("Error reading deviceinfo")
		log.Println("\t/usr/share/deviceinfo/deviceinfo:", deverr_usr)
		log.Println("\t/etc/deviceinfo:", deverr_etc)
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
			log.Println(e)
			log.Println("unable to remove temporary work directory")
		}
	}()

	log.Print("Generating for kernel version: ", kernVer)
	log.Print("Output directory: ", *outDir)

	//
	// initramfs
	//
	// deviceinfo.InitfsCompression needs a little more post-processing
	compressionFormat, compressionLevel := archive.ExtractFormatLevel(devinfo.InitfsCompression)
	log.Printf("== Generating %s ==\n", "initramfs")
	log.Printf("- Using compression format %s with level %q\n", compressionFormat, compressionLevel)

	start := time.Now()
	initramfsAr := archive.New(compressionFormat, compressionLevel)
	initfs := initramfs.New([]filelist.FileLister{
		hookdirs.New("/usr/share/mkinitfs/dirs"),
		hookdirs.New("/etc/mkinitfs/dirs"),
		hookfiles.New("/usr/share/mkinitfs/files"),
		hookfiles.New("/etc/mkinitfs/files"),
		hookscripts.New("/usr/share/mkinitfs/hooks", "/hooks"),
		hookscripts.New("/etc/mkinitfs/hooks", "/hooks"),
		hookscripts.New("/usr/share/mkinitfs/hooks-cleanup", "/hooks-cleanup"),
		hookscripts.New("/etc/mkinitfs/hooks-cleanup", "/hooks-cleanup"),
		modules.New("/usr/share/mkinitfs/modules"),
		modules.New("/etc/mkinitfs/modules"),
	})
	initfsExtra := initramfs.New([]filelist.FileLister{
		hookfiles.New("/usr/share/mkinitfs/files-extra"),
		hookfiles.New("/etc/mkinitfs/files-extra"),
		hookscripts.New("/usr/share/mkinitfs/hooks-extra", "/hooks-extra"),
		hookscripts.New("/etc/mkinitfs/hooks-extra", "/hooks-extra"),
		modules.New("/usr/share/mkinitfs/modules-extra"),
		modules.New("/etc/mkinitfs/modules-extra"),
	})

	if err := initramfsAr.AddItems(initfs); err != nil {
		log.Println(err)
		log.Println("failed to generate: ", "initramfs")
		retCode = 1
		return
	}

	// Include initramfs-extra files in the initramfs if not making a separate
	// archive
	if !devinfo.CreateInitfsExtra {
		if err := initramfsAr.AddItems(initfsExtra); err != nil {
			log.Println(err)
			log.Println("failed to generate: ", "initramfs")
			retCode = 1
			return
		}
	}

	if err := initramfsAr.Write(filepath.Join(workDir, "initramfs"), os.FileMode(0644)); err != nil {
		log.Println(err)
		log.Println("failed to generate: ", "initramfs")
		retCode = 1
		return
	}
	misc.TimeFunc(start, "initramfs")

	if devinfo.CreateInitfsExtra {
		//
		// initramfs-extra
		//
		// deviceinfo.InitfsExtraCompression needs a little more post-processing
		compressionFormat, compressionLevel = archive.ExtractFormatLevel(devinfo.InitfsExtraCompression)
		log.Printf("== Generating %s ==\n", "initramfs-extra")
		log.Printf("- Using compression format %s with level %q\n", compressionFormat, compressionLevel)

		start = time.Now()
		initramfsExtraAr := archive.New(compressionFormat, compressionLevel)
		if err := initramfsExtraAr.AddItemsExclude(initfsExtra, initfs); err != nil {
			log.Println(err)
			log.Println("failed to generate: ", "initramfs-extra")
			retCode = 1
			return
		}
		if err := initramfsExtraAr.Write(filepath.Join(workDir, "initramfs-extra"), os.FileMode(0644)); err != nil {
			log.Println(err)
			log.Println("failed to generate: ", "initramfs-extra")
			retCode = 1
			return
		}
		misc.TimeFunc(start, "initramfs-extra")
	}

	// Final processing of initramfs / kernel is done by boot-deploy
	if !disableBootDeploy {
		if err := bootDeploy(workDir, *outDir, devinfo); err != nil {
			log.Println(err)
			log.Println("boot-deploy failed")
			retCode = 1
			return
		}
	}
}

func bootDeploy(workDir string, outDir string, devinfo deviceinfo.DeviceInfo) error {
	log.Print("== Using boot-deploy to finalize/install files ==")
	defer misc.TimeFunc(time.Now(), "boot-deploy")

	bd := bootdeploy.New(workDir, outDir, devinfo)
	return bd.Run()
}
