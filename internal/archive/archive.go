// Copyright 2021 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package archive

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/cavaliercoder/go-cpio"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/osutil"
)

type Archive struct {
	items      archiveItems
	cpioWriter *cpio.Writer
	buf        *bytes.Buffer
}

func New() (*Archive, error) {
	buf := new(bytes.Buffer)
	archive := &Archive{
		cpioWriter: cpio.NewWriter(buf),
		buf:        buf,
	}

	return archive, nil
}

type archiveItem struct {
	sourcePath string
	header     *cpio.Header
}

type archiveItems struct {
	items []archiveItem
	sync.RWMutex
}

// Adds the given item to the archiveItems, only if it doesn't already exist in
// the list. The items are kept sorted in ascending order.
func (a *archiveItems) add(item archiveItem) {
	a.Lock()
	defer a.Unlock()

	if len(a.items) < 1 {
		// empty list
		a.items = append(a.items, item)
		return
	}

	// find existing item, or index of where new item should go
	i := sort.Search(len(a.items), func(i int) bool {
		return strings.Compare(item.header.Name, a.items[i].header.Name) <= 0
	})

	if i >= len(a.items) {
		// doesn't exist in list, but would be at the very end
		a.items = append(a.items, item)
		return
	}

	if strings.Compare(a.items[i].header.Name, item.header.Name) == 0 {
		// already in list
		return
	}

	// grow list by 1, shift right at index, and insert new string at index
	a.items = append(a.items, archiveItem{})
	copy(a.items[i+1:], a.items[i:])
	a.items[i] = item
}

// iterate through items and send each one over the returned channel
func (a *archiveItems) IterItems() <-chan archiveItem {
	ch := make(chan archiveItem)
	go func() {
		a.RLock()
		defer a.RUnlock()

		for _, item := range a.items {
			ch <- item
		}
		close(ch)
	}()
	return ch
}

func (archive *Archive) Write(path string, mode os.FileMode) error {
	if err := archive.writeCpio(); err != nil {
		return err
	}

	if err := archive.cpioWriter.Close(); err != nil {
		return fmt.Errorf("archive.Write: error closing archive: %w", err)
	}

	// Write archive to path
	if err := archive.writeCompressed(path, mode); err != nil {
		return fmt.Errorf("unable to write archive to location %q: %w", path, err)
	}

	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("unable to chmod %q to %s: %w", path, mode, err)
	}

	return nil
}

// Adds the given items in the map to the archive. The map format is {source path:dest path}.
// Internally this just calls AddItem on each key,value pair in the map.
func (archive *Archive) AddItems(f filelist.FileLister) error {
	list, err := f.List()
	if err != nil {
		return err
	}
	for i := range list.IterItems() {
		if err := archive.AddItem(i.Source, i.Dest); err != nil {
			return err
		}
	}
	return nil
}

// Adds the given file or directory at "source" to the archive at "dest"
func (archive *Archive) AddItem(source string, dest string) error {

	sourceStat, err := os.Lstat(source)
	if err != nil {
		e, ok := err.(*os.PathError)
		if e.Err == syscall.ENOENT && ok {
			// doesn't exist in current filesystem, assume it's a new directory
			return archive.addDir(dest)
		}
		return fmt.Errorf("AddItem: failed to get stat for %q: %w", source, err)
	}

	if sourceStat.Mode()&os.ModeDir != 0 {
		return archive.addDir(dest)
	}

	return archive.addFile(source, dest)
}

func (archive *Archive) addFile(source string, dest string) error {
	if err := archive.addDir(filepath.Dir(dest)); err != nil {
		return err
	}

	sourceStat, err := os.Lstat(source)
	if err != nil {
		log.Print("addFile: failed to stat file: ", source)
		return err
	}

	// Symlink: write symlink to archive then set 'file' to link target
	if sourceStat.Mode()&os.ModeSymlink != 0 {
		// log.Printf("File %q is a symlink", file)
		target, err := os.Readlink(source)
		if err != nil {
			log.Print("addFile: failed to get symlink target: ", source)
			return err
		}

		destFilename := strings.TrimPrefix(dest, "/")

		archive.items.add(archiveItem{
			sourcePath: source,
			header: &cpio.Header{
				Name:     destFilename,
				Linkname: target,
				Mode:     0644 | cpio.ModeSymlink,
				Size:     int64(len(target)),
				// Checksum: 1,
			},
		})

		if filepath.Dir(target) == "." {
			target = filepath.Join(filepath.Dir(source), target)
		}
		// make sure target is an absolute path
		if !filepath.IsAbs(target) {
			target, err = osutil.RelativeSymlinkTargetToDir(target, filepath.Dir(source))
			if err != nil {
				return err
			}
		}
		err = archive.addFile(target, target)
		return err
	}

	destFilename := strings.TrimPrefix(dest, "/")

	archive.items.add(archiveItem{
		sourcePath: source,
		header: &cpio.Header{
			Name: destFilename,
			Mode: cpio.FileMode(sourceStat.Mode().Perm()),
			Size: sourceStat.Size(),
			// Checksum: 1,
		},
	})

	return nil
}

func (archive *Archive) writeCompressed(path string, mode os.FileMode) error {
	// TODO: support other compression formats, based on deviceinfo
	fd, err := os.Create(path)
	if err != nil {
		return err
	}

	gz := gzip.NewWriter(fd)

	if _, err = io.Copy(gz, archive.buf); err != nil {
		return err
	}

	if err := gz.Close(); err != nil {
		return err
	}

	// call fsync just to be sure
	if err := fd.Sync(); err != nil {
		return err
	}

	if err := os.Chmod(path, mode); err != nil {
		return err
	}

	return nil
}

func (archive *Archive) writeCpio() error {
	// having a transient function for actually adding files to the archive
	// allows the deferred fd.close to run after every copy and prevent having
	// tons of open file handles until the copying is all done
	copyToArchive := func(source string, header *cpio.Header) error {

		if err := archive.cpioWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("archive.writeCpio: unable to write header: %w", err)
		}

		// don't copy actual dirs into the archive, writing the header is enough
		if !header.Mode.IsDir() {
			if header.Mode.IsRegular() {
				fd, err := os.Open(source)
				if err != nil {
					return fmt.Errorf("archive.writeCpio: uname to open file %q, %w", source, err)
				}
				defer fd.Close()
				if _, err := io.Copy(archive.cpioWriter, fd); err != nil {
					return fmt.Errorf("archive.writeCpio: unable to write out archive: %w", err)
				}
			} else if header.Linkname != "" {
				// the contents of a symlink is just need the link name
				if _, err := archive.cpioWriter.Write([]byte(header.Linkname)); err != nil {
					return fmt.Errorf("archive.writeCpio: unable to write out symlink: %w", err)
				}
			} else {
				return fmt.Errorf("archive.writeCpio: unknown type for file: %s", source)
			}
		}

		return nil
	}

	for i := range archive.items.IterItems() {
		if err := copyToArchive(i.sourcePath, i.header); err != nil {
			return err
		}
	}
	return nil
}

func (archive *Archive) addDir(dir string) error {
	if dir == "/" {
		dir = "."
	}

	subdirs := strings.Split(strings.TrimPrefix(dir, "/"), "/")
	for i, subdir := range subdirs {
		path := filepath.Join(strings.Join(subdirs[:i], "/"), subdir)
		archive.items.add(archiveItem{
			sourcePath: path,
			header: &cpio.Header{
				Name: path,
				Mode: cpio.ModeDir | 0755,
			},
		})
	}

	return nil
}
