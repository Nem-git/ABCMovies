package connector

// Connection informations
type ConnectionDetails struct {
	Address  string
	User     string
	Password string
	DB       string
}

type DatabaseConnector interface {
	Setup(ConnectionDetails) error
	Execute(string) error
	FetchSingle(string) (any, error)
	FetchCollection(string) ([]any, error)
}
