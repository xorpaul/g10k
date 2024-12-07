package fsutils

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func createTestTar() ([]byte, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	// Create a directory
	hdr := &tar.Header{
		Name:     "testdir/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}

	// Create a regular file
	hdr = &tar.Header{
		Name:     "testdir/testfile.txt",
		Mode:     0644,
		Size:     int64(len("hello world")),
		Typeflag: tar.TypeReg,
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte("hello world")); err != nil {
		return nil, err
	}

	// Create a symlink
	hdr = &tar.Header{
		Name:     "testdir/testsymlink",
		Linkname: "testfile.txt",
		Typeflag: tar.TypeSymlink,
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}

	// Create a hardlink
	hdr = &tar.Header{
		Name:     "testdir/testhardlink",
		Linkname: "testdir/testfile.txt",
		Typeflag: tar.TypeLink,
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestUnTar(t *testing.T) {
	tarData, err := createTestTar()
	if err != nil {
		t.Fatalf("Failed to create test tar: %v", err)
	}

	targetDir, err := os.MkdirTemp("", "tar_test")
	if err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}
	//defer os.Remove(targetDir)

	r := bytes.NewReader(tarData)
	UnTar(r, targetDir, targetDir, []string{})

	// Check if directory was created
	if _, err := os.Stat(filepath.Join(targetDir, "testdir")); os.IsNotExist(err) {
		t.Errorf("Directory not created: %v", err)
	}

	// Check if file was created
	if _, err := os.Stat(filepath.Join(targetDir, "testdir/testfile.txt")); os.IsNotExist(err) {
		t.Errorf("File not created: %v", err)
	}

	// Check if symlink was created
	symlinkPath := filepath.Join(targetDir, "testdir/testsymlink")
	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		t.Errorf("Symlink not created: %v", err)
	} else {
		linkTarget, err := os.Readlink(symlinkPath)
		if err != nil {
			t.Errorf("Failed to read symlink: %v", err)
		}
		if linkTarget != "testfile.txt" {
			t.Errorf("Symlink points to wrong target: %v", linkTarget)
		}
	}

	// Check if hardlink was created
	hardlinkPath := filepath.Join(targetDir, "testdir/testhardlink")
	if _, err := os.Stat(hardlinkPath); os.IsNotExist(err) {
		t.Errorf("Hardlink not created: %v", err)
	} else {
		origFileInfo, _ := os.Stat(filepath.Join(targetDir, "testdir/testfile.txt"))
		hardlinkFileInfo, _ := os.Stat(hardlinkPath)
		if !os.SameFile(origFileInfo, hardlinkFileInfo) {
			t.Errorf("Hardlink does not point to the same inode")
		}
	}
}
