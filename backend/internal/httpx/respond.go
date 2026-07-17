// Package httpx provides the standard API response envelope.
package httpx

import (
	"encoding/json"
	"net/http"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type envelope struct {
	Data  any        `json:"data,omitempty"`
	Meta  any        `json:"meta,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
}

func write(w http.ResponseWriter, status int, e envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(e)
}

func Data(w http.ResponseWriter, status int, data any) {
	write(w, status, envelope{Data: data})
}

func DataMeta(w http.ResponseWriter, status int, data, meta any) {
	write(w, status, envelope{Data: data, Meta: meta})
}

func Error(w http.ResponseWriter, status int, code, message string) {
	write(w, status, envelope{Error: &ErrorBody{Code: code, Message: message}})
}
