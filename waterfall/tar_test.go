package waterfall

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	kindFile = iota
	kindSymlink
)

var (
	contents = testBytes()

	rootVar = "$ROOT"

	singleFile = fs{path: "", files: []file{file{name: "foo.txt"}}}
	dir        = fs{path: "foo", files: []file{file{name: "a.txt"}, file{name: "b.txt"}}}

	cases = []fs{
		singleFile,
		dir,
		fs{
			path: "zoo",
			files: []file{
				file{name: "a_symlink", kind: kindSymlink, link: rootVar + "/zoo/baz/d.txt"},
				file{name: "a.txt"},
				file{name: "b.txt"}},
			dirs: []fs{
				fs{path: "zoo/bar"},
				fs{
					path: "zoo/baz",
					files: []file{
						file{name: "d.txt"}},
					dirs: []fs{
						fs{
							path:  "zoo/baz/qux",
							files: []file{file{name: "e.txt"}},
						}}}}}}
)

type file struct {
	name string
	link string
	kind int
}

type fs struct {
	path  string
	files []file
	dirs  []fs
}

func makeFs(baseDir string, tree fs) ([]string, error) {
	seen := []string{}
	dirPath := filepath.Join(baseDir, tree.path)
	if tree.path != "" {
		seen = append(seen, tree.path)
		if err := os.Mkdir(dirPath, modeDir); err != nil {
			return nil, err
		}
	}

	bits := testBytes()
	for _, f := range tree.files {
		seen = append(seen, filepath.Join(tree.path, f.name))
		switch f.kind {
		case kindFile:
			if err := ioutil.WriteFile(filepath.Join(dirPath, f.name), bits, 0655); err != nil {
				return nil, err
			}
		case kindSymlink:
			link := f.link
			if strings.Contains(link, rootVar) {
				link = filepath.Join(baseDir, strings.Split(link, rootVar+"/")[1])
			}
			if err := os.Symlink(link, filepath.Join(dirPath, f.name)); err != nil {
				return nil, err
			}
		}
	}

	for _, nfs := range tree.dirs {
		s, err := makeFs(baseDir, nfs)
		if err != nil {
			return nil, err
		}
		seen = append(seen, s...)
	}
	return seen, nil
}

func testBytes() []byte {
	// Just create a sequential byte stream to avoid having to create test files
	b := make([]byte, 1024)
	bb := bytes.NewBuffer(b)
	var i uint32
	for i = 0; i < uint32(len(b)); i++ {
		binary.Write(bb, binary.LittleEndian, i)
	}
	return bb.Bytes()
}

func fileCompare(f string) error {
	bs, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}
	if bytes.Compare(contents, bs) != 0 {
		return fmt.Errorf("bytes for file %s not as expected", f)
	}
	return nil
}

func dirCompare(dir1, dir2, path string, info os.FileInfo, seen map[string]bool) error {
	if path == dir1 {
		return nil
	}

	rel := path[len(dir1)+1:]
	seen[rel] = true
	if info.IsDir() {
		return nil
	}
	rf := filepath.Join(dir2, rel)
	if _, err := os.Stat(rf); os.IsNotExist(err) {
		return err
	}
	return fileCompare(rf)
}

func TestTar(t *testing.T) {
	for _, tc := range cases {
		td, err := ioutil.TempDir("", "tartest")
		if err != nil {
			t.Fatalf("error creating temp dir: %v", err)
		}
		defer os.RemoveAll(td)

		all, err := makeFs(td, tc)
		if err != nil {
			t.Fatalf("error creating temp fs: %v", err)
		}

		tp := tc.path
		if tc.path == "" {
			// This is a single file test case
			tp = tc.files[0].name
		}

		tf, err := ioutil.TempFile("", "src.tar")
		if err != nil {
			t.Fatalf("error creating tar file: %v", err)
		}
		defer os.Remove(tf.Name())

		if err := Tar(tf, filepath.Join(td, tp)); err != nil {
			t.Errorf("error taring file: %v", err)
			return
		}

		// Untar using reference implementation
		tr, err := ioutil.TempDir("", "tarres")
		if err != nil {
			t.Fatalf("error creating temp dir: %v", err)
		}
		defer os.RemoveAll(tr)

		args := []string{"-C", tr, "-xf", tf.Name()}
		cmd := exec.Command("tar", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("error untaring cmd %v: %s, %v", args, out, err)
		}

		// Walk the original dir and compare against extracted
		seen := make(map[string]bool)
		err = filepath.Walk(td, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return dirCompare(td, tr, path, info, seen)
		})

		if err != nil {
			t.Errorf("got error taring: %v", err)
			return
		}
		if len(seen) != len(all) {
			t.Errorf("wrong number of files. Got %v expected %v", seen, all)
			return
		}
		for _, f := range all {
			if !seen[f] {
				t.Errorf("file %s not in tared files", f)
				return
			}
		}
	}
}

func makeTar(dir, name string) (*os.File, error) {
	// uses tar bin to tar using reference implementation
	tf, err := ioutil.TempFile("", "src.tar")
	if err != nil {
		return nil, err
	}

	// tar using reference implementation
	args := []string{"-C", dir, "-cf", tf.Name(), name}
	cmd := exec.Command("tar", args...)
	if _, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tf.Name())
		return nil, err
	}
	return tf, nil
}

// TestUntar[A-Za-z]+ tests the different possible scenarios for the creation of
// the first entry in the tar stream

// TestUntarDirDstIsDir push /foo/bar /remote/existing_dir (dir is created inside dir)
func TestUntarDirDstIsDir(t *testing.T) {
	td, err := ioutil.TempDir("", "untartest")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)

	all, err := makeFs(td, dir)
	if err != nil {
		t.Fatalf("error creating temp fs: %v", err)
	}

	tp := dir.path
	tf, err := makeTar(td, tp)
	if err != nil {
		t.Fatalf("error creating taring src: %v", err)
	}
	defer tf.Close()
	defer os.RemoveAll(tf.Name())

	tr, err := ioutil.TempDir("", "tarres")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tr)

	if err := Untar(tf, tr); err != nil {
		t.Errorf("error untaring file: %v", err)
		return
	}

	// Walk the original dir and compare against extracted
	seen := make(map[string]bool)
	err = filepath.Walk(td, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return dirCompare(td, tr, path, info, seen)
	})

	if err != nil {
		t.Errorf("got error untaring: %v", err)
		return
	}
	if len(seen) != len(all) {
		t.Errorf("wrong number of files. Got %v expected %v", seen, all)
		return
	}
	for _, f := range all {
		if !seen[f] {
			t.Errorf("file %s not in tared files", f)
			return
		}
	}
}

// TestUntarDirDstIsFile push /foo/bar /remote/existing_file (invalid operation)
func TestUntarDirDstIsFile(t *testing.T) {
	td, err := ioutil.TempDir("", "untartest")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)

	if _, err := makeFs(td, dir); err != nil {
		t.Fatalf("error creating temp fs: %v", err)
	}

	tp := dir.path
	tf, err := makeTar(td, tp)
	if err != nil {
		t.Fatalf("error creating taring src: %v", err)
	}
	defer tf.Close()
	defer os.RemoveAll(tf.Name())

	tr, err := ioutil.TempDir("", "tarres")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tr)

	dst := filepath.Join(tr, "file.txt")
	f, err := os.Create(dst)
	if err != nil {
		t.Fatal(f)
	}
	f.Close()

	if err := Untar(tf, dst); err == nil {
		t.Errorf("expected error but got none")
		return
	}
}

// TestUntarDirDstNotExist push /foo/bar /remote/non_existing_path (dir is created in remote path)
func TestUntarDirDstNotExist(t *testing.T) {
	td, err := ioutil.TempDir("", "untartest")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)

	all, err := makeFs(td, dir)
	if err != nil {
		t.Fatalf("error creating temp fs: %v", err)
	}

	tp := dir.path
	tf, err := makeTar(td, tp)
	if err != nil {
		t.Fatalf("error creating taring src: %v", err)
	}
	defer tf.Close()
	defer os.RemoveAll(tf.Name())

	tr, err := ioutil.TempDir("", "tarres")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tr)

	newName := "new_dir"
	dst := filepath.Join(tr, newName)
	if err := Untar(tf, dst); err != nil {
		t.Errorf("got error untaring: %v", err)
		return
	}

	if err := os.Rename(filepath.Join(td, tp), filepath.Join(td, newName)); err != nil {
		t.Fatalf("error renaming: %v", err)
	}

	// Walk the original dir and compare against extracted
	seen := make(map[string]bool)
	err = filepath.Walk(td, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return dirCompare(td, tr, path, info, seen)
	})

	if err != nil {
		t.Errorf("got error untaring: %v", err)
		return
	}
	if len(seen) != len(all) {
		t.Errorf("wrong number of files. Got %v expected %v", seen, all)
		return
	}
	for _, f := range all {
		if !seen[fmt.Sprintf("%s%s", newName, strings.TrimLeft(f, tp))] {
			t.Errorf("file %s not in tared files", f)
			return
		}
	}

}

// TestUntarFileDstIsDir push /foo/bar/file.txt /remote/existing_dir (file is created in dir)
func TestUntarFileDstIsDir(t *testing.T) {
	td, err := ioutil.TempDir("", "untartest")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)

	if _, err := makeFs(td, singleFile); err != nil {
		t.Fatalf("error creating temp fs: %v", err)
	}
	tp := singleFile.files[0].name

	tf, err := makeTar(td, tp)
	if err != nil {
		t.Fatalf("error creating taring src: %v", err)
	}
	defer tf.Close()
	defer os.RemoveAll(tf.Name())

	tr, err := ioutil.TempDir("", "tarres")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tr)

	if err := Untar(tf, tr); err != nil {
		t.Errorf("error untaring file: %v", err)
		return
	}

	if _, err := os.Stat(filepath.Join(tr, tp)); err != nil {
		t.Errorf("error untaring file: %v", err)
	}
}

// TestUntarFileDstIsFile push /foo/bar/file.txt /remote/file.txt (replace file)
func TestUntarFileDstIsFile(t *testing.T) {
	td, err := ioutil.TempDir("", "untartest")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)

	if _, err := makeFs(td, singleFile); err != nil {
		t.Fatalf("error creating temp fs: %v", err)
	}
	tp := singleFile.files[0].name

	tf, err := makeTar(td, tp)
	if err != nil {
		t.Fatalf("error creating taring src: %v", err)
	}
	defer tf.Close()
	defer os.RemoveAll(tf.Name())

	tr, err := ioutil.TempDir("", "tarres")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tr)

	// create the remote file
	dst := filepath.Join(tr, tp)
	f, err := os.Create(dst)
	if err != nil {
		t.Fatalf("error creating existing file: %v", err)
	}

	if _, err := f.Write([]byte("foo")); err != nil {
		t.Fatalf("error writing existing file: %v", err)
	}
	f.Close()

	if err := Untar(tf, dst); err != nil {
		t.Errorf("error untaring file: %v", err)
		return
	}

	if _, err := os.Stat(dst); err != nil {
		t.Errorf("error untaring file: %v", err)
		return
	}

	if err := fileCompare(dst); err != nil {
		t.Error(err)
	}
}

// TestUntarFileDstNotExist /foo/bar/file.txt /remote/non_existing_path (create file)
func TestUntarFileDstNotExist(t *testing.T) {
	td, err := ioutil.TempDir("", "untartest")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)

	if _, err := makeFs(td, singleFile); err != nil {
		t.Fatalf("error creating temp fs: %v", err)
	}
	tp := singleFile.files[0].name

	tf, err := makeTar(td, tp)
	if err != nil {
		t.Fatalf("error creating taring src: %v", err)
	}
	defer tf.Close()
	defer os.RemoveAll(tf.Name())

	tr, err := ioutil.TempDir("", "tarres")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tr)

	dst := filepath.Join(tr, "new_file.txt")
	if err := Untar(tf, dst); err != nil {
		t.Errorf("error untaring file: %v", err)
		return
	}

	if _, err := os.Stat(dst); err != nil {
		t.Errorf("error untaring file: %v", err)
		return
	}
}

func TestUntar(t *testing.T) {
	for _, tc := range cases {
		td, err := ioutil.TempDir("", "untartest")
		if err != nil {
			t.Fatalf("error creating temp dir: %v", err)
		}
		defer os.RemoveAll(td)

		all, err := makeFs(td, tc)
		if err != nil {
			t.Fatalf("error creating temp fs: %v", err)
		}

		tp := tc.path
		if tc.path == "" {
			// This is a single file test case
			tp = tc.files[0].name
		}

		tf, err := ioutil.TempFile("", "src.tar")
		if err != nil {
			t.Fatalf("error creating tar file: %v", err)
		}
		defer os.Remove(tf.Name())

		// tar using reference implementation
		args := []string{"-C", td, "-cf", tf.Name(), tp}

		cmd := exec.Command("tar", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("error taring cmd %v: %s, %v", args, out, err)
		}

		tr, err := ioutil.TempDir("", "tarres")
		if err != nil {
			t.Fatalf("error creating temp dir: %v", err)
		}
		defer os.RemoveAll(tr)

		if err := Untar(tf, tr); err != nil {
			t.Errorf("error untaring file: %v", err)
			return
		}

		// Walk the original dir and compare against extracted
		seen := make(map[string]bool)
		err = filepath.Walk(td, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return dirCompare(td, tr, path, info, seen)
		})

		if err != nil {
			t.Errorf("got error untaring: %v", err)
			return
		}
		if len(seen) != len(all) {
			t.Errorf("wrong number of files. Got %v expected %v", seen, all)
			return
		}
		for _, f := range all {
			if !seen[f] {
				t.Errorf("file %s not in tared files", f)
				return
			}
		}
	}

}
