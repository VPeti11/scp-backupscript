package main

import (
    "context"
    "fmt"
    "io/fs"
    "log"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/bramvdbogaerde/go-scp"
    "github.com/fsnotify/fsnotify"
    "golang.org/x/crypto/ssh"
)

var (
    remoteIP    = "serverip"
    remotePort  = "22"
    sshUser     = "backupscp"
    remoteRoot  = "/opt/backup/"
    localRoot   = `F:\tobackup\`
    privateKey  = `-----BEGIN OPENSSH PRIVATE KEY-----
    aaaaaaa
    -----END OPENSSH PRIVATE KEY-----`

    SyncDeletions = true
    DeletedFiles  = make(chan string, 100)
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

    fmt.Printf("Monitoring %s for changes...\n", targetDir)

    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                return
            }

            if event.Has(fsnotify.Remove) {
                if SyncDeletions {
                    DeletedFiles <- event.Name
                }
            }

            if event.Has(fsnotify.Create) {
                info, err := os.Stat(event.Name)
                if err == nil && info.IsDir() {
                    watcher.Add(event.Name)
                    continue
                }
            }

            if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
                info, err := os.Stat(event.Name)
                if err == nil && !info.IsDir() {
                    uploadFileWithRetry(event.Name, targetDir)
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

func uploadFileWithRetry(localPath, baseDir string) {
    for attempt := 1; attempt <= 10; attempt++ {
        err := uploadFile(localPath, baseDir)
        if err == nil {
            return
        }
        time.Sleep(time.Duration(attempt) * time.Second)
    }
}

func uploadFile(localPath, baseDir string) error {
    cleanKey := strings.TrimSpace(privateKey)
    lines := strings.Split(cleanKey, "\n")
    for i, line := range lines {
        lines[i] = strings.TrimSpace(line)
    }
    finalKey := strings.Join(lines, "\n")

    signer, err := ssh.ParsePrivateKey([]byte(finalKey))
    if err != nil {
        return err
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
        return err
    }
    defer client.Close()

    relPath, _ := filepath.Rel(baseDir, localPath)
    remoteDest := filepath.ToSlash(filepath.Join(remoteRoot, relPath))
    remoteDir := filepath.ToSlash(filepath.Dir(remoteDest))

    err = remoteMkdir(client.SSHClient(), remoteDir)
    if err != nil {
        return err
    }

    f, err := os.Open(localPath)
    if err != nil {
        return err
    }
    defer f.Close()

    err = client.CopyFromFile(context.Background(), *f, remoteDest, "0644")
    if err != nil {
        return err
    }

    return nil
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
