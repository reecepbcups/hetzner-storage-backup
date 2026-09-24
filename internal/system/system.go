// Package system ports src/system.py: shells out to standard Linux tools
// for storage/RAM/hostname info, same as the Python version.
package system

import (
	"fmt"
	"os/exec"
	"strings"
)

func run(shellCmd string) string {
	out, _ := exec.Command("sh", "-c", shellCmd).Output()
	return strings.TrimSpace(string(out))
}

// GetStorageAmount returns size, used, free, percentUsed for the root filesystem.
func GetStorageAmount() (size, used, free, percentUsed string) {
	fields := strings.Fields(run(`df -h / | grep /`))
	size, used, free, percentUsed = fields[1], fields[2], fields[3], fields[4]
	fmt.Printf("size=%q used=%q free=%q percentUsed=%q\n", size, used, free, percentUsed)
	return
}

// GetRamUsage returns totalRam, usedRam, percentUsed.
func GetRamUsage() (totalRam, usedRam, percentUsed string) {
	totalRam = run(`free -h | grep Mem | awk '{print $2}'`)
	usedRam = run(`free -h | grep Mem | awk '{print $3}'`)
	percentUsed = run(`free -m | grep Mem | awk '{print (($3/$2)*100)}'`)
	fmt.Printf("System is using %s%% of TOTAL RAM (%s/%s)\n", percentUsed, usedRam, totalRam)
	return
}

// GetLargestDirs returns the top num largest subdirectories/files under path.
func GetLargestDirs(path string, num int) string {
	return run(fmt.Sprintf("du -h %s | sort -rh | head -%d", path, num))
}

// GetCurrentHostname returns the machine hostname via the `hostname` command.
func GetCurrentHostname() string {
	return run("hostname")
}

// GetNetworkUsage prints and returns `ip -h -s link` output.
func GetNetworkUsage() string {
	out := run("ip -h -s link")
	fmt.Println(out)
	return out
}
