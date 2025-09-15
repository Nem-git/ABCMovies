package handlers

import "net/http"

func DashHandler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("/manifest", func(w http.ResponseWriter, r *http.Request) {
		dash.Modify()
	})

	return http.StripPrefix("/api/stream/dash", mux)
}
