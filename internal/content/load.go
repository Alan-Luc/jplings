package content

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

func Load(paths ...string) ([]*Unit, []Issue, error) {
	files, err := collect(paths)
	if err != nil {
		return nil, nil, err
	}

	var (
		units  []*Unit
		issues []Issue
	)
	seen := map[string]string{}
	for _, file := range files {
		unit, err := decode(file)
		if err != nil {
			issues = append(issues, Issue{Path: file, Msg: err.Error()})
			continue
		}
		if prev, dup := seen[unit.ID]; dup && unit.ID != "" {
			issues = append(issues, Issue{Path: file, Msg: fmt.Sprintf("duplicate unit id %q, already in %s", unit.ID, prev)})
		} else if unit.ID != "" {
			seen[unit.ID] = file
		}
		issues = append(issues, unit.Validate()...)
		units = append(units, unit)
	}
	return units, issues, nil
}

func decode(file string) (*Unit, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	var unit Unit
	if err := dec.Decode(&unit); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("empty file")
		}
		return nil, err
	}
	return &unit, nil
}

func collect(paths []string) ([]string, error) {
	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			switch filepath.Ext(path) {
			case ".yaml", ".yml":
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}
