package common

import (
	"mime/multipart"
	"net/http"
	"strings"
)

func ReadFormWithFiles(w http.ResponseWriter, r *http.Request, maxRequestSize, maxMemory int64) ([]*multipart.FileHeader, error) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return nil, r.ParseForm()
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return nil, err
	}
	if r.MultipartForm == nil {
		return nil, nil
	}
	return r.MultipartForm.File["files"], nil
}
