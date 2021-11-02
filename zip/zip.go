package zip

import (
	"bytes"
	"compress/gzip"
	"github.com/blinkops/blink-openapi-sdk/consts"
	"github.com/getkin/kin-openapi/openapi3"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	fp "path/filepath"
)

// UnzipFile unzips the file and saves
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

// UnzipData gets a byte slice and returns an unzipped slice
func UnzipData(data []byte) (resData []byte, err error) {
	// create new buffer from the data slice.
	dataBuffer := bytes.NewBuffer(data)

	var gzipReader io.Reader
	// create a gzip reader to read from the data buffer
	gzipReader, err = gzip.NewReader(dataBuffer)
	if err != nil {
		return
	}

	var resB bytes.Buffer
	// read the data from the reader into a new buffer
	_, err = resB.ReadFrom(gzipReader)
	if err != nil {
		return
	}

	// turn the buffer into byte slice
	resData = resB.Bytes()

	return
}

// ReadGzipFile reads and unzips the given file.
func ReadGzipFile(filePath string) ([]byte, error) {
	// read data from file path
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// decompress the data
	uncompressedData, err := UnzipData(content)
	if err != nil {
		return nil, err
	}

	return uncompressedData, nil
}

// LoadFromGzipFile is used as replacement for loader.LoadFromFile, but for reading gzipped files.
func LoadFromGzipFile(loader *openapi3.Loader, filePath string) (*openapi3.T, error) {
	data, err := ReadGzipFile(filePath)
	if err != nil {
		return nil, err
	}

	return loader.LoadFromDataWithPath(data, &url.URL{Path: fp.ToSlash(filePath)})
}
