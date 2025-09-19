package season

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/services/{serviceTag}/{showID}/{seasonNumber}", func(w http.ResponseWriter, r *http.Request) {

		stringNumber := r.PathValue("seasonNumber")
		n, err := strconv.Atoi(stringNumber)
		if err != nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("season number needs to be an integer"))
			return
		}

		request := SeasonRequestHandler(r.PathValue("serviceTag"), r.PathValue("showID"), n)

		response := SeasonResponse{
			Number: request.SeasonNumber,
		}

		utils.JSONResponse(w, response)
	})

	return mux
}
