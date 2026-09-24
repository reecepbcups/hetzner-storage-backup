// Package backup ports src/backup.py: zip configured directories, optionally
// dump MongoDB first, upload to a Hetzner SFTP storage box, prune old backups.
package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/reecepbcups/hetzner-storage-backup/internal/config"
	"github.com/reecepbcups/hetzner-storage-backup/internal/cosmetics"
	"github.com/reecepbcups/hetzner-storage-backup/internal/notifications"
	"github.com/reecepbcups/hetzner-storage-backup/internal/system"
)

// TimeFormat mirrors Python's "%b-%d-%Y_%I-%M-%S%p".
const TimeFormat = "Jan-02-2006_03-04-05PM"

type Backup struct {
	cfg *config.Config

	currentTime time.Time
	debug       bool
	showIgnored bool
	showSuccess bool

	rootPaths       map[string]config.ParentPath
	backupPath      string
	maxLocalBackups int
	discordWebhook  string
	saveRelative    bool

	zipFilename        string
	backupFileName     string
	mongodbAbsLocation string
}

// New mirrors Backup.__init__ in backup.py.
func New(cfg *config.Config, debug, showIgnored, showSuccess bool) (*Backup, error) {
	if cfg.Backups == nil {
		cosmetics.CPrint("&cNo backup section in config, grab example from 'secret.json.example'")
		os.Exit(0)
	}

	b := &Backup{
		cfg:             cfg,
		currentTime:     time.Now(),
		debug:           debug,
		showIgnored:     showIgnored,
		showSuccess:     showSuccess,
		rootPaths:       cfg.Backups.ParentPaths,
		backupPath:      cfg.Backups.SaveLocation,
		maxLocalBackups: cfg.Backups.MaxLocalBackups,
		discordWebhook:  cfg.DiscordWebhook,
		saveRelative:    cfg.Backups.SaveRelative,
	}
	if b.maxLocalBackups <= 0 {
		b.maxLocalBackups = 0
	}
	if b.rootPaths == nil {
		b.rootPaths = map[string]config.ParentPath{}
	}

	if _, err := os.Stat(b.backupPath); os.IsNotExist(err) {
		if err := os.MkdirAll(b.backupPath, 0o755); err != nil {
			return nil, fmt.Errorf("creating backup path %s: %w", b.backupPath, err)
		}
	}

	b.zipFilename = fmt.Sprintf("%s_%s.zip", system.GetCurrentHostname(), b.currentTime.Format(TimeFormat))
	b.backupFileName = filepath.Join(b.backupPath, b.zipFilename)

	return b, nil
}

// backupMongoDB mirrors Backup.backup_mongodb.
func (b *Backup) backupMongoDB(mongoCfg config.MongoDB) error {
	b.mongodbAbsLocation = filepath.Join(b.backupPath, fmt.Sprintf("mongodb_dump_%s", b.currentTime.Format(TimeFormat)))

	cmd := exec.Command("mongodump", fmt.Sprintf("--uri=%s", mongoCfg.BackupURI), "--out", b.mongodbAbsLocation)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cosmetics.CPrint("&cMongoDB backup failed!")
		os.Exit(0)
	}

	// Adds mongodb absolute location to the paths to be looped through
	b.rootPaths["mongodb"] = config.ParentPath{
		Path:   b.mongodbAbsLocation,
		Ignore: []string{},
	}
	return nil
}

// DeleteOldestFilesInDirIfOverMax mirrors Backup.delete_oldest_files_in_dir_if_over_max.
func (b *Backup) DeleteOldestFilesInDirIfOverMax() error {
	entries, err := os.ReadDir(b.backupPath)
	if err != nil {
		return err
	}

	if len(entries) <= b.maxLocalBackups {
		return nil
	}

	toRemove := len(entries) - b.maxLocalBackups
	for i := 0; i < toRemove; i++ {
		entries, err := os.ReadDir(b.backupPath)
		if err != nil {
			return err
		}

		var oldestPath string
		var oldestTime time.Time
		for _, e := range entries {
			full := filepath.Join(b.backupPath, e.Name())
			info, err := os.Stat(full)
			if err != nil {
				continue
			}
			ctime := info.ModTime()
			if oldestPath == "" || ctime.Before(oldestTime) {
				oldestPath = full
				oldestTime = ctime
			}
		}
		if oldestPath == "" {
			return nil
		}

		info, err := os.Stat(oldestPath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := os.RemoveAll(oldestPath); err != nil {
				return err
			}
			cosmetics.CPrint(fmt.Sprintf("&cRemoved %s as it was the oldest backup directory", oldestPath))
		} else {
			if err := os.Remove(oldestPath); err != nil {
				return err
			}
			cosmetics.CPrint(fmt.Sprintf("&cRemoved %s as it was the oldest backup file", oldestPath))
		}
	}
	return nil
}

// ZipFiles mirrors Backup.zip_files.
func (b *Backup) ZipFiles() error {
	zipFile, err := os.Create(b.backupFileName)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	mongoCfg := b.cfg.Backups.Database.MongoDB
	if mongoCfg.Enabled {
		if err := b.backupMongoDB(mongoCfg); err != nil {
			return err
		}
	}

	for nickname, pathCfg := range b.rootPaths {
		rootPath := pathCfg.Path
		ignoreRegex := pathCfg.Ignore

		info, err := os.Stat(rootPath)
		if err != nil || !info.IsDir() {
			cosmetics.CPrint(fmt.Sprintf("\n&c%s is not a directory, ignoring...", rootPath))
			continue
		}

		compiled := make([]*regexp.Regexp, 0, len(ignoreRegex))
		for _, pattern := range ignoreRegex {
			re, err := regexp.Compile(pattern)
			if err != nil {
				cosmetics.CPrint(fmt.Sprintf("&cInvalid ignore regex %q for %s: %v", pattern, nickname, err))
				continue
			}
			compiled = append(compiled, re)
		}

		textOutput := ""
		newlineCount := 0

		err = filepath.Walk(rootPath, func(absPath string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fi.IsDir() {
				return nil
			}

			var relativeFilename string
			if rootPath == b.mongodbAbsLocation {
				rel := strings.TrimPrefix(strings.TrimPrefix(absPath, b.mongodbAbsLocation), string(os.PathSeparator))
				relativeFilename = filepath.Join("mongodb", rel)
			} else {
				rel := strings.TrimPrefix(strings.TrimPrefix(absPath, rootPath), string(os.PathSeparator))
				relativeFilename = filepath.Join(nickname, rel)
			}

			ignored := false
			for _, re := range compiled {
				if re.MatchString(absPath) {
					ignored = true
					break
				}
			}

			if !ignored {
				var arcname string
				if b.saveRelative {
					arcname = relativeFilename
				} else {
					arcname = absPath
				}
				if err := addFileToZip(zw, absPath, arcname); err != nil {
					return err
				}
				if b.debug && b.showSuccess {
					textOutput += fmt.Sprintf("&a%s\n", relativeFilename)
					newlineCount++
				}
			} else {
				if b.debug && b.showIgnored && !strings.Contains(absPath, "node_modules") {
					textOutput += fmt.Sprintf("&c%s ignored\n", relativeFilename)
					newlineCount++
				}
			}

			if b.debug && newlineCount > 0 && newlineCount%25 == 0 {
				cosmetics.CPrint(textOutput)
				textOutput = ""
			}
			return nil
		})
		if err != nil {
			return err
		}
		if b.debug && textOutput != "" {
			cosmetics.CPrint(textOutput)
		}
	}

	if err := zw.Close(); err != nil {
		return err
	}
	if err := zipFile.Close(); err != nil {
		return err
	}

	if mongoCfg.Enabled {
		if len(b.mongodbAbsLocation) > 3 {
			if err := os.RemoveAll(b.mongodbAbsLocation); err != nil {
				return err
			}
		} else {
			fmt.Printf("Safety check hit, can't delete %s\n", b.mongodbAbsLocation)
		}
	}

	if len(b.discordWebhook) > 0 {
		b.sendZipCompleteNotification(mongoCfg)
	}

	return nil
}

func addFileToZip(zw *zip.Writer, srcPath, arcname string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(arcname)
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

func (b *Backup) sendZipCompleteNotification(mongoCfg config.MongoDB) {
	info, err := os.Stat(b.backupFileName)
	if err != nil {
		return
	}
	fileSizeMB := roundTo(float64(info.Size())/1024/1024, 4)

	size, used, free, storagePercent := system.GetStorageAmount()
	totalRam, usedRam, percentUsed := system.GetRamUsage()

	dirNames := make([]string, 0, len(b.rootPaths))
	for k := range b.rootPaths {
		dirNames = append(dirNames, k)
	}
	sort.Strings(dirNames)

	var percentUsedFloat float64
	fmt.Sscanf(percentUsed, "%f", &percentUsedFloat)

	values := map[string]notifications.Field{
		"Backup (MB)": {Text: fmt.Sprintf("%v", fileSizeMB), Inline: true},
		"Backup (GB)": {Text: fmt.Sprintf("%v", roundTo(fileSizeMB/1024, 4)), Inline: true},
		"To Hetzner":  {Text: fmt.Sprintf("%v", b.cfg.Backups.HetznerSFTP.Enabled), Inline: true},
		"MongoDB":     {Text: fmt.Sprintf("%v", mongoCfg.Enabled), Inline: true},
		"Directories": {Text: strings.Join(dirNames, "\n"), Inline: false},
		"Storage":     {Text: fmt.Sprintf("%s/%s (%s used) - Free: %s", used, size, storagePercent, free), Inline: false},
		"RAM":         {Text: fmt.Sprintf("%s/%s (%.1f%% used)", usedRam, totalRam, percentUsedFloat), Inline: false},
	}
	order := []string{"Backup (MB)", "Backup (GB)", "To Hetzner", "MongoDB", "Directories", "Storage", "RAM"}

	timePassed := int(time.Since(b.currentTime).Seconds())

	notifications.DiscordNotification(
		b.cfg.DiscordWebhookEnable,
		b.discordWebhook,
		fmt.Sprintf("Panel - Backup - %s", system.GetCurrentHostname()),
		fmt.Sprintf("Backup of %s | (%ds)", b.zipFilename, timePassed),
		"11ff44",
		values,
		order,
		"https://media.istockphoto.com/vectors/digital-signage-pixel-icon-tech-element-vector-logo-icon-illustrator-vector-id1164466990?k=20&m=1164466990&s=612x612&w=0&h=K5Zp0dtbjKWQS9CdOO53O09EKphYnxZTqDHppSMZ8Rk=",
		"",
	)
}

func roundTo(v float64, places int) float64 {
	shift := 1.0
	for i := 0; i < places; i++ {
		shift *= 10
	}
	return float64(int64(v*shift+0.5)) / shift
}

// SendFileToSFTPServer mirrors Backup.send_file_to_sftp_server.
func (b *Backup) SendFileToSFTPServer() error {
	hetzner := b.cfg.Backups.HetznerSFTP
	if !hetzner.Enabled {
		return nil
	}

	start := time.Now()

	err := func() error {
		sshCfg := &ssh.ClientConfig{
			User:            hetzner.Username,
			Auth:            []ssh.AuthMethod{ssh.Password(hetzner.Password)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), // matches pysftp cnopts.hostkeys = None
			Timeout:         30 * time.Second,
		}

		conn, err := ssh.Dial("tcp", hostWithPort(hetzner.ServerURL), sshCfg)
		if err != nil {
			return err
		}
		defer conn.Close()

		client, err := sftp.NewClient(conn)
		if err != nil {
			return err
		}
		defer client.Close()

		fmt.Println("Uploading backup to Hetzner SFTP server...")

		if _, err := client.Stat(hetzner.RemoteDir); err != nil {
			if err := client.MkdirAll(hetzner.RemoteDir); err != nil {
				return err
			}
		}

		remotePath := path.Join(hetzner.RemoteDir, filepath.Base(b.backupFileName))

		src, err := os.Open(b.backupFileName)
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := client.Create(remotePath)
		if err != nil {
			return err
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return err
		}

		fmt.Println("Upload to Hetzner finished!")
		return nil
	}()

	if err != nil {
		fmt.Println("Error sending backup to Hetzner SFTP server", err)
		notifications.DiscordNotification(
			b.cfg.DiscordWebhookEnable,
			b.discordWebhook,
			fmt.Sprintf("Panel - Upload failed - %s", system.GetCurrentHostname()),
			fmt.Sprintf("Upload failed -> Hetzner. Reason: %v", err),
			"ff0000",
			nil, nil,
			"https://cdn.icon-icons.com/icons2/2407/PNG/512/hetzner_icon_146165.png",
			"",
		)
		return nil
	}

	notifications.DiscordNotification(
		b.cfg.DiscordWebhookEnable,
		b.discordWebhook,
		fmt.Sprintf("Panel - Upload - %s", system.GetCurrentHostname()),
		fmt.Sprintf("Completed upload -> Hetzner in %.2f seconds", time.Since(start).Seconds()),
		"00ff00",
		nil, nil,
		"https://cdn.icon-icons.com/icons2/2407/PNG/512/hetzner_icon_146165.png",
		"",
	)
	return nil
}

func hostWithPort(host string) string {
	if strings.Contains(host, ":") {
		return host
	}
	return host + ":22"
}
