package utils

import (
	"abcmovies/models"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

func BindJSONOrErr[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var t T
	err := JSONRequest(r, &t)

	if err != nil {
		err = fmt.Errorf("json data body invalid: %w", err)
		log.Println(err)
		JSONError(w, err, http.StatusBadRequest)
		return *new(T), false
	}

	return t, true
}

func JSONResponse(w http.ResponseWriter, content models.Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(content)
}

func JSONError(w http.ResponseWriter, err error, code int) {
	w.WriteHeader(code)
	data := models.ErrorResponse{
		Error:   strconv.Itoa(code),
		Message: err.Error(),
	}
	JSONResponse(w, models.Response(data))
}

func JSONRequest[T models.Request](r *http.Request, model *T) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(model)
	if err != nil {
		return err
	}

	return err
}
