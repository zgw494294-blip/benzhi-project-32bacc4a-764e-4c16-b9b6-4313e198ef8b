package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func calculateChecksum(event eventRecord) (string, error) {
	event.Checksum = ""
	data, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func verifyChecksum(event eventRecord) error {
	want := event.Checksum
	got, err := calculateChecksum(event)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("事件 %d 校验和不匹配", event.Sequence)
	}
	return nil
}

func appendEvent(file *os.File, event eventRecord) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("编码事件: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("追加事件: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("同步事件日志: %w", err)
	}
	return nil
}

func writeSnapshot(path string, value snapshot) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("编码快照: %w", err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("写入临时快照: %w", err)
	}
	file, err := os.OpenFile(temporary, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("打开临时快照: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("同步临时快照: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("关闭临时快照: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("原子替换快照: %w", err)
	}
	directory, err := os.Open(filepath.Dir(path))
	if err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
