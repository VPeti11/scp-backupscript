# SCP-backupscript

A simple cross-platform file watcher that automatically uploads changed files to a remote server via SCP over SSH.

It watches a local directory recursively and pushes new or modified files to a remote Linux path.

---

## What It Does

* Recursively watches a local folder
* Detects:

  * File creation
  * File modification
  * New directories (adds them to watcher automatically)
* Mirrors the relative folder structure remotely
* Creates remote directories automatically (`mkdir -p`)
* Uploads files via SCP using an embedded private SSH key

---

## Dependencies

```
go get github.com/bramvdbogaerde/go-scp
go get github.com/fsnotify/fsnotify
go get golang.org/x/crypto/ssh
```

---

## Configuration

Edit these variables in `main.go`:

```
remoteIP   = "serverip"
remotePort = "22"
sshUser    = "backupscp"
remoteRoot = "/opt/backup/"
localRoot  = `F:\tobackup\`
privateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
...
-----END OPENSSH PRIVATE KEY-----`
```

### Required:

* SSH key must match authorized_keys on remote server
* Remote user must have write access to `remoteRoot`

---

## Run It

```
go run main.go
```

You’ll see:

```
Monitoring F:\tobackup\ for changes...
```

When files change:

```
[FILE] WRITE: F:\tobackup\file.txt
Successfully uploaded -> /opt/backup/file.txt
```

---

## How It Works

1. Walks the entire local directory
2. Adds every folder to fsnotify watcher
3. Listens forever for events
4. On file change:

   * Builds relative path
   * Creates remote directory if needed
   * Uploads via SCP

---

## Example

Local:

```
F:\tobackup\projects\app\config.json
```

Remote:

```
/opt/backup/projects/app/config.json
```

---

## ⚠Warnings

* Uses `ssh.InsecureIgnoreHostKey()` (NOT secure for production)
* SSH private key is hardcoded (bad practice)
* No retry logic
* No deletion sync
* No batching
* Opens new SSH connection per file change

This is intentionally quick & dirty.

---

## Use Case

* Lightweight backup
* Dev folder mirroring
* Poor man's deployment tool
* Windows → Linux sync
* Lab environments

---
