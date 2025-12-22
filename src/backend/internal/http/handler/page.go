package handler

// import (
// 	"net/http"

// 	"github.com/nem-git/abcmovies/internal/api"
// 	"github.com/nem-git/abcmovies/internal/models"
// 	"github.com/nem-git/abcmovies/internal/plugin"
// 	"github.com/nem-git/abcmovies/internal/utils"
// )

// type PageHandler struct {
// 	Request  models.PageRequest
// 	Response models.Page
// }

// func (h *PageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

// 	_, err := utils.GetPluginContextValue[plugin.IPlugin](r)
// 	if err != nil {
// 		api.BadRequestErrorHandler(w, err)
// 		return
// 	}

// 	utils.JSONResponse(w, h.Response)
// }

// func (h *PageHandler) Map(r *http.Request) error {

// 	return nil
// }

// func (h *PageHandler) GetRequest() any {
// 	return h.Request
// }

// func (h *PageHandler) GetResponse() any {
// 	return h.Response
// }
