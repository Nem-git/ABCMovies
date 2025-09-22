package stream

// Request Slug
type StreamRequest struct {
}

// Stream Response
type StreamResponse struct {
	Content []byte
}

func RequestHandler() StreamRequest {
	return StreamRequest{}
}
