package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func extract(r io.Reader, targetFolder string) error {
	gzf, err := gzip.NewReader(r)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(gzf)
	i := 0

	if _, err = os.Stat(targetFolder); err != nil {
		if err := os.MkdirAll(targetFolder, 0755); err != nil {
			return err
		}
	}

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		name := header.Name

		// The files in the archive are all in a parent folder,
		// we want to extract all files directly to TargetFolder
		namePath := strings.Split(name, "/")
		switch len(namePath) {
		case 0:
			break
		case 1:
			name = "/"
		default:
			name = strings.Join(namePath[1:], "/")
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(path.Join(targetFolder, name), 0755); err != nil {
				return fmt.Errorf("Error creating %s: %v", path.Join(targetFolder, name), err)
			}
			continue

		case tar.TypeReg:
			var data bytes.Buffer
			io.Copy(&data, tarReader)
			if err := ioutil.WriteFile(path.Join(targetFolder, name), data.Bytes(), 0644); err != nil {
				return fmt.Errorf("Error creating %s: %v", path.Join(targetFolder, name), err)
			}

		case tar.TypeXGlobalHeader:
			continue

		default:
			return fmt.Errorf("Error extracting Tar file: %v", err)
		}

		i++
	}

	return nil
}
