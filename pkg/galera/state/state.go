package state

import (
	"fmt"
	"os"
	"path"
)

type State struct {
	stateDir string
}

func NewState(stateDir string) *State {
	return &State{
		stateDir: stateDir,
	}
}

func (i *State) IsGaleraInit() (bool, error) {
	entries, err := os.ReadDir(i.stateDir)
	if err != nil {
		return false, fmt.Errorf("error reading state directory: %v", err)
	}
	if len(entries) > 0 {
		info, err := os.Stat(path.Join(i.stateDir, "grastate.dat"))
		return !os.IsNotExist(err) && info.Size() > 0, nil
	}
	return false, nil
}
