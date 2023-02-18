package initramfs

import (
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist"
)

// Initramfs allows building arbitrarily complex lists of features, by slurping
// up types that implement FileLister (which includes this type! yippee) and
// combining the output from them.
type Initramfs struct {
	features []filelist.FileLister
}

// New returns a new Initramfs that generate a list of files based on the given
// list of FileListers.
func New(features []filelist.FileLister) *Initramfs {
	return &Initramfs{
		features: features,
	}
}

func (i *Initramfs) List() (*filelist.FileList, error) {
	files := filelist.NewFileList()

	for _, f := range i.features {
		list, err := f.List()
		if err != nil {
			return nil, err
		}
		files.Import(list)
	}

	return files, nil
}
