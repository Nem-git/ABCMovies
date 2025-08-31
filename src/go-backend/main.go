package main

import (
	"abcmovies/models"
	"abcmovies/utils"
	"encoding/json"
	"fmt"
	"net/http"

	"abcmovies/drm"
)

func getDashManifestHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, make([]int, 1))

}

func getDecryptionKeysHandler(w http.ResponseWriter, r *http.Request) {

	var requestData models.WidevineKeysRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&requestData)
	if err != nil {
		utils.JSONError(w, fmt.Sprintf("json data invalid: %v", err), http.StatusNotAcceptable)
		return
	}

	var wvd drm.Widevine

	keys, err := wvd.GetKeys(requestData.Pssh, requestData.Url, requestData.Headers)
	if err != nil {
		utils.JSONError(w, fmt.Sprintf("couldn't retrieve decryption keys: %v", err), http.StatusNotAcceptable)
		return
	}

	data := make(map[string]any)

	data["keys"] = keys
	data["error"] = "0"
	utils.JSONResponse(w, data)
}

func getPsshHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "got Pssh\n")
}

func main() {

	mux := http.NewServeMux()

	mux.HandleFunc("POST /dash", getDashManifestHandler)
	mux.HandleFunc("POST /widevine/keys", getDecryptionKeysHandler)
	mux.HandleFunc("POST /widevine/pssh", getPsshHandler)

	http.ListenAndServe(":8090", mux)
}
