package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/crypto/ssh"
)

var (
	remoteIP   = "serverip"
	remotePort = "22"
	sshUser    = "backupscp"
	remoteRoot = "/opt/backup/"
	localRoot = `F:\tobackup\`
	privateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
	aaaaaaa
	-----END OPENSSH PRIVATE KEY-----`
)


func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()


	targetDir := filepath.FromSlash(localRoot)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		os.MkdirAll(targetDir, 0755)
	}


	err = filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		log.Fatal("Error walking path:", err)
	}

	fmt.Printf("Monitoring %s for changes...)\n", targetDir)

	for {
		select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}


				if event.Has(fsnotify.Create) {
					info, err := os.Stat(event.Name)
					if err == nil && info.IsDir() {
						watcher.Add(event.Name)
						log.Printf("[DIR] Now watching: %s", event.Name)
						continue
					}
				}


				if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
					info, err := os.Stat(event.Name)
					if err == nil && !info.IsDir() {
						log.Printf("[FILE] %v: %s", event.Op, event.Name)
						uploadFile(event.Name, targetDir)
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("Watcher error:", err)
		}
	}
}

func uploadFile(localPath, baseDir string) {

	cleanKey := strings.TrimSpace(privateKey)
	lines := strings.Split(cleanKey, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	finalKey := strings.Join(lines, "\n")

	signer, err := ssh.ParsePrivateKey([]byte(finalKey))
	if err != nil {
		log.Printf("SSH Key Error: %v", err)
		return
	}

	sshConfig := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%s", remoteIP, remotePort)
	client := scp.NewClient(addr, sshConfig)

	err = client.Connect()
	if err != nil {
		log.Printf("SSH Connection failed: %v", err)
		return
	}
	defer client.Close()


	relPath, _ := filepath.Rel(baseDir, localPath)
	remoteDest := filepath.ToSlash(filepath.Join(remoteRoot, relPath))
	remoteDir := filepath.ToSlash(filepath.Dir(remoteDest))


	err = remoteMkdir(client.SSHClient(), remoteDir)
	if err != nil {
		log.Printf("Remote mkdir failed: %v", err)
		return
	}

	f, err := os.Open(localPath)
	if err != nil {
		log.Printf("Local file error: %v", err)
		return
	}
	defer f.Close()

	err = client.CopyFromFile(context.Background(), *f, remoteDest, "0644")
	if err != nil {
		log.Printf("SCP Transfer error: %v", err)
	} else {
		log.Printf("Successfully uploaded -> %s", remoteDest)
	}
}

func remoteMkdir(sshClient *ssh.Client, path string) error {
	session, err := sshClient.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()


	escapedPath := strings.ReplaceAll(path, "'", "\\'")
	return session.Run(fmt.Sprintf("mkdir -p '%s'", escapedPath))
}
