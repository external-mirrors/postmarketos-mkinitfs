package bootdeploy

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/pkgs/deviceinfo"
)

type BootDeploy struct {
	inDir   string
	outDir  string
	devinfo deviceinfo.DeviceInfo
}

// New returns a new BootDeploy, which then runs:
//
//	boot-deploy -d indir -o outDir
//
// devinfo is used to access some deviceinfo values, such as UbootBoardname
func New(inDir string, outDir string, devinfo deviceinfo.DeviceInfo) *BootDeploy {
	return &BootDeploy{
		inDir:   inDir,
		outDir:  outDir,
		devinfo: devinfo,
	}
}

func (b *BootDeploy) Run() error {
	if err := copyUbootFiles(b.inDir, b.devinfo.UbootBoardname); errors.Is(err, os.ErrNotExist) {
		log.Println("u-boot files copying skipped: ", err)
	} else {
		if err != nil {
			log.Fatal("copyUbootFiles: ", err)
		}
	}

	// boot-deploy -i initramfs -k vmlinuz-postmarketos-rockchip -d /tmp/cpio -o /tmp/foo initramfs-extra
	args := []string{
		"-i", "initramfs",
		"-d", b.inDir,
		"-o", b.outDir,
	}

	if b.devinfo.CreateInitfsExtra {
		args = append(args, "initramfs-extra")
	}
	cmd := exec.Command("boot-deploy", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// Copy copies the file at srcFile path to a new file at dstFile path
func copy(srcFile, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}

	defer func() {
		errClose := out.Close()
		if err == nil {
			err = errClose
		}
	}()

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

// copyUbootFiles uses deviceinfo_uboot_boardname to copy u-boot files required
// for running boot-deploy
func copyUbootFiles(path, ubootBoardname string) error {
	if ubootBoardname == "" {
		return nil
	}

	srcDir := filepath.Join("/usr/share/u-boot", ubootBoardname)
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(path, entry.Name())

		if err := copy(sourcePath, destPath); err != nil {
			return err
		}
	}

	return nil
}
