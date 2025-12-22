package controller

type CacheController interface {

	// Puts an object in the db
	Create(key string, value any) error

	// Select an object from the db
	ReadSingle(key string) (string, error)

	// Select an object from the db
	ReadCollection(key string) ([]string, error)

	// Updates an object using id in db
	Update(key string, value any) error

	// Deletes an object using id in db
	Delete(key string) error
}
