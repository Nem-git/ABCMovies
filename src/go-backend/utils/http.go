package utils

import (
	"encoding/json"
	"net/http"
)

func JSONResponse(w http.ResponseWriter, content any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(content)
}

func JSONError(w http.ResponseWriter, err any, code int) {
	w.WriteHeader(code)
	var data = make(map[string]any)
	data["code"] = code
	data["message"] = err
	JSONResponse(w, data)
}

func ParseJsonRequest(r *http.Request, model any) (any, error) {

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&model)
	if err != nil {
		return nil, err
	}

	return model, nil
}
