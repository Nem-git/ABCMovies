package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/nem-git/abcmovies/internal/models"
)

func BindJSONOrErr[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var t T

	if err := BindJSON(r, &t); err != nil {
		err = fmt.Errorf("json data body invalid: %w", err)
		log.Println(err)
		JSONError(w, err, http.StatusBadRequest)
		return *new(T), false
	}

	return t, true
}

func JSONResponse(w http.ResponseWriter, r models.Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(r)
}

func JSONError(w http.ResponseWriter, err error, code int) {
	w.WriteHeader(code)
	data := models.ErrorResponse{
		Error:   strconv.Itoa(code),
		Message: err.Error(),
	}
	JSONResponse(w, models.Response(data))
}

func BindJSON[T any](r *http.Request, model *T) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(model); err != nil {
		return err
	}

	return nil
}
