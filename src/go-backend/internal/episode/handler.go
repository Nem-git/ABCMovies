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

	mux.HandleFunc("GET /api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", func(w http.ResponseWriter, r *http.Request) {

		tag := r.PathValue("serviceTag")
		showID := r.PathValue("showID")
		seasonNumberStr := r.PathValue("seasonNumber")
		episodeNumberStr := r.PathValue("episodeNumber")

		seasonNumber, err := strconv.Atoi(seasonNumberStr)
		if err != nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("season number needs to be an integer"))
			return
		}
		episodeNumber, err := strconv.Atoi(episodeNumberStr)
		if err != nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("episode number needs to be an integer"))
			return
		}

		pi, err := utils.GetPluginBySlug(tag, pis)
		if err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		request := episodeApi.EpisodeRequestHandler(tag, showID, seasonNumber, episodeNumber)

		response := &episodeApi.EpisodeResponse{}

		(*pi).GetEpisode(request, response)

		utils.JSONResponse(w, *response)
	})

	mux.HandleFunc("GET /api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/next", func(w http.ResponseWriter, r *http.Request) {

		tag := r.PathValue("serviceTag")
		showID := r.PathValue("showID")
		seasonNumberStr := r.PathValue("seasonNumber")
		episodeNumberStr := r.PathValue("episodeNumber")

		seasonNumber, err := strconv.Atoi(seasonNumberStr)
		if err != nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("season number needs to be an integer"))
			return
		}

		episodeNumber, err := strconv.Atoi(episodeNumberStr)
		if err != nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("episode number needs to be an integer"))
			return
		}

		pi, err := utils.GetPluginBySlug(tag, pis)
		if err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		request := episodeApi.EpisodeRequestHandler(tag, showID, seasonNumber, episodeNumber)

		response := &episodeApi.NextEpisodeResponse{}

		(*pi).GetNextEpisode(request, response)

		utils.JSONResponse(w, *response)
	})

	return mux
}
