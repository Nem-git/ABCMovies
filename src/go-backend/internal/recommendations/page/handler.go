package page

import (
	"net/http"
)

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /pages", func(w http.ResponseWriter, r *http.Request) {

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

	mux.HandleFunc("GET /pages/{pageID}", func(w http.ResponseWriter, r *http.Request) {

		pageID := r.PathValue("pageID")
		if pageID == "" {

		}

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

	return http.StripPrefix("/api/", mux)
}
