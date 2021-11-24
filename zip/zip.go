package zip

import (
	"bytes"
	"compress/gzip"
	"net/url"
	"os"
	fp "path/filepath"

	"github.com/blinkops/blink-openapi-sdk/consts"
	"github.com/getkin/kin-openapi/openapi3"
)

// UnzipFile unzips the file and saves to disk.
func UnzipFile(filePath string) (err error) {
	// the file path plus the file ending
	gzipFile := filePath + consts.GzipFile

	// read the data from the file
	data, err := ReadGzipFile(gzipFile)
	if err != nil {
		return
	}

	// write the decompressed data to a new file.
	if err = os.WriteFile(filePath, data, 0644); err != nil {
		return
	}

	// clean up the unused gzipped file.
	if err = os.Remove(gzipFile); err != nil {
		return
	}

	return
}

// ReadGzipFile returns the unzipped content of the given file.
func ReadGzipFile(filePath string) ([]byte, error) {
	// os.open creates a io.reader
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	r, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}

	defer r.Close()

	var resB bytes.Buffer
	_, err = resB.ReadFrom(r)
	if err != nil {
		return nil, err
	}

	return resB.Bytes(), nil
}

// LoadFromGzipFile is used as replacement for loader.LoadFromFile, but for reading gzipped files.
func LoadFromGzipFile(loader *openapi3.Loader, filePath string) (*openapi3.T, error) {
	data, err := ReadGzipFile(filePath)
	if err != nil {
		return nil, err
	}

	return loader.LoadFromDataWithPath(data, &url.URL{Path: fp.ToSlash(filePath)})
}
