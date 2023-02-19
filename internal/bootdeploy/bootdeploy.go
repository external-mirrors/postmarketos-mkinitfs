package bootdeploy

import ()

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

	return nil
}
