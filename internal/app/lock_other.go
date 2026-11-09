//go:build !linux

package app

import (
	"errors"
	"os"
)

func lockJournal(string) (*os.File, error) {
	return nil, errors.New("live journal monitoring requires Linux; use -replay with a journal JSON export")
}
