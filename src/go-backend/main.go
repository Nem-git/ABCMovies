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

	requestData, ok := utils.BindJSONOrErr[models.DashManifestModificationRequest](w, r)
	if !ok {
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

	response.Manifest = manifestString

	fmt.Println("modified dash manifest:", requestData.Url)

	utils.JSONResponse(w, response)
}

func getWidevineDecryptionKeysHandler(w http.ResponseWriter, r *http.Request) {

	requestData, ok := utils.BindJSONOrErr[models.WidevineKeysRequest](w, r)
	if !ok {
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

	response.Keys = keys

	fmt.Println("found decryption keys:", keys)

	utils.JSONResponse(w, response)
}

func getWidevinePsshHandler(w http.ResponseWriter, r *http.Request) {

	requestData, ok := utils.BindJSONOrErr[models.WidevinePsshRequest](w, r)
	if !ok {
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

	response.Pssh = pssh

	fmt.Println("found pssh:", pssh)

	utils.JSONResponse(w, response)
}

func getWidevineRemovalHandler(w http.ResponseWriter, r *http.Request) {

	requestData, ok := utils.BindJSONOrErr[models.WidevineRemovalRequest](w, r)
	if !ok {
		return
	}

	var wvd drm.Widevine

	segment, err := wvd.GetDecryptedSegment(requestData.Init, requestData.Segment, requestData.Keys, requestData.IsInit)

	if err != nil {
		fmt.Println(fmt.Errorf("couldn't remove drm from segment: %w", err))
		utils.JSONError(w, fmt.Errorf("couldn't remove drm from segment: %w", err), http.StatusNotAcceptable)
		return
	}

	var response models.WidevineRemovalResponse

	response.Segment = segment

	fmt.Println("removed drm from segment:", segment)

	utils.JSONResponse(w, response)
}

func main() {

	mux := http.NewServeMux()

	mux.HandleFunc("POST /dash/manifest", getDashManifestHandler)
	mux.HandleFunc("POST /widevine/keys", getWidevineDecryptionKeysHandler)
	mux.HandleFunc("POST /widevine/pssh", getWidevinePsshHandler)
	mux.HandleFunc("POST /widevine/remove", getWidevineRemovalHandler)

	http.ListenAndServe(":8090", mux)
}
