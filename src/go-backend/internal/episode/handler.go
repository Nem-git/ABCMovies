package episode

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/nem-git/abcmovies/internal/api"
	episodeApi "github.com/nem-git/abcmovies/internal/episode/api"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler(pis []*plugin.PluginInterface) http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/services/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", func(w http.ResponseWriter, r *http.Request) {

		stringNumber := r.PathValue("seasonNumber")
		sn, err := strconv.Atoi(stringNumber)
		if err != nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("season number needs to be an integer"))
			return
		}

		stringNumber = r.PathValue("episodeNumber")
		n, err := strconv.Atoi(stringNumber)
		if err != nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("episode number needs to be an integer"))
			return
		}

		request := episodeApi.EpisodeRequestHandler(r.PathValue("serviceTag"), r.PathValue("showID"), sn, n)

		response := episodeApi.EpisodeResponse{
			Number: request.EpisodeNumber,
		}

		utils.JSONResponse(w, response)
	})

	return mux
}
