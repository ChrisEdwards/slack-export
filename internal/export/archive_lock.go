package export

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type archiveLock struct {
	file *os.File
}

func acquireArchiveLock(archiveDir string) (*archiveLock, bool, error) {
	lockPath := archiveDir + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0750); err != nil {
		return nil, false, fmt.Errorf("creating lock directory: %w", err)
	}
	fd, err := syscall.Open(lockPath, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, false, fmt.Errorf("opening archive lock: %w", err)
	}
	file := os.NewFile(uintptr(fd), lockPath)
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("locking archive: %w", err)
	}
	return &archiveLock{file: file}, true, nil
}

func (l *archiveLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	var unlockErr error
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN); err != nil {
		unlockErr = fmt.Errorf("unlocking archive: %w", err)
	}
	if err := file.Close(); err != nil && unlockErr == nil {
		unlockErr = fmt.Errorf("closing archive lock: %w", err)
	}
	return unlockErr
}
