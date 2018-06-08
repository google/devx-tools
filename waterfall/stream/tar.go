// Package stream provides functions to handle different stream formats
package stream

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var modeDir os.FileMode = 0755

// Untar untars the contents of a reader into path
func Untar(r io.Reader, path string) error {
	tr := tar.NewReader(r)
	for {
		hr, err := tr.Next()

		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		target := filepath.Join(path, hr.Name)
		switch hr.Typeflag {
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), modeDir); err != nil {
				return err
			}
			f, err := os.OpenFile(
				target, os.O_CREATE|os.O_RDWR, os.FileMode(hr.Mode))
			if err != nil {
				return err
			}
			// Close file ASAP to avoid too many open files
			_, err = io.Copy(f, tr)
			if err := f.Close(); err != nil {
				return err
			}
			if err != nil {
				return err
			}
		case tar.TypeDir:
			if err := os.MkdirAll(target, modeDir); err != nil {
				return err
			}
		case tar.TypeLink:
			return errors.New("error: unimplemented mode TypeLink")
		case tar.TypeSymlink:
			return errors.New("error: unimplemented mode TypeSymLink")
		case tar.TypeXGlobalHeader:
			return errors.New("error: unimplemented mode TypeSymLink")
		default:
			return errors.New("error: unimplemented mode XGlobalHeader")
		}
	}
}

// Tar archive tars the contents of src into a writer stream
func Tar(writer io.Writer, src string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}
	baseDir := src

	tw := tar.NewWriter(writer)
	defer tw.Close()

	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {

		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(filepath.Dir(baseDir), file)
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		return nil
	})
}
