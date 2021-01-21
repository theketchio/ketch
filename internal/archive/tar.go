// Package archive contains functionality to create tar archives
package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	ignore "github.com/sabhiram/go-gitignore"

	"github.com/shipa-corp/ketch/internal/errors"
)

const (
	ignoreFile = ".ketchignore"
	currentDir = "."
)

// DefaultSourcePaths is a single element array containing current directory
func DefaultSourcePaths() []string {
	return []string{currentDir}
}

type fileIgnoreList struct {
	ign *ignore.GitIgnore
}

func createFileIgnoreList(ignoreFile string) (*fileIgnoreList, error) {
	var il fileIgnoreList
	ign, err := ignore.CompileIgnoreFile(ignoreFile)
	if err != nil {
		if !os.IsExist(err) {
			return &il, nil
		}
		return nil, err
	}
	il.ign = ign
	return &il, nil
}

func (fil fileIgnoreList) ignore(testFile string) bool {
	if fil.ign == nil {
		return false
	}
	return fil.ign.MatchesPath(testFile)
}

type tarOptions struct {
	dirs       []string
	files      []string
	workingDir string
}

type Option func(options *tarOptions)

// IncludeDirs defines directories to be included in tarball. Directories are relative to working directory.
func IncludeDirs(dir ...string) Option {
	return func(options *tarOptions) {
		options.dirs = append(options.dirs, dir...)
	}
}

// Defines files to be included in tarball. File paths are relative to the working directory.
// Files included here override .shipaignore
func IncludeFiles(file ...string) Option {
	return func(options *tarOptions) {
		options.files = append(options.files, file...)
	}
}

// WithWorkingDirectory set working or root directory, of this is not set it defaults to os.Getwd
func WithWorkingDirectory(path string) Option {
	return func(options *tarOptions) {
		options.workingDir = path
	}
}

// Create a tar file named archiveFile. Use includeFiles and includeDirs to define the files to
// be incorporated into the archive file. If no files or directories are specified the archive will be built
// rooted at the current directory. withWorkingDirectory is used to set working directory and defaults
// to the current working directory. The archival process can be instructed to exclude certain files by placing a
// special file .shipaignore in the working directory.  It uses pattern matches like .gitignore such the files
// matching the patterns are excluded from the archive. Files specified with includeFiles override .shipaignore and
// are added to the archive.
func Create(archiveFile string, inputs ...Option) error {
	var options tarOptions

	currentWorkingDir, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "could not get working directory")
	}
	// set default working dir
	options.workingDir = currentWorkingDir

	for _, input := range inputs {
		input(&options)
	}
	// if no directories or files are specified, default to current directory
	if len(options.dirs) == 0 && len(options.files) == 0 {
		options.dirs = DefaultSourcePaths()
	}

	// change to working directory, return to original directory when we're done.
	if err = os.Chdir(options.workingDir); err != nil {
		return err
	}
	defer os.Chdir(currentWorkingDir)

	ign, err := createFileIgnoreList(ignoreFile)
	if err != nil {
		return err
	}
	// walk directories, adding files to tarball
	var tarData bytes.Buffer
	tarWriter := tar.NewWriter(&tarData)
	defer tarWriter.Close()
	for _, path := range options.dirs {
		if err = writeDir(path, ign, tarWriter); err != nil {
			return err
		}
	}

	for _, file := range options.files {
		if err = writeFile(file, tarWriter); err != nil {
			return err
		}
	}

	if err := tarWriter.Flush(); err != nil {
		return err
	}
	// gzip tarball and write to file
	outF, err := os.OpenFile(archiveFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer outF.Close()
	gzipWtr := gzip.NewWriter(outF)
	defer gzipWtr.Close()
	if _, err := io.Copy(gzipWtr, &tarData); err != nil {
		return err
	}

	return nil
}

func writeDir(dir string, ignoreList *fileIgnoreList, w *tar.Writer) error {
	return filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("skipping %q\n", path)
			return nil
		}
		if ignoreList.ignore(path) {
			return nil
		}
		if !fi.IsDir() {
			return addToTarball(path, fi, w)
		}
		return nil
	})
}

func writeFile(path string, w *tar.Writer) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	return addToTarball(path, fi, w)
}

func addToTarball(path string, fi os.FileInfo, w *tar.Writer) error {
	hdr := &tar.Header{
		Name: path,
		Size: fi.Size(),
		Mode: int64(fi.Mode().Perm()),
	}
	if err := w.WriteHeader(hdr); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	if _, err = io.Copy(w, f); err != nil {
		return err
	}
	return nil
}
