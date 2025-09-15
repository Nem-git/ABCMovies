package main

import (
	"log"

	"github.com/nem-git/abcmovies/models"

	"fmt"
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"

	"github.com/nem-git/abcmovies/drm"
	"github.com/nem-git/abcmovies/streaming"

	"github.com/nem-git/abcmovies/utils"
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

	response.Init = requestData.Init
	response.Segment = segment

	fmt.Println("removed drm from segment:", segment)

	utils.JSONResponse(w, response)
}

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	mux := http.NewServeMux()

	handlers.Handler(mux)

	log.Println("Welcome to ABCMovies' Go API!")

	if err := http.ListenAndServe(":8090", mux); err != nil {
		log.Println(err)
	}
}
