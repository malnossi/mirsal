package wormhole

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipDirectory compresses a directory recursively into a zip file at destPath.
func ZipDirectory(sourceDir, destPath string) error {
	zipFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	info, err := os.Stat(sourceDir)
	if err != nil {
		return err
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(sourceDir)
	}

	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		if baseDir != "" {
			// Maintain the root folder structure inside zip
			relPath, err := filepath.Rel(sourceDir, path)
			if err != nil {
				return err
			}
			if relPath == "." {
				return nil
			}
			header.Name = filepath.ToSlash(filepath.Join(baseDir, relPath))
		} else {
			header.Name = filepath.ToSlash(filepath.Base(path))
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	return err
}

// UnzipDirectory extracts a zip archive to the targetDir directory.
func UnzipDirectory(zipPath, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Clean target directory path to prevent path traversal
	cleanedTarget := filepath.Clean(targetDir)

	for _, file := range reader.File {
		// Secure path resolution to prevent Zip Slip vulnerability
		filePath := filepath.Join(cleanedTarget, file.Name)
		if !strings.HasPrefix(filePath, cleanedTarget+string(os.PathSeparator)) && filePath != cleanedTarget {
			continue // Skip files trying to escape target dir
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		srcFile, err := file.Open()
		if err != nil {
			dstFile.Close()
			return err
		}

		_, err = io.Copy(dstFile, srcFile)
		dstFile.Close()
		srcFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
