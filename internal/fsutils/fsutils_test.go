package fsutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileExists(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if !FileExists(tmpFile.Name()) {
		t.Errorf("FileExists() = false, want true")
	}

	if FileExists("nonexistentfile") {
		t.Errorf("FileExists() = true, want false")
	}
}

func TestIsDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "testdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if !IsDir(tmpDir) {
		t.Errorf("IsDir() = false, want true")
	}

	if IsDir("nonexistentdir") {
		t.Errorf("IsDir() = true, want false")
	}
}

func TestNormalizeDir(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"//path//to//dir//", "/path/to/dir"},
		{"/path/to/dir/", "/path/to/dir"},
		{"/path/to/dir", "/path/to/dir"},
	}
	for _, tt := range tests {
		if got := NormalizeDir(tt.input); got != tt.want {
			t.Errorf("NormalizeDir(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCheckDirAndCreate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "testdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dir := filepath.Join(tmpDir, "newdir")
	CheckDirAndCreate(dir, "testdir")
	if !IsDir(dir) {
		t.Errorf("CheckDirAndCreate() did not create directory")
	}
}

func TestCreateOrPurgeDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "testdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dir := filepath.Join(tmpDir, "newdir")
	CreateOrPurgeDir(dir, "TestCreateOrPurgeDir")
	if !IsDir(dir) {
		t.Errorf("CreateOrPurgeDir() did not create directory")
	}

	CreateOrPurgeDir(dir, "TestCreateOrPurgeDir")
	if !IsDir(dir) {
		t.Errorf("CreateOrPurgeDir() did not purge and recreate directory")
	}
}

func TestPurgeDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "testdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dir := filepath.Join(tmpDir, "newdir")
	os.Mkdir(dir, 0777)
	PurgeDir(dir, "TestPurgeDir")
	if FileExists(dir) {
		t.Errorf("PurgeDir() did not remove directory")
	}
}

func TestMoveFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "testdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, "srcfile")
	destFile := filepath.Join(tmpDir, "destfile")
	os.WriteFile(srcFile, []byte("test content"), 0644)

	err = MoveFile(srcFile, destFile, true)
	if err != nil {
		t.Errorf("MoveFile() error = %v, want nil", err)
	}
	if !FileExists(destFile) {
		t.Errorf("MoveFile() did not move file")
	}
	if FileExists(srcFile) {
		t.Errorf("MoveFile() did not delete source file")
	}
}

func TestGetSha256sumFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := "test content"
	tmpFile.WriteString(content)
	tmpFile.Close()

	want := "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"
	if got := GetSha256sumFile(tmpFile.Name()); got != want {
		t.Errorf("GetSha256sumFile() = %q, want %q", got, want)
	}
}
