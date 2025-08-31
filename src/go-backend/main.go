package main

import (
	"abcmovies/models"
	"abcmovies/utils"
	"encoding/json"
	"fmt"
	"net/http"

	"abcmovies/drm"
)

type TaskServer interface {
	getDashManifestHandler(w http.ResponseWriter, r *http.Request)
	getDecryptionKeysHandler(w http.ResponseWriter, r *http.Request)
	getPsshHandler(w http.ResponseWriter, r *http.Request)
}

type taskServer struct {
}

func (ts *taskServer) getDashManifestHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, make([]int, 1))

}

func (ts *taskServer) getDecryptionKeysHandler(w http.ResponseWriter, r *http.Request) {

	var requestData models.WidevineKeysRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&requestData)
	if err != nil {
		utils.JSONError(w, fmt.Sprintf("json data invalid: %v", err), 200)
		return
	}

	var wvd drm.Widevine

	keys, err := wvd.GetKeys(requestData.Pssh, requestData.Url, requestData.Headers)
	if err != nil {
		utils.JSONError(w, fmt.Sprintf("couldn't retrieve decryption keys: %v", err), 200)
		return
	}

	data := make(map[string][]string)

	data["keys"] = keys
	utils.JSONResponse(w, data)
}

func (ts *taskServer) getPsshHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "got Pssh\n")
}

func main() {

	mux := http.NewServeMux()
	var ts TaskServer = &taskServer{}

	mux.HandleFunc("POST /dash", ts.getDashManifestHandler)
	mux.HandleFunc("POST /widevine/keys", ts.getDecryptionKeysHandler)
	mux.HandleFunc("POST /widevine/pssh", ts.getPsshHandler)

	http.ListenAndServe(":8090", mux)
}
