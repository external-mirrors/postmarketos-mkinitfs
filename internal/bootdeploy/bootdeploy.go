package bootdeploy

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
)

type BootDeploy struct {
	inDir          string
	outDir         string
	ubootBoardname string
}

// New returns a new BootDeploy, which then runs:
//
//	boot-deploy -d indir -o outDir
//
// ubootBoardname is used for copying in some u-boot files prior to running
// boot-deploy. This is optional, passing an empty string is ok if this is not
// needed.
func New(inDir, outDir, ubootBoardname string) *BootDeploy {
	return &BootDeploy{
		inDir:          inDir,
		outDir:         outDir,
		ubootBoardname: ubootBoardname,
	}
}

func (b *BootDeploy) Run() error {

	if err := copyUbootFiles(b.inDir, b.ubootBoardname); errors.Is(err, os.ErrNotExist) {
		log.Println("u-boot files copying skipped: ", err)
	} else {
		if err != nil {
			log.Fatal("copyUbootFiles: ", err)
		}
	}

	return bootDeploy(b.inDir, b.outDir)
}

func bootDeploy(workDir string, outDir string) error {
	// boot-deploy expects the kernel to be in the same dir as initramfs.
	// Assume that the kernel is in the output dir...
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

func copy(srcFile, dstFile string) error {
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
