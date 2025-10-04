package requests

import "net/http"

type IRequest interface {
	Map(req *http.Request) error
	Validate() error
}
