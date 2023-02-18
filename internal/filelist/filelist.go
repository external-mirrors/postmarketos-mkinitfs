package filelist

import "sync"

type FileLister interface {
	List() (*FileList, error)
}

type File struct {
	Source string
	Dest   string
}

type FileList struct {
	m map[string]string
	sync.RWMutex
}

func NewFileList() *FileList {
	return &FileList{
		m: make(map[string]string),
	}
}

func (f *FileList) Add(src string, dest string) {
	f.Lock()
	defer f.Unlock()

	f.m[src] = dest
}

func (f *FileList) Get(src string) (string, bool) {
	f.RLock()
	defer f.RUnlock()

	dest, found := f.m[src]
	return dest, found
}

// Import copies in the contents of src. If a source path already exists when
// importing, then the destination path is updated with the new value.
func (f *FileList) Import(src *FileList) {
	for i := range src.IterItems() {
		f.Add(i.Source, i.Dest)
	}
}

// iterate through the list and and send each one as a new File over the
// returned channel
func (f *FileList) IterItems() <-chan File {
	ch := make(chan File)
	go func() {
		f.RLock()
		defer f.RUnlock()

		for src, dest := range f.m {
			ch <- File{
				Source: src,
				Dest:   dest,
			}
		}
		close(ch)
	}()
	return ch
}
