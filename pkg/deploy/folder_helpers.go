package deploy

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

type fileProcessor func(path string, src string) string

func listAllFilesInFolder(srcFs afero.Fs, folder string) ([]string, error) {
	files := make([]string, 0)

	err := afero.Walk(srcFs, folder, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !f.IsDir() {
			// Append to Files Array
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func copyAndProcessFolder(srcFs afero.Fs, srcFolder string, destFs afero.Fs, destFolder string, processor fileProcessor) error {
	err := afero.Walk(srcFs, srcFolder, func(src string, srcFileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcFolder, src)
		if err != nil {
			return err
		}
		dest := filepath.Join(destFolder, relPath)

		if srcFileInfo.IsDir() {
			err = destFs.MkdirAll(dest, srcFileInfo.Mode())
			if err != nil {
				return err
			}
		} else {
			// Append to Files Array
			f, err := destFs.Create(dest)
			if err != nil {
				return err
			}

			if err = destFs.Chmod(f.Name(), srcFileInfo.Mode()); err != nil {
				return err
			}

			s, err := afero.ReadFile(srcFs, src)
			if err != nil {
				return err
			}

			processedSrc := processor(f.Name(), string(s))
			if processedSrc == "" {
				return nil
			}

			_, err = io.Copy(f, strings.NewReader(processedSrc))
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func expandPath(path string) string {
	home := os.Getenv("HOME")

	if path == "~" {
		// In case of "~", which won't be caught by the "else if"
		path = home
	} else if strings.HasPrefix(path, "~/") {
		// Use strings.HasPrefix so we don't match paths like
		// "/something/~/something/"
		path = filepath.Join(home, path[2:])
	}

	return os.ExpandEnv(path)
}
