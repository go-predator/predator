/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: zlib.go (c) 2021
 * @Created: 2021-07-23 14:55:04
 * @Modified: 2021-07-30 12:22:14
 */

package tools

import (
	"bytes"
	"io"

	"github.com/klauspost/compress/zlib"
)

func Compress(src []byte) []byte {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write(src)
	w.Close()
	return buf.Bytes()
}

func Decompress(src []byte) ([]byte, error) {
	srcReader := bytes.NewReader(src)

	r, err := zlib.NewReader(srcReader)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	return buf.Bytes(), nil
}
