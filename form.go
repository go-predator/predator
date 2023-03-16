/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   form.go
 * @Created At:  2023-02-20 20:34:40
 * @Modified At: 2023-03-16 10:01:51
 * @Modified By: thepoy
 */

package predator

import (
	"bytes"
	"io"
	"mime/multipart"
	"os"
	"path"
	"sync"
)

// A sync.Pool that holds MultipartFormWriter instances for reuse.
var (
	multipartFormWriterPool = &sync.Pool{
		New: func() any {
			return NewMultipartFormWriter()
		},
	}
)

// AcquireMultipartFormWriter returns a new MultipartFormWriter from the pool.
func AcquireMultipartFormWriter() *MultipartFormWriter {
	return multipartFormWriterPool.Get().(*MultipartFormWriter)
}

// ReleaseMultipartFormWriter resets and returns an MultipartFormWriter instance to the pool.
func ReleaseMultipartFormWriter(mfw *MultipartFormWriter) {
	mfw.Reset()

	multipartFormWriterPool.Put(mfw)
}

// MultipartFormWriter provides an interface to generate multipart forms.
type MultipartFormWriter struct {
	sync.Mutex

	// Buffer used to write the form to.
	buf *bytes.Buffer

	// Writer for the multipart form.
	w *multipart.Writer

	// A map for caching multipart fields to prevent unnecessary duplications.
	cachedMap map[string]string
}

// NewMultipartFormWriter returns a new MultipartFormWriter.
func NewMultipartFormWriter() *MultipartFormWriter {
	form := new(MultipartFormWriter)

	form.buf = new(bytes.Buffer)
	form.w = multipart.NewWriter(form.buf)
	form.cachedMap = make(map[string]string)

	return form
}

// AddValue adds a key-value pair to the form.
func (mfw *MultipartFormWriter) AddValue(fieldname, value string) {
	mfw.Lock()
	defer mfw.Unlock()

	mfw.w.WriteField(fieldname, value)
	mfw.cachedMap[fieldname] = value
}

// Reset resets the multipart form writer to its initial state.
func (mfw *MultipartFormWriter) Reset() {
	mfw.buf.Reset()
	ResetMap(mfw.cachedMap)
	mfw.w = nil
}

// AppendString is an alias for AddValue.
func (mfw *MultipartFormWriter) AppendString(fieldname, value string) {
	mfw.AddValue(fieldname, value)
}

// AddFile adds a file to the form.
func (mfw *MultipartFormWriter) AddFile(fieldname, filename, path string) {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	mfw.Lock()
	mfw.cachedMap[fieldname] = filename
	w, err := mfw.w.CreateFormFile(fieldname, filename)
	if err != nil {
		panic(err)
	}
	mfw.Unlock()

	_, err = io.Copy(w, f)
	if err != nil {
		panic(err)
	}
}

// NewMultipartForm finalizes the form and returns the content type and buffer.
func NewMultipartForm(mfw *MultipartFormWriter) (string, *bytes.Buffer) {
	defer mfw.w.Close()

	return mfw.w.FormDataContentType(), mfw.buf
}

// AppendFile adds a file to the form using only the filepath.
func (mfw *MultipartFormWriter) AppendFile(fieldname, filepath string) {
	filename := path.Base(filepath)

	mfw.AddFile(fieldname, filename, filepath)
}
