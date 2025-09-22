package utils

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
)

func BindJSONOrErr[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var t T

	if err := BindJSON(r, &t); err != nil {
		err = errors.New("json data body invalid")
		log.Println(err)
		api.BadRequestErrorHandler(w, err)
		return *new(T), false
	}

	return t, true
}

func JSONResponse(w http.ResponseWriter, r any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(r)
}

func BindJSON[T any](r *http.Request, model *T) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	return dec.Decode(model)
}
