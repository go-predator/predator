/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   form.go
 * @Created At:  2023-02-20 20:34:40
 * @Modified At: 2023-02-25 21:20:00
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
	multipartForm = sync.Pool{
		New: func() any {
			form := new(multipart.Form)
			form.File = make(map[string][]*multipart.FileHeader)
			form.Value = make(map[string][]string)

			return form
		},
	}
)

func resetMultipartForm(form *multipart.Form) {
	err := form.RemoveAll()
	if err != nil {
		panic(err)
	}

	ResetMap(form.File)
	ResetMap(form.Value)
}

func acquireMultipartForm() *multipart.Form {
	return multipartForm.Get().(*multipart.Form)
}

func releaseMultipartForm(form *multipart.Form) {
	if form != nil {
		resetMultipartForm(form)
	}
	multipartForm.Put(form)
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
	mfw.w.WriteField(fieldname, value)

	mfw.Lock()
	mfw.cachedMap[fieldname] = value
	mfw.Unlock()
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

	w, err := mfw.w.CreateFormFile(fieldname, filename)
	if err != nil {
		panic(err)
	}

	_, err = io.Copy(w, f)
	if err != nil {
		panic(err)
	}

	mfw.Lock()
	mfw.cachedMap[fieldname] = filename
	mfw.Unlock()
}

func NewMultipartForm(mfw *MultipartFormWriter) (string, *bytes.Buffer) {
	defer mfw.w.Close()

	return mfw.w.FormDataContentType(), mfw.buf
}

func (mfw *MultipartFormWriter) AppendFile(fieldname, filepath string) {
	filename := path.Base(filepath)

	mfw.AddFile(fieldname, filename, filepath)
}
