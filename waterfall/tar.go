// Package waterfall provides provides functionality to control remote devices.
package waterfall

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var modeDir os.FileMode = 0755

// Untar untars the contents of a reader into dst
func Untar(r io.Reader, dst string) error {
	tr := tar.NewReader(r)
	root := dst

	// Special case the first file to figure out where to unpack stream.
	hdr, err := tr.Next()
	if err != nil {
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
			f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
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
				log.Printf("Skipping broken link %s -> %s\n", file)
				return nil
			}

			// ok to ignore error file always lives inside src
			sr, _ := filepath.Rel(src, file)
			name := filepath.Join(filepath.Base(src), sr)
			if rel, err := filepath.Rel(src, t); err == nil {
				// Pointee lives under same root as pointer, so write the symlink.
				if hdr, err = tar.FileInfoHeader(fi, rel); err != nil {
					return err
				}
				hdr.Name = name
			} else {
				// Pointee lives outside pointer root, write the file pointed to.
				log.Printf("Resolving symlink %s before pushing\n", file)
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
