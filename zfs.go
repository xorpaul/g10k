package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"

	"github.com/docker/docker/pkg/mount"
	"github.com/mistifyio/go-zfs"
)

func createZdevice(zfsDevice string) {
	_, err := zfs.GetDataset(zfsDevice)
	if err != nil {
		_, createErr := zfs.CreateFilesystem(zfsDevice, nil)
		if err != nil {
			panic(createErr)
		}
		Verbosef("created filesystem " + zfsDevice)
	} else {
		Verbosef("filesystem " + zfsDevice + " already existing")
	}
}

func ensureG10kMounted(zfsDevice string, gkMountpoint string) {
	if _, err := os.Stat(gkMountpoint); os.IsNotExist(err) {
		os.Mkdir(gkMountpoint, 0755)
	}

	status, _ := mount.Mounted(gkMountpoint)
	if status == false {
		g10kDataset, _ := zfs.GetDataset(zfsDevice)
		_, err := g10kDataset.Mount(false, nil)
		if err != nil {
			panic(err)
		} else {
			Verbosef("mounted filesytem " + zfsDevice)
		}
	} else {
		Verbosef("filesytem " + zfsDevice + " already mounted")
	}
}

func createSnapshot(nextSnap, zfsDevice string) {
	zfsDataset, _ := zfs.GetDataset(zfsDevice)
	_, err := zfsDataset.Snapshot(nextSnap, false)
	if err != nil {
		panic(err)
	} else {
		Verbosef("created snapshot " + zfsDevice + "@" + nextSnap)
	}
}

func destroySnapshots(snapList []*zfs.Dataset, zfsPool string) {
	mountedLine := "unmounted"
	for _, eachDataset := range snapList {
		zfsDevName := fmt.Sprintf("%v", eachDataset.Name)
		match, _ := regexp.MatchString("^"+zfsPool+"/g10k@+", zfsDevName)
		if match == true {
			f, err := os.Open("/proc/self/mountinfo")
			if err != nil {
				panic(err)
			}
			defer f.Close()

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				if strings.Contains(scanner.Text(), " "+zfsDevName+" ") {
					mountedLine = scanner.Text()
				}
			}
			if mountedLine != "unmounted" {
				snapMount := strings.Split(mountedLine, " ")[4]
				umountSnapshot(snapMount)
				mountedLine = "unmounted"
			}
			zfsDataset, err := zfs.GetDataset(zfsDevName)
			if err == nil {
				err := zfsDataset.Destroy(16)
				if err != nil {
					panic(err)
				} else {
					Verbosef("destroyed snapshot " + zfsDevName)
				}
			}
		}
	}
}

func umountSnapshot(mountPoint string) {
	status, _ := mount.Mounted(mountPoint)
	if status == true {
		err := mount.Unmount(mountPoint)
		if err != nil {
			panic(err)
		} else {
			Verbosef("unmounted " + mountPoint)
		}
	}
}

func mountSnapshot(mountPoint, zfsDevice, nextSnap string) {
	mountDevice := zfsDevice + "@" + nextSnap
	err := mount.Mount(mountDevice, mountPoint, "zfs", "defaults")
	if err != nil {
		panic(err)
	} else {
		fmt.Printf("mounted snapshot " + zfsDevice + "@" + nextSnap + " on " + mountPoint + "\n")
	}
}

func checkUserGroupExistence(userName, groupName string) {
	_, userErr := user.Lookup(userName)
	if userErr != nil {
		Verbosef("the user " + userName + " does not exist")
		panic(userErr)
	}
	_, groupErr := user.LookupGroup(groupName)
	if groupErr != nil {
		Verbosef("the group " + groupName + " does not exist")
		panic(groupErr)
	}
}

func chownRecursive(path string, userName, groupName int) {
	Verbosef("fixing files ownership under " + path)
	uidgid := fmt.Sprintf("%v:%v", userName, groupName)
	cmd := exec.Command("chown", "-R", uidgid, path)
	cmd.Run()
}
