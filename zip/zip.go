package zip

import (
	"bytes"
	"compress/gzip"
	"github.com/getkin/kin-openapi/openapi3"
	"io"
	"io/ioutil"
)

func UnZipData(data []byte) (resData []byte, err error) {
	b := bytes.NewBuffer(data)

	var r io.Reader
	r, err = gzip.NewReader(b)
	if err != nil {
		return
	}

	var resB bytes.Buffer
	_, err = resB.ReadFrom(r)
	if err != nil {
		return
	}

	resData = resB.Bytes()

	return
}

func LoadFromGzipFile(loader *openapi3.Loader, filePath string) (*openapi3.T, error) {

	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// decompress data
	uncompressedData, uncompressedDataErr := UnZipData(content)
	if uncompressedDataErr != nil {
		return nil, uncompressedDataErr
	}

	return loader.LoadFromData(uncompressedData)

}