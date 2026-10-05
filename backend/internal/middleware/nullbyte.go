package middleware

import (
	"mime"
	"net/http"
	"net/url"
	"strings"

	"sn-backend/internal/service/filesvc"
)

// hasNullByte says whether the path, the query or any form field (file names
// included) carries a NUL byte (%00). Text with one in it is never valid: it
// can cut strings short and make "admin\x00x" look like "admin". The form is
// parsed here, so the handlers reuse it and never parse the body again.
func hasNullByte(w http.ResponseWriter, r *http.Request) (bool, error) {
	if strings.ContainsRune(r.URL.Path, 0) {
		return true, nil
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return false, err
	}
	if valuesHaveNull(query) {
		return true, nil
	}

	contentType, _, contentTypeErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentTypeErr != nil {
		return false, nil
	}
	switch {
	case contentType == "multipart/form-data":
		r.Body = http.MaxBytesReader(w, r.Body, filesvc.MaxRequestSize)
		if err := r.ParseMultipartForm(filesvc.MaxMemory); err != nil {
			return false, err
		}
		if valuesHaveNull(r.MultipartForm.Value) {
			return true, nil
		}
		for name, headers := range r.MultipartForm.File {
			if strings.ContainsRune(name, 0) {
				return true, nil
			}
			for _, header := range headers {
				if strings.ContainsRune(header.Filename, 0) {
					return true, nil
				}
			}
		}
	case contentType == "application/x-www-form-urlencoded":
		if err := r.ParseForm(); err != nil {
			return false, err
		}
		if valuesHaveNull(r.PostForm) {
			return true, nil
		}
	}
	return false, nil
}

func valuesHaveNull(values map[string][]string) bool {
	for name, list := range values {
		if strings.ContainsRune(name, 0) {
			return true
		}
		for _, value := range list {
			if strings.ContainsRune(value, 0) {
				return true
			}
		}
	}
	return false
}

// rejectNullBytes answers 400 when the request carries a NUL byte, and says
// whether it did.
func rejectNullBytes(w http.ResponseWriter, r *http.Request) bool {
	found, err := hasNullByte(w, r)
	if err != nil {
		http.Error(w, "request is too large or invalid", http.StatusBadRequest)
		return true
	}
	if found {
		http.Error(w, "request contains a null byte", http.StatusBadRequest)
		return true
	}
	return false
}
