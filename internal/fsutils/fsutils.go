package fsutils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/xorpaul/g10k/internal"
	"github.com/xorpaul/g10k/internal/logging"
	"golang.org/x/sys/unix"
)

// fileExists checks if the given file exists and returns a bool
func FileExists(file string) bool {
	//logging.Debugf("checking for file existence " + file)
	if _, err := os.Lstat(file); os.IsNotExist(err) {
		return false
	}
	return true
}

// isDir checks if the given dir exists and returns a bool
func IsDir(dir string) bool {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false
	}
	if fi.Mode().IsDir() {
		return true
	}
	return false
}

// normalizeDir removes from the given directory path multiple redundant slashes and removes a trailing slash
func NormalizeDir(dir string) string {
	if strings.Count(dir, "//") > 0 {
		dir = NormalizeDir(strings.Replace(dir, "//", "/", -1))
	}
	dir = strings.TrimSuffix(dir, "/")
	return dir
}

// checkDirAndCreate tests if the given directory exists and tries to create it
func CheckDirAndCreate(dir string, name string) string {
	if !internal.DryRun {
		funcname := logging.FuncName()
		if len(dir) != 0 {
			if !FileExists(dir) {
				if err := os.MkdirAll(dir, 0777); err != nil {
					logging.Fatalf(funcname + "s: Error: failed to create directory: " + dir)
				}
			} else {
				if !IsDir(dir) {
					logging.Fatalf(funcname + ": Error: " + dir + " exists, but is not a directory! Exiting!")
				} else {
					if unix.Access(dir, unix.W_OK) != nil {
						logging.Fatalf(funcname + ": Error: " + dir + " exists, but is not writable! Exiting!")
					}
				}
			}
		} else {
			// TODO make dir optional
			logging.Fatalf(funcname + ": Error: dir setting '" + name + "' missing! Exiting!")
		}
	}
	dir = NormalizeDir(dir)
	logging.Debugf("Using as " + name + ": " + dir)
	return dir
}

func CreateOrPurgeDir(dir string, callingFunction string) {
	if !internal.DryRun {
		if !FileExists(dir) {
			logging.Debugf("Trying to create dir: " + dir + " called from " + callingFunction)
			os.MkdirAll(dir, 0777)
		} else {
			logging.Debugf("Trying to remove: " + dir + " called from " + callingFunction)
			if err := os.RemoveAll(dir); err != nil {
				log.Print("createOrPurgeDir(): error: removing dir failed", err)
			}
			logging.Debugf("Trying to create dir: " + dir + " called from " + callingFunction)
			os.MkdirAll(dir, 0777)
		}
	}
}

func PurgeDir(dir string, callingFunction string) {
	if !FileExists(dir) {
		logging.Debugf("Unnecessary to remove dir: " + dir + " it does not exist. Called from " + callingFunction)
	} else {
		logging.Debugf("Trying to remove: " + dir + " called from " + callingFunction)
		if err := os.RemoveAll(dir); err != nil {
			log.Print("fsutils.PurgeDir(): os.RemoveAll() error: removing dir failed: ", err.Error())
			if err = syscall.Unlink(dir); err != nil {
				log.Print("fsutils.PurgeDir(): syscall.Unlink() error: removing link failed: ", err.Error())
			}
		}
	}
}

// MoveFile uses io.Copy to create a copy of the given file https://stackoverflow.com/a/50741908/682847
func MoveFile(sourcePath, destPath string, deleteSourceFileToggle bool) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("couldn't open source file: %s", err)
	}
	outputFile, err := os.Create(destPath)
	if err != nil {
		inputFile.Close()
		return fmt.Errorf("couldn't open dest file: %s", err)
	}
	defer outputFile.Close()
	_, err = io.Copy(outputFile, inputFile)
	inputFile.Close()
	if err != nil {
		return fmt.Errorf("writing to output file failed: %s", err)
	}
	if deleteSourceFileToggle {
		// The copy was successful, so now delete the original file
		err = os.Remove(sourcePath)
		if err != nil {
			return fmt.Errorf("failed removing original file: %s", err)
		}
	}
	return nil
}

// GetSha256sumFile calculates the SHA256 sum of the given file
func GetSha256sumFile(file string) string {
	f, err := os.Open(file)
	if err != nil {
		logging.Fatalf("failed to open file " + file + " to calculate SHA256 sum. Error: " + err.Error())
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		logging.Fatalf("failed to calculate SHA256 sum of file " + file + " Error: " + err.Error())
	}

	return hex.EncodeToString(h.Sum(nil))
}
