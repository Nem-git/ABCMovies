package widevine

import "github.com/nem-git/abcmovies/internal/storage/cache/connector"

func NewSegmentRepository(conn connector.CacheConnector) *SegmentRepository {
	c := new(SegmentRepository)
	c.conn = conn
	return c
}

type SegmentRepository struct {
	conn connector.CacheConnector
}

// Puts an object in the db
func (r *SegmentRepository) Create(key string, value any) error {

	if err := r.conn.Create(key); err != nil {
		return err
	}

	return nil
}

// Select an object from the db
func (r *SegmentRepository) ReadSingle(key string) (string, error) {

	value, err := r.conn.FetchSingle(key)
	if err != nil {
		return "", err
	}

	return value, nil
}

// Select an object from the db
func (r *SegmentRepository) ReadCollection(key string) ([]string, error) {

	value, err := r.conn.FetchCollection(key)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Updates an object using id in db
func (r *SegmentRepository) Update(key string, value any) error {

	if err := r.conn.Update(key, value); err != nil {
		return err
	}

	return nil
}

// Deletes an object using id in db
func (r *SegmentRepository) Delete(key string) error {

	if err := r.conn.Delete(key); err != nil {
		return err
	}

	return nil
}
