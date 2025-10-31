package widevine

import (
	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/nem-git/abcmovies/internal/storage/cache/repository/widevine"
)

func NewKeysController(repo *widevine.KeysRepository) *KeysController {
	c := new(KeysController)
	c.repo = repo
	return c
}

type KeysController struct {
	repo *widevine.KeysRepository
}

func (c *KeysController) Create(key string, value any) error {

	if err := c.repo.Create(key, value); err != nil {
		return err
	}

	return nil
}

func (c *KeysController) ReadSingle(key string) (string, error) {
	return "", errs.ErrRedisValueEmpty
}

func (c *KeysController) ReadCollection(key string) ([]string, error) {

	value, err := c.repo.ReadCollection(key)

	if err != nil {
		return nil, err
	}

	return value, nil
}

func (c *KeysController) Update(key string, value any) error {

	if err := c.repo.Update(key, value); err != nil {
		return err
	}

	return nil
}

func (c *KeysController) Delete(key string) error {

	if err := c.repo.Delete(key); err != nil {
		return err
	}

	return nil
}
