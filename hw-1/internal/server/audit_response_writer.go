package server

import (
	"bytes"
	"net/http"
)

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
	buffer     bytes.Buffer
}

func newResponseWriterWrapper(w http.ResponseWriter) *responseWriterWrapper {
	return &responseWriterWrapper{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	w.buffer.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *responseWriterWrapper) GetStatusCode() int {
	return w.statusCode
}

func (w *responseWriterWrapper) GetBody() []byte {
	return w.buffer.Bytes()
}
