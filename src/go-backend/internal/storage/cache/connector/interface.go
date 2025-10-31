package connector

// Database connection
type ConnectionDetails struct {
	Address  string
	User     string
	Password string
	DB       string
}

type CacheConnector interface {
	Create(key string, value any) error
	FetchSingle(key string) (string, error)
	FetchCollection(key string) ([]string, error)
	Update(key string, value any) error
	Delete(key string) error
}
