package filelist

type FileLister interface {
	List() ([]string, error)
}
