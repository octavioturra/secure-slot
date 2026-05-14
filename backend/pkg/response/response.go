package response

import (
	"encoding/json"
	"net/http"
)

func OK(w http.ResponseWriter, data any) {
	write(w, http.StatusOK, data)
}

func Created(w http.ResponseWriter, data any) {
	write(w, http.StatusCreated, data)
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func Error(w http.ResponseWriter, status int, code, message string) {
	write(w, status, errorBody{Error: message, Code: code})
}

type paginatedBody struct {
	Entries any `json:"entries"`
	Total   int `json:"total"`
	Page    int `json:"page"`
}

func Paginated(w http.ResponseWriter, entries any, total, page int) {
	write(w, http.StatusOK, paginatedBody{Entries: entries, Total: total, Page: page})
}

func write(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
