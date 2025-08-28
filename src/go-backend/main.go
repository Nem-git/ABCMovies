package gobackend

import (
	"fmt"
	"net/http"
)

type TaskServer interface {
	getDashManifestHandler(w http.ResponseWriter, r *http.Request)
	getDecryptionKeysHandler(w http.ResponseWriter, r *http.Request)
	getPsshHandler(w http.ResponseWriter, r *http.Request)
}

type taskServer struct {
}

func (ts *taskServer) getDashManifestHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, make([]int, 1))

}

func (ts *taskServer) getDecryptionKeysHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "got Decryption Keys\n")
}

func (ts *taskServer) getPsshHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "got Pssh\n")
}

func main() {

	mux := http.NewServeMux()
	var ts TaskServer = &taskServer{}

	mux.HandleFunc("GET /dash", ts.getDashManifestHandler)
	mux.HandleFunc("GET /widevine/keys", ts.getDecryptionKeysHandler)
	mux.HandleFunc("GET /widevine/pssh", ts.getPsshHandler)

	http.ListenAndServe(":8090", mux)
}
