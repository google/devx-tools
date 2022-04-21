// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stream

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	modeDir os.FileMode = 0755
)

// Untar untars the contents of a reader into dst
func Untar(r io.Reader, dst string) error {
	tr := tar.NewReader(r)
	root := dst

	// Special case the first file to figure out where to unpack stream.
	hdr, err := tr.Next()
	if err != nil {
		if err == io.EOF {
			return errors.New("received empty Tar file")
		}
		return err
	}

	var name string
	var rootReplace string

	// ferr is used to determine where to unpack the stream
	fi, ferr := os.Stat(dst)
	switch hdr.Typeflag {
	case tar.TypeSymlink:
		fallthrough
	case tar.TypeRegA:
		fallthrough
	case tar.TypeReg:
		if ferr == nil {
			// dst exists and user specified directory as dst
			// eg push /foo/bar/file.txt /baz/some_dir
			if fi.Mode().IsDir() {
				name = filepath.Join(dst, hdr.Name)
				root = dst
			} else {
				// dst exists and user specified file name as dst
				// eg push /foo/bar/file.txt /baz/some_dir/file.txt
				name = dst
				root = filepath.Dir(dst)
			}
			break
		}
		if os.IsNotExist(ferr) {
			// dst does not exist push to parent dir and rename to dst.
			root = filepath.Dir(dst)
			if ferr = os.MkdirAll(root, modeDir); ferr == nil {
				name = dst
				break
			}
		}
		return ferr
	case tar.TypeDir:
		if ferr == nil {
			if fi.Mode().IsDir() {
				// dst exists and is a directory. Create dir in directory
				name = filepath.Join(dst, hdr.Name)
				break
			}
			return fmt.Errorf("file %s exists and is not a directory", dst)
		} else if os.IsNotExist(ferr) {
			root = dst
			name = dst
			rootReplace = hdr.Name
			break
		}
		return ferr
	}

	for {
		switch hdr.Typeflag {
		case tar.TypeRegA:
			fallthrough
		case tar.TypeReg:
			os.RemoveAll(name)
			f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
			if err != nil {
				return err
			}
			// Close file ASAP to avoid too many open files
			if _, err = io.Copy(f, tr); err != nil {
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		case tar.TypeDir:
			if err := os.MkdirAll(name, modeDir); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.Symlink(hdr.Linkname, name); err != nil {
				return err
			}
		}

		hdr, err = tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name = filepath.Join(root, strings.TrimPrefix(hdr.Name, rootReplace))
	}
}

// UntarBytes untars a tar r to a dst writer. This assumes that the tar contains only a single
// file.
func UntarBytes(dst io.Writer, r io.Reader) error {
	tr := tar.NewReader(r)
	// Advance to the first and only file in the tar. If there are more files present in the
	// tar, return an error.
	hdr, err := tr.Next()
	if err != nil {
		return err
	}
	switch hdr.Typeflag {
	case tar.TypeReg:
		fallthrough
	case tar.TypeRegA:
		if _, err = io.Copy(dst, tr); err != nil {
			return err
		}
	default:
		return fmt.Errorf("trying to read a directory into writer")
	}
	if _, eof := tr.Next(); eof != io.EOF {
		return fmt.Errorf("too many files received when trying to untar file")
	}
	return nil
}

// Tar archive tars the contents of src into a writer stream
func Tar(writer io.Writer, src string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}

	// avoid making the directory relative to itself
	src = strings.TrimRight(src, "/")

	tw := tar.NewWriter(writer)
	defer tw.Close()

	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		var hdr *tar.Header
		if fi.Mode()&os.ModeSymlink != 0 {
			t, err := filepath.EvalSymlinks(file)
			if err != nil {
				// Don't include broken symlinks
				log.Printf("Skipping broken link %s\n", file)
				return nil
			}

			// ok to ignore error file always lives inside src
			sr, _ := filepath.Rel(src, file)
			name := filepath.Join(filepath.Base(src), sr)
			if strings.HasPrefix(t, src) {
				// Pointee lives under same root as pointer, so write the symlink.
				if hdr, err = tar.FileInfoHeader(fi, t); err != nil {
					return err
				}
				hdr.Name = name
			} else {
				// Pointee lives outside pointer root, write the file pointed to.
				var ti os.FileInfo
				if ti, err = os.Stat(t); err != nil {
					// Skip broken links. Error out on everything else.
					if os.IsNotExist(err) {
						log.Printf("Skipping broken link %s -> %s\n", file, t)
						return nil
					}
					return err
				}
				if !ti.Mode().IsRegular() {
					return nil
				}
				if hdr, err = tar.FileInfoHeader(ti, ""); err != nil {
					return err
				}
				hdr.Name = name
			}
		} else {
			hdr, err = tar.FileInfoHeader(fi, fi.Name())
			if err != nil {
				return err
			}

			rel, err := filepath.Rel(filepath.Dir(src), file)
			if err != nil {
				return err
			}
			hdr.Name = rel
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if (hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA) || hdr.Size == 0 {
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

// TarBytes tars bytes as a single logical file into a tar file writer.
func TarBytes(writer io.Writer, bytes []byte) error {
	tw := tar.NewWriter(writer)
	defer tw.Close()

	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "bytefile",
		Size:     int64(len(bytes)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	if _, err := tw.Write(bytes); err != nil {
		return err
	}
	return nil
}
