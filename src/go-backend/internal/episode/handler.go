package episode

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler() http.Handler {

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

		request := EpisodeRequestHandler(r.PathValue("serviceTag"), r.PathValue("showID"), sn, n)

		response := EpisodeResponse{
			Number: request.EpisodeNumber,
		}

		utils.JSONResponse(w, response)
	})

	return mux
}
