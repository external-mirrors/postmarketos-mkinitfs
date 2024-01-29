package bootdeploy

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

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
// and GenerateSystemdBoot
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

	kernels, err := getKernelPath(b.outDir, b.devinfo.GenerateSystemdBoot == "true")
	if err != nil {
		return err
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

	kernFilename := path.Base(kernFile)
	kernFileCopy, err := os.Create(filepath.Join(b.inDir, kernFilename))
	if err != nil {
		return err
	}

	if _, err = io.Copy(kernFileCopy, kernFd); err != nil {
		return err
	}
	if err := kernFileCopy.Close(); err != nil {
		return fmt.Errorf("error closing %s: %w", kernFilename, err)
	}

	// boot-deploy -i initramfs -k vmlinuz-postmarketos-rockchip -d /tmp/cpio -o /tmp/foo initramfs-extra
	cmd := exec.Command("boot-deploy",
		"-i", "initramfs",
		"-k", kernFilename,
		"-d", b.inDir,
		"-o", b.outDir,
		"initramfs-extra")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func getKernelPath(outDir string, zboot bool) ([]string, error) {
	var kernels []string
	if zboot {
		kernels, _ = filepath.Glob(filepath.Join(outDir, "linux.efi"))
		if len(kernels) > 0 {
			return kernels, nil
		}
		// else fallback to vmlinuz* below
	}

	kernFile := "vmlinuz*"
	kernels, _ = filepath.Glob(filepath.Join(outDir, kernFile))
	if len(kernels) == 0 {
		return nil, errors.New("Unable to find any kernels at " + filepath.Join(outDir, kernFile))
	}

	return kernels, nil
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
