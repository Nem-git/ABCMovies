package handlers

import "net/http"

func WidevineHandler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		widevine.GetKeys()
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		widevine.GetKeys()
	})
	mux.HandleFunc("/pssh", func(w http.ResponseWriter, r *http.Request) {
		widevine.GetPssh()
	})
	mux.HandleFunc("/remove", func(w http.ResponseWriter, r *http.Request) {
		widevine.RemoveDRM()
	})

	return http.StripPrefix("/api/drm/widevine", mux)
}
