package controller

import "github.com/nem-git/abcmovies/internal/storage/db/repository"

type DatabaseController interface {

	// Sets the repo in the controller
	Setup(repository.DatabaseRepository) error

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
