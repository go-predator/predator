/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   form.go
 * @Created At:  2023-02-20 20:34:40
 * @Modified At: 2023-02-27 11:31:04
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

var (
	multipartFormWriterPool = &sync.Pool{
		New: func() any {
			return NewMultipartFormWriter()
		},
	}
)

func AcquireMultipartFormWriter() *MultipartFormWriter {
	return multipartFormWriterPool.Get().(*MultipartFormWriter)
}

func ReleaseMultipartFormWriter(mfw *MultipartFormWriter) {
	mfw.Reset()

	multipartFormWriterPool.Put(mfw)
}

type MultipartFormWriter struct {
	sync.Mutex

	buf *bytes.Buffer
	w   *multipart.Writer

	cachedMap map[string]string
}

func NewMultipartFormWriter() *MultipartFormWriter {
	form := new(MultipartFormWriter)

	form.buf = new(bytes.Buffer)
	form.w = multipart.NewWriter(form.buf)
	form.cachedMap = make(map[string]string)

	return form
}

func (mfw *MultipartFormWriter) AddValue(fieldname, value string) {
	mfw.Lock()
	defer mfw.Unlock()

	mfw.w.WriteField(fieldname, value)
	mfw.cachedMap[fieldname] = value
}

func (mfw *MultipartFormWriter) Reset() {
	mfw.buf.Reset()
	ResetMap(mfw.cachedMap)
	mfw.w = nil
}

func (mfw *MultipartFormWriter) AppendString(fieldname, value string) {
	mfw.AddValue(fieldname, value)
}

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

func NewMultipartForm(mfw *MultipartFormWriter) (string, *bytes.Buffer) {
	defer mfw.w.Close()

	return mfw.w.FormDataContentType(), mfw.buf
}

func (mfw *MultipartFormWriter) AppendFile(fieldname, filepath string) {
	filename := path.Base(filepath)

	mfw.AddFile(fieldname, filename, filepath)
}
