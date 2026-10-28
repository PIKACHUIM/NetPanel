package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
)

func TestRootedSFTPFSRejectsSymlinkEscape(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()
	secretPath := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secretPath, filepath.Join(rootDir, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	fs := &rootedSFTPFS{root: root}
	file, err := fs.Fileread(&sftp.Request{Method: "Get", Filepath: "/escape"})
	if err == nil {
		fileCloser, ok := file.(interface{ Close() error })
		if ok {
			fileCloser.Close()
		}
		t.Fatal("SFTP read followed a symlink outside the configured root")
	}
}

func TestRootedSFTPFSReadOnlyRejectsMutation(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	fs := &rootedSFTPFS{root: root, readOnly: true}
	if _, err := fs.Filewrite(&sftp.Request{Method: "Put", Filepath: "new.txt"}); err == nil {
		t.Fatal("read-only SFTP allowed a write request")
	}
	if err := fs.Filecmd(&sftp.Request{Method: "Mkdir", Filepath: "new-dir"}); err == nil {
		t.Fatal("read-only SFTP allowed a file command")
	}
}

func TestRootedSFTPFSMapsAbsolutePathsIntoShare(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	fs := &rootedSFTPFS{root: root}
	name, err := fs.resolve("/etc/passwd")
	if err != nil {
		t.Fatalf("resolve absolute client path: %v", err)
	}
	if name != "etc/passwd" {
		t.Fatalf("absolute client path mapped to %q, want path under the share", name)
	}
}
