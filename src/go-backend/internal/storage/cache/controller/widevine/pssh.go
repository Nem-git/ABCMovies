package widevine

import (
	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/nem-git/abcmovies/internal/storage/cache/repository/widevine"
)

func NewPSSHController(repo *widevine.PSSHRepository) *PSSHController {
	c := new(PSSHController)
	c.repo = repo
	return c
}

type PSSHController struct {
	repo *widevine.PSSHRepository
}

func (c *PSSHController) Create(key string, value any) error {

	if err := c.repo.Create(key, value); err != nil {
		return err
	}

	return nil
}

func (c *PSSHController) ReadSingle(key string) (string, error) {

	value, err := c.repo.ReadSingle(key)

	if err != nil {
		return "", err
	}

	return value, nil
}

func (c *PSSHController) ReadCollection(key string) ([]string, error) {
	return nil, errs.ErrRedisValueEmpty
}

func (c *PSSHController) Update(key string, value any) error {

	if err := c.repo.Update(key, value); err != nil {
		return err
	}

	return nil
}

func (c *PSSHController) Delete(key string) error {

	if err := c.repo.Delete(key); err != nil {
		return err
	}

	return nil
}
