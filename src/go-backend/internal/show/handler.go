package show

import (
	"net/http"
)

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{showID}", func(w http.ResponseWriter, r *http.Request) {

		// content, err := GetManifest(model.Url, model.Content)
		// if err != nil {
		// 	api.InternalErrorHandler(w, err)
		// 	return
		// }

		// response := DashManifestResponse{
		// 	Content: content,
		// }

		// utils.JSONResponse(w, response)
	})

	return mux
}
