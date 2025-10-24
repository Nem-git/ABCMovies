package dash

import "github.com/nem-git/abcmovies/internal/storage/cache/connector"

type ManifestRepository struct {
	conn connector.CacheConnector
}

// Puts an object in the db
func (r *ManifestRepository) Create(key string, value any) error {

	if err := r.conn.Create(key); err != nil {
		return err
	}

	return nil
}

// Select an object from the db
func (r *ManifestRepository) ReadSingle(key string) (string, error) {

	value, err := r.conn.FetchSingle(key)
	if err != nil {
		return "", err
	}

	return value, nil
}

// Select an object from the db
func (r *ManifestRepository) ReadCollection(key string) ([]string, error) {

	value, err := r.conn.FetchCollection(key)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Updates an object using id in db
func (r *ManifestRepository) Update(key string, value any) error {

	if err := r.conn.Update(key, value); err != nil {
		return err
	}

	return nil
}

// Deletes an object using id in db
func (r *ManifestRepository) Delete(key string) error {

	if err := r.conn.Delete(key); err != nil {
		return err
	}

	return nil
}
