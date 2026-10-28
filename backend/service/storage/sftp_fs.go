package storage

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/sftp"
)

type rootedSFTPFS struct {
	root     *os.Root
	readOnly bool
}

type sftpFileList []os.FileInfo

func (l sftpFileList) ListAt(dst []os.FileInfo, offset int64) (int, error) {
	if offset < 0 || offset >= int64(len(l)) {
		return 0, io.EOF
	}
	n := copy(dst, l[int(offset):])
	if n == 0 || int(offset)+n >= len(l) {
		return n, io.EOF
	}
	return n, nil
}

func (fs *rootedSFTPFS) resolve(clientPath string) (string, error) {
	if strings.ContainsRune(clientPath, '\x00') {
		return "", errors.New("invalid path")
	}
	cleaned := path.Clean("/" + strings.ReplaceAll(clientPath, "\\", "/"))
	if cleaned == "/" {
		return ".", nil
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}

func (fs *rootedSFTPFS) Fileread(req *sftp.Request) (io.ReaderAt, error) {
	name, err := fs.resolve(req.Filepath)
	if err != nil {
		return nil, err
	}
	return fs.root.Open(name)
}

func (fs *rootedSFTPFS) Filewrite(req *sftp.Request) (io.WriterAt, error) {
	if fs.readOnly {
		return nil, os.ErrPermission
	}
	flags := req.Pflags()
	if !flags.Write || flags.Append {
		return nil, os.ErrPermission
	}
	name, err := fs.resolve(req.Filepath)
	if err != nil {
		return nil, err
	}
	openFlags := os.O_WRONLY
	if flags.Creat {
		openFlags |= os.O_CREATE
	}
	if flags.Trunc {
		openFlags |= os.O_TRUNC
	}
	if flags.Excl {
		openFlags |= os.O_EXCL
	}
	return fs.root.OpenFile(name, openFlags, 0600)
}

func (fs *rootedSFTPFS) OpenFile(req *sftp.Request) (sftp.WriterAtReaderAt, error) {
	flags := req.Pflags()
	if fs.readOnly && (flags.Write || flags.Creat || flags.Trunc || flags.Append) {
		return nil, os.ErrPermission
	}
	if flags.Append {
		return nil, os.ErrPermission
	}
	name, err := fs.resolve(req.Filepath)
	if err != nil {
		return nil, err
	}
	openFlags := os.O_RDONLY
	if flags.Write && flags.Read {
		openFlags = os.O_RDWR
	} else if flags.Write {
		openFlags = os.O_WRONLY
	}
	if flags.Creat {
		openFlags |= os.O_CREATE
	}
	if flags.Trunc {
		openFlags |= os.O_TRUNC
	}
	if flags.Excl {
		openFlags |= os.O_EXCL
	}
	return fs.root.OpenFile(name, openFlags, 0600)
}

func (fs *rootedSFTPFS) Filelist(req *sftp.Request) (sftp.ListerAt, error) {
	if req.Method == "Readlink" {
		return nil, os.ErrPermission
	}
	name, err := fs.resolve(req.Filepath)
	if err != nil {
		return nil, err
	}
	if req.Method == "Stat" {
		info, err := fs.root.Stat(name)
		if err != nil {
			return nil, err
		}
		return sftpFileList{info}, nil
	}
	if req.Method != "List" {
		return nil, os.ErrPermission
	}
	dir, err := fs.root.Open(name)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	infos := make(sftpFileList, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (fs *rootedSFTPFS) Filecmd(req *sftp.Request) error {
	if fs.readOnly {
		return os.ErrPermission
	}
	name, err := fs.resolve(req.Filepath)
	if err != nil {
		return err
	}
	switch req.Method {
	case "Mkdir":
		return fs.root.Mkdir(name, 0700)
	case "Remove", "Rmdir":
		return fs.root.Remove(name)
	case "Rename", "PosixRename":
		target, err := fs.resolve(req.Target)
		if err != nil {
			return err
		}
		return fs.root.Rename(name, target)
	case "Setstat":
		attrs := req.Attributes()
		if attrs == nil {
			return errors.New("invalid file attributes")
		}
		flags := req.AttrFlags()
		if flags.UidGid {
			return os.ErrPermission
		}
		if flags.Size {
			if attrs.Size > uint64(1<<63-1) {
				return errors.New("file size is out of range")
			}
			file, err := fs.root.OpenFile(name, os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			err = file.Truncate(int64(attrs.Size))
			closeErr := file.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
		if flags.Permissions {
			if err := fs.root.Chmod(name, os.FileMode(attrs.Mode)&0777); err != nil {
				return err
			}
		}
		if flags.Acmodtime {
			return fs.root.Chtimes(name, time.Unix(int64(attrs.Atime), 0), time.Unix(int64(attrs.Mtime), 0))
		}
		return nil
	case "Symlink", "Link":
		return os.ErrPermission
	default:
		return os.ErrPermission
	}
}
