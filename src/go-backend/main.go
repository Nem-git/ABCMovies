package main

import (
	"abcmovies/models"
	"abcmovies/utils"
	"fmt"
	"net/http"

	"abcmovies/drm"
)

func getDashManifestHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, make([]int, 1))

}

func getDecryptionKeysHandler(w http.ResponseWriter, r *http.Request) {

	var requestData models.WidevineKeysRequest

	err := utils.JSONRequest(r, &requestData)
	if err != nil {
		utils.JSONError(w, fmt.Errorf("json data invalid: %w", err), http.StatusNotAcceptable)
		return
	}

	var wvd drm.Widevine

	keys, err := wvd.GetKeys(requestData.Pssh, requestData.Url, requestData.Headers)
	if err != nil {
		utils.JSONError(w, fmt.Errorf("couldn't retrieve decryption keys: %w", err), http.StatusNotAcceptable)
		return
	}

	var response models.WidevineKeysResponse

	response.Error = "0"
	response.Keys = keys

	utils.JSONResponse(w, response)
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
