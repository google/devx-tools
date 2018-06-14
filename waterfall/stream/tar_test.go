package stream

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var (
	contents = testBytes()

	cases = []fs{
		fs{
			path:  "",
			files: []string{"foo.txt"},
		},
		fs{
			path:  "foo",
			files: []string{"a.txt", "b.txt"},
		},
		fs{
			path:  "zoo",
			files: []string{"a.txt", "b.txt"},
			dirs: []fs{
				fs{
					path: "zoo/bar",
				},
				fs{
					path:  "zoo/baz",
					files: []string{"d.txt"},
					dirs: []fs{
						fs{
							path:  "zoo/baz/qux",
							files: []string{"e.txt"},
						}}}}}}
)

type fs struct {
	path  string
	files []string
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

	for _, f := range tree.files {
		seen = append(seen, filepath.Join(tree.path, f))
		if err := makeTestFile(filepath.Join(dirPath, f), testBytes()); err != nil {
			return nil, err
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
	b := make([]byte, 64*1024)
	bb := bytes.NewBuffer(b)
	var i uint32
	for i = 0; i < uint32(len(b)); i++ {
		binary.Write(bb, binary.LittleEndian, i)
	}
	return bb.Bytes()
}

func makeTestFile(path string, bits []byte) error {
	return ioutil.WriteFile(path, bits, 0655)
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
	bs, err := ioutil.ReadFile(rf)
	if err != nil {
		return err
	}
	if bytes.Compare(contents, bs) != 0 {
		return fmt.Errorf("bytes for file %s not as expected", rel)
	}
	return nil
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
			tp = tc.files[0]
		}

		tf, err := ioutil.TempFile("", "src.tar")
		if err != nil {
			t.Fatalf("error creating tar file: %v", err)
		}
		defer os.Remove(tf.Name())

		if err := Tar(tf, filepath.Join(td, tp)); err != nil {
			t.Errorf("error taring file: %v", err)
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
			t.Errorf("error untaring cmd %v: %s, %v", args, out, err)
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
		}
		if len(seen) != len(all) {
			t.Errorf("wrong number of files. Got %v expected %v", seen, all)
		}
		for _, f := range all {
			if !seen[f] {
				t.Errorf("file %s not in tared files", f)
			}
		}
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
			tp = tc.files[0]
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
			t.Errorf("error taring cmd %v: %s, %v", args, out, err)
		}

		tr, err := ioutil.TempDir("", "tarres")
		if err != nil {
			t.Fatalf("error creating temp dir: %v", err)
		}
		defer os.RemoveAll(tr)

		if err := Untar(tf, tr); err != nil {
			t.Errorf("error untaring file: %v", err)
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
		}
		if len(seen) != len(all) {
			t.Errorf("wrong number of files. Got %v expected %v", seen, all)
		}
		for _, f := range all {
			if !seen[f] {
				t.Errorf("file %s not in tared files", f)
			}
		}
	}

}
