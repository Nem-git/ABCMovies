package season

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/plugin"
	seasonApi "github.com/nem-git/abcmovies/internal/season/api"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler(pis []*plugin.PluginInterface) http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/service/{serviceTag}/{showID}/{seasonNumber}", func(w http.ResponseWriter, r *http.Request) {

		tag := r.PathValue("serviceTag")
		showID := r.PathValue("showID")
		seasonNumberStr := r.PathValue("seasonNumber")

		seasonNumber, err := strconv.Atoi(seasonNumberStr)
		if err != nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("season number needs to be an integer"))
			return
		}

		pi, err := utils.GetPluginBySlug(tag, pis)
		if err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		request := seasonApi.SeasonRequestHandler(tag, showID, seasonNumber)

		response := &seasonApi.SeasonResponse{}

		(*pi).GetSeason(request, response)

		utils.JSONResponse(w, *response)
	})

	return mux
}
