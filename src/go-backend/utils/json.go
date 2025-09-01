package utils

import (
	"abcmovies/models"
	"encoding/json"
	"net/http"
	"strconv"
)

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
