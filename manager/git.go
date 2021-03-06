package manager

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"io/ioutil"
	"path/filepath"
)

const (
	VERSION_FILE = "VERSION"
	AUTHORS_FILE = "AUTHORS"
)

func WriteVersionFile(workdir string, version string) error {

	data := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<VERSION>%s</VERSION>`, version)

	filename := filepath.Join(workdir, VERSION_FILE)
	err := ioutil.WriteFile(filename, []byte(data), 0644)

	return err

}

func CommitVersionFile(workdir string) error {

	r, err := git.PlainOpen(workdir)

	if err != nil {
		return err
	}

	filename := filepath.Join(workdir, VERSION_FILE)

	w, err := r.Worktree()

	if err != nil {
		return err
	}

	w.Add(filename)
	c, err := w.Commit("Изменена версия в файле VERSION", &git.CommitOptions{})

	if err != nil {
		return err
	}

	_, err = r.CommitObject(c)

	return err
}
