package main

import (
	"abcmovies/models"
	"abcmovies/utils"
	"fmt"
	"net/http"

	"abcmovies/drm"
	"abcmovies/streaming"
)

func getDashManifestHandler(w http.ResponseWriter, r *http.Request) {
	var requestData models.DashManifestModificationRequest

	err := utils.JSONRequest(r, &requestData)
	if err != nil {
		fmt.Println(fmt.Errorf("json data invalid: %w", err))
		utils.JSONError(w, fmt.Errorf("json data invalid: %w", err), http.StatusNotAcceptable)
		return
	}

	var dash streaming.Dash

	manifestString, err := dash.GetModifiedManifest(requestData.Url, requestData.Content)
	if err != nil {
		fmt.Println(fmt.Errorf("couldn't modify dash manifest: %w", err))
		utils.JSONError(w, fmt.Errorf("couldn't modify dash manifest: %w", err), http.StatusNotAcceptable)
		return
	}

	var response models.DashManifestModificationResponse

	response.Error = "0"
	response.Manifest = manifestString

	fmt.Println("modified dash manifest:", requestData.Url)

	utils.JSONResponse(w, response)
}

func getDecryptionKeysHandler(w http.ResponseWriter, r *http.Request) {

	var requestData models.WidevineKeysRequest

	err := utils.JSONRequest(r, &requestData)
	if err != nil {
		fmt.Println(fmt.Errorf("json data invalid: %w", err))
		utils.JSONError(w, fmt.Errorf("json data invalid: %w", err), http.StatusNotAcceptable)
		return
	}

	var wvd drm.Widevine

	keys, err := wvd.GetKeys(requestData.Pssh, requestData.Url, requestData.Headers)
	if err != nil {
		fmt.Println(fmt.Errorf("couldn't retrieve decryption keys: %w", err))
		utils.JSONError(w, fmt.Errorf("couldn't retrieve decryption keys: %w", err), http.StatusNotAcceptable)
		return
	}

	var response models.WidevineKeysResponse

	response.Error = "0"
	response.Keys = keys

	fmt.Println("found decryption keys:", keys)

	utils.JSONResponse(w, response)
}

func getPsshHandler(w http.ResponseWriter, r *http.Request) {

	var requestData models.WidevinePsshRequest

	err := utils.JSONRequest(r, &requestData)
	if err != nil {
		fmt.Println(fmt.Errorf("json data invalid: %w", err))
		utils.JSONError(w, fmt.Errorf("json data invalid: %w", err), http.StatusNotAcceptable)
		return
	}

	var wvd drm.Widevine

	pssh, err := wvd.GetPssh(requestData.Url, requestData.Headers, requestData.SegHeaders)
	if err != nil {
		fmt.Println(fmt.Errorf("couldn't retrieve pssh: %w", err))
		utils.JSONError(w, fmt.Errorf("couldn't retrieve pssh: %w", err), http.StatusNotAcceptable)
		return
	}

	var response models.WidevinePsshResponse

	response.Error = "0"
	response.Pssh = pssh

	fmt.Println("found pssh:", pssh)

	utils.JSONResponse(w, response)
}

func main() {

	mux := http.NewServeMux()

	mux.HandleFunc("POST /dash/manifest", getDashManifestHandler)
	mux.HandleFunc("POST /widevine/keys", getDecryptionKeysHandler)
	mux.HandleFunc("POST /widevine/pssh", getPsshHandler)

	http.ListenAndServe(":8090", mux)
}
