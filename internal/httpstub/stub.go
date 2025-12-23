package httpstub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func loadStubs(dir string, storage *Storage) error {
	if err := filepath.WalkDir(dir, walk(dir, storage)); err != nil {
		return fmt.Errorf("read stubs from dir %v: %w", dir, err)
	}
	return nil
}

func walk(root string, storage *Storage) fs.WalkDirFunc {
	return func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			var stub Stub
			var err error

			switch filepath.Ext(path) {

			case ".json":
				stub, err = loadJSONFile(root, path)
			case ".http":
				stub, err = loadHTTPFile(root, path)
			// pass
			default:
				return nil
			}

			if err != nil {
				return fmt.Errorf("load stub from %v: %w", path, err)
			}
			storage.Add(stub)

		}
		return nil
	}
}

func loadJSONFile(_ string, path string) (s Stub, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %v: %w", path, err)
	}
	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close file: %w", closeErr))
		}
	}()

	var stub JSONStub
	if err := json.NewDecoder(f).Decode(&stub); err != nil {
		return nil, fmt.Errorf("unmarshal stub %v: %w", path, err)
	}

	if err = stub.Validate(); err != nil {
		return nil, fmt.Errorf("stub validation %v: %w", path, err)
	}
	return stub, nil
}
