package rewrite

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
)

type RewriteResult struct {
	Changed   bool
	FileCount int
}

type FixFunc func(ctx context.Context, content string) (string, bool, error)

func Rewrite(ctx context.Context, filePaths []string, ignoreDirs []string, f FixFunc) (RewriteResult, error) {
	if len(filePaths) == 0 {
		slog.Debug("searching for workflow files to process")
		workflowPaths, err := findWorkflowFiles(".", ignoreDirs)
		if err != nil {
			return RewriteResult{}, err
		}
		slog.Debug("found workflow files", "count", len(workflowPaths))
		if len(workflowPaths) == 0 {
			return RewriteResult{}, nil
		}

		filePaths = workflowPaths
	}

	res := RewriteResult{}
	var errs []error

	for _, filePath := range filePaths {
		slog.Debug("processing file", "path", filePath)
		changed, err := processFile(ctx, filePath, f)
		if err != nil {
			// Collect the error but continue processing remaining files.
			errs = append(errs, errors.Wrapf(err, "failed to process file: %s", filePath))
			continue
		}

		if changed {
			slog.Info("file updated", "path", filePath)
			res.Changed = true
			res.FileCount++
		}
	}

	if len(errs) > 0 {
		return res, errors.Join(errs...)
	}

	return res, nil
}

func processFile(ctx context.Context, filePath string, f FixFunc) (bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, errors.WithStack(err)
	}

	modifiedContent, changed, err := f(ctx, string(content))
	if err != nil {
		return false, errors.Wrapf(err, "failed to replace actions in file: %s", filePath)
	}
	if !changed {
		return false, nil
	}

	err = writeFileAtomic(filePath, modifiedContent)
	if err != nil {
		return false, errors.Wrapf(err, "failed to write file: %s", filePath)
	}

	return true, nil
}

// findWorkflowFiles finds all workflow files (.yml or .yaml) in the current directory and subdirectories
// ignoreDirs is an optional list of directory names to skip during traversal
func findWorkflowFiles(root string, ignoreDirs []string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			dirName := info.Name()

			// Skip directories specified in ignoreDirs (defaults to .git and node_modules)
			for _, ignoreDir := range ignoreDirs {
				if dirName == ignoreDir {
					slog.Debug("skipping directory", "path", path, "name", dirName)
					return filepath.SkipDir
				}
			}
		}

		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".yml" || ext == ".yaml" {
				files = append(files, path)
			}
		}

		return nil
	})

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return files, nil
}

func writeFileAtomic(targetPath, content string) error {
	dir := filepath.Dir(targetPath)
	fileName := filepath.Base(targetPath)
	ext := filepath.Ext(fileName)
	nameWithoutExt := strings.TrimSuffix(fileName, ext)

	// <name>-<random string>.<extension>
	pattern := nameWithoutExt + "-*" + ext

	tmpFile, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return errors.WithStack(err)
	}
	tmpPath := tmpFile.Name()

	// Ensure temp file is removed if anything goes wrong
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.WriteString(content); err != nil {
		return errors.WithStack(err)
	}
	if err := tmpFile.Sync(); err != nil {
		return errors.WithStack(err)
	}
	if err := tmpFile.Close(); err != nil {
		return errors.WithStack(err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
