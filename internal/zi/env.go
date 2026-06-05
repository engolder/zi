package zi

import (
	"os"
	"path/filepath"
)

type Env struct {
	Home string
	Cwd  string
}

func NewEnv() (Env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Env{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return Env{}, err
	}
	return Env{Home: home, Cwd: cwd}, nil
}

func (e Env) ConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "zi", "config.json")
	}
	return filepath.Join(e.Home, ".config", "zi", "config.json")
}
