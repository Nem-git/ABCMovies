package repository

import "github.com/nem-git/abcmovies/internal/storage/db/connector"

type DatabaseRepository interface {

	// Sets the database in the repo
	Setup(connector.DatabaseConnector) error

	// Puts an object in the db
	Create(any) (int, error)

	// Select an object from the db using int id
	ReadSingle(int) (any, error)

	// Select multiple objects from the db
	ReadCollection() ([]any, error)

	// Updates an object using id in db
	Update(int, any) error

	// Deletes an object using id in db
	Delete(int) error
}
