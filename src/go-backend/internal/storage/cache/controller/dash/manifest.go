package dash

import (
	"github.com/nem-git/abcmovies/internal/storage/cache/repository/dash"
)

func NewManifestController(repo *dash.ManifestRepository) *ManifestController {
	c := new(ManifestController)
	c.repo = repo
	return c
}

type ManifestController struct {
	repo *dash.ManifestRepository
}

func (c *ManifestController) Create(key string, value any) error {

	if err := (*c.repo).Create(key, value); err != nil {
		return err
	}

	return nil
}

func (c *ManifestController) ReadSingle(key string) (string, error) {

	value, err := (*c.repo).ReadSingle(key)

	if err != nil {
		return "", err
	}

	return value, nil
}

func (c *ManifestController) ReadCollection(key string) ([]string, error) {

	value, err := (*c.repo).ReadCollection(key)

	if err != nil {
		return nil, err
	}

	return value, nil
}

func (c *ManifestController) Update(key string, value any) error {

	if err := (*c.repo).Update(key, value); err != nil {
		return err
	}

	return nil
}

func (c *ManifestController) Delete(key string) error {

	if err := (*c.repo).Delete(key); err != nil {
		return err
	}

	return nil
}
