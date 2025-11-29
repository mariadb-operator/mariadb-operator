package filemanager

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	writeFileMode = fs.FileMode(0777)
)

type FileManager struct {
	configDir string
	stateDir  string
}

func NewFileManager(configDir, stateDir string) (*FileManager, error) {
	if _, err := os.Stat(configDir); err != nil {
		return nil, fmt.Errorf("error reading config directory: %v", err)
	}
	if _, err := os.Stat(stateDir); err != nil {
		return nil, fmt.Errorf("error reading state directory: %v", err)
	}
	return &FileManager{
		configDir: configDir,
		stateDir:  stateDir,
	}, nil
}

func (f *FileManager) GetStateDir() string {
	return f.stateDir
}

func (f *FileManager) StateFilePath(name string) string {
	return filepath.Join(f.stateDir, name)
}

func (f *FileManager) WriteStateFile(name string, bytes []byte) error {
	return os.WriteFile(f.StateFilePath(name), bytes, writeFileMode)
}

func (f *FileManager) ReadStateFile(name string) ([]byte, error) {
	return os.ReadFile(f.StateFilePath(name))
}

func (f *FileManager) DeleteStateFile(name string) error {
	return os.Remove(f.StateFilePath(name))
}

func (f *FileManager) StateFileExists(name string) (bool, error) {
	if _, err := os.Stat(f.StateFilePath(name)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (f *FileManager) GetConfigDir() string {
	return f.configDir
}

func (f *FileManager) ConfigFilePath(name string) string {
	return filepath.Join(f.configDir, name)
}

func (f *FileManager) WriteConfigFile(name string, bytes []byte) error {
	return os.WriteFile(f.ConfigFilePath(name), bytes, writeFileMode)
}

func (f *FileManager) ReadConfigFile(name string) ([]byte, error) {
	return os.ReadFile(f.ConfigFilePath(name))
}

func (f *FileManager) DeleteConfigFile(name string) error {
	return os.Remove(f.ConfigFilePath(name))
}

func (f *FileManager) ConfigFileExists(name string) (bool, error) {
	if _, err := os.Stat(f.ConfigFilePath(name)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
