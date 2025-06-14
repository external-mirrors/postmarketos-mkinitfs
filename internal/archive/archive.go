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
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
	"github.com/ulikunitz/xz"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/filelist"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/osutil"
)

type CompressFormat string

const (
	FormatGzip CompressFormat = "gzip"
	FormatLzma CompressFormat = "lzma"
	FormatLz4  CompressFormat = "lz4"
	FormatZstd CompressFormat = "zstd"
	FormatNone CompressFormat = "none"
)

type CompressLevel string

const (
	// Mapped to the "default" level for the given format
	LevelDefault CompressLevel = "default"
	// Maps to the fastest compression level for the given format
	LevelFast CompressLevel = "fast"
	// Maps to the best compression level for the given format
	LevelBest CompressLevel = "best"
)

type Archive struct {
	cpioWriter      *cpio.Writer
	buf             *bytes.Buffer
	compress_format CompressFormat
	compress_level  CompressLevel
	items           archiveItems
}

func New(format CompressFormat, level CompressLevel) *Archive {
	buf := new(bytes.Buffer)
	archive := &Archive{
		cpioWriter:      cpio.NewWriter(buf),
		buf:             buf,
		compress_format: format,
		compress_level:  level,
	}

	return archive
}

type archiveItem struct {
	header     *cpio.Header
	sourcePath string
}

type archiveItems struct {
	items []archiveItem
	sync.RWMutex
}

// ExtractFormatLevel parses the given string in the format format[:level],
// where :level is one of CompressLevel consts. If level is omitted from the
// string, or if it can't be parsed, the level is set to the default level for
// the given format. If format is unknown, gzip is selected. This function is
// designed to always return something usable within this package.
func ExtractFormatLevel(s string) (format CompressFormat, level CompressLevel) {

	f, l, found := strings.Cut(s, ":")
	if !found {
		l = "default"
	}

	level = CompressLevel(strings.ToLower(l))
	format = CompressFormat(strings.ToLower(f))
	switch level {

	}
	switch level {
	case LevelBest:
	case LevelDefault:
	case LevelFast:
	default:
		log.Print("Unknown or no compression level set, using default")
		level = LevelDefault
	}

	switch format {
	case FormatGzip:
	case FormatLzma:
		log.Println("Format lzma doesn't support a compression level, using default settings")
		level = LevelDefault
	case FormatLz4:
	case FormatNone:
	case FormatZstd:
	default:
		log.Print("Unknown or no compression format set, using gzip")
		format = FormatGzip
	}

	return
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

// AddItems adds the given items in the map to the archive. The map format is
// {source path:dest path}. Internally this just calls AddItem on each
// key,value pair in the map.
func (archive *Archive) AddItems(flister filelist.FileLister) error {
	list, err := flister.List()
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

// AddItemsExclude is like AddItems, but takes a second FileLister that lists
// items that should not be added to the archive from the first FileLister
func (archive *Archive) AddItemsExclude(flister filelist.FileLister, exclude filelist.FileLister) error {
	list, err := flister.List()
	if err != nil {
		return err
	}

	excludeList, err := exclude.List()
	if err != nil {
		return err
	}

	for i := range list.IterItems() {
		dest, found := excludeList.Get(i.Source)

		if found {
			if i.Dest != dest {
				found = false
			}
		}

		if !found {
			if err := archive.AddItem(i.Source, i.Dest); err != nil {
				return err
			}
		}
	}

	return nil
}

// Adds the given file or directory at "source" to the archive at "dest"
func (archive *Archive) AddItem(source string, dest string) error {
	if osutil.HasMergedUsr() {
		source = osutil.MergeUsr(source)
		dest = osutil.MergeUsr(dest)
	}
	sourceStat, err := os.Lstat(source)
	if err != nil {
		e, ok := err.(*os.PathError)
		if e.Err == syscall.ENOENT && ok {
			// doesn't exist in current filesystem, assume it's a new directory
			return archive.addDir(dest)
		}
		return fmt.Errorf("AddItem: failed to get stat for %q: %w", source, err)
	}

	// A symlink to a directory doesn't have the os.ModeDir bit set, so we need
	// to check if it's a symlink first
	if sourceStat.Mode()&os.ModeSymlink != 0 {
		return archive.addSymlink(source, dest)
	}

	if sourceStat.Mode()&os.ModeDir != 0 {
		return archive.addDir(dest)
	}

	return archive.addFile(source, dest)
}

func (archive *Archive) addSymlink(source string, dest string) error {
	target, err := os.Readlink(source)
	if err != nil {
		log.Print("addSymlink: failed to get symlink target for: ", source)
		return err
	}

	// Make sure we pick up the symlink target too
	targetAbs := target
	if filepath.Dir(target) == "." {
		// relative symlink, make it absolute so we can add the target to the archive
		targetAbs = filepath.Join(filepath.Dir(source), target)
	}

	if !filepath.IsAbs(targetAbs) {
		targetAbs, err = osutil.RelativeSymlinkTargetToDir(targetAbs, filepath.Dir(source))
		if err != nil {
			return err
		}
	}

	archive.AddItem(targetAbs, targetAbs)

	// Now add the symlink itself
	destFilename := strings.TrimPrefix(dest, "/")

	archive.items.add(archiveItem{
		sourcePath: source,
		header: &cpio.Header{
			Name:     destFilename,
			Linkname: target,
			Mode:     0644 | cpio.ModeSymlink,
			Size:     int64(len(target)),
		},
	})

	return nil
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

func (archive *Archive) writeCompressed(path string, mode os.FileMode) (err error) {

	var compressor io.WriteCloser
	defer func() {
		e := compressor.Close()
		if e != nil && err == nil {
			err = e
		}
	}()

	fd, err := os.Create(path)
	if err != nil {
		return err
	}
	// Note: fd.Close omitted since it'll be closed in "compressor"

	switch archive.compress_format {
	case FormatGzip:
		level := gzip.DefaultCompression
		switch archive.compress_level {
		case LevelBest:
			level = gzip.BestCompression
		case LevelFast:
			level = gzip.BestSpeed
		}
		compressor, err = gzip.NewWriterLevel(fd, level)
		if err != nil {
			return err
		}
	case FormatLzma:
		compressor, err = xz.NewWriter(fd)
		if err != nil {
			return err
		}
	case FormatLz4:
		// The default compression for the lz4 library is Fast, and
		// they don't define a Default level otherwise
		level := lz4.Fast
		switch archive.compress_level {
		case LevelBest:
			level = lz4.Level9
		case LevelFast:
			level = lz4.Fast
		}

		var writer = lz4.NewWriter(fd)
		err = writer.Apply(lz4.LegacyOption(true), lz4.CompressionLevelOption(level))
		if err != nil {
			return err
		}
		compressor = writer
	case FormatNone:
		compressor = fd
	case FormatZstd:
		level := zstd.SpeedDefault
		switch archive.compress_level {
		case LevelBest:
			level = zstd.SpeedBestCompression
		case LevelFast:
			level = zstd.SpeedFastest
		}
		compressor, err = zstd.NewWriter(fd, zstd.WithEncoderLevel(level))
		if err != nil {
			return err
		}
	default:
		log.Print("Unknown or no compression format set, using gzip")
		compressor = gzip.NewWriter(fd)
	}

	if _, err = io.Copy(compressor, archive.buf); err != nil {
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
	// Just in case
	if osutil.HasMergedUsr() {
		archive.addSymlink("/bin", "/bin")
		archive.addSymlink("/sbin", "/sbin")
		archive.addSymlink("/lib", "/lib")
	}
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
					return fmt.Errorf("archive.writeCpio: Unable to open file %q, %w", source, err)
				}
				defer fd.Close()
				if _, err := io.Copy(archive.cpioWriter, fd); err != nil {
					return fmt.Errorf("archive.writeCpio: Couldn't process %q: %w", source, err)
				}
			} else if header.Linkname != "" {
				// the contents of a symlink is just need the link name
				if _, err := archive.cpioWriter.Write([]byte(header.Linkname)); err != nil {
					return fmt.Errorf("archive.writeCpio: unable to write out symlink: %q -> %q: %w", source, header.Linkname, err)
				}
			} else {
				return fmt.Errorf("archive.writeCpio: unknown type for file: %q: %d", source, header.Mode)
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
