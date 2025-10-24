package widevine

import (
	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/nem-git/abcmovies/internal/storage/cache/repository/widevine"
)

func NewSegmentController(repo *widevine.SegmentRepository) *SegmentController {
	c := new(SegmentController)
	c.repo = repo
	return c
}

type SegmentController struct {
	repo *widevine.SegmentRepository
}

func (c *SegmentController) Create(key string, value any) error {

	if err := c.repo.Create(key, value); err != nil {
		return err
	}

	return nil
}

func (c *SegmentController) ReadSingle(key string) (string, error) {

	value, err := c.repo.ReadSingle(key)

	if err != nil {
		return "", err
	}

	return value, nil
}

func (c *SegmentController) ReadCollection(key string) ([]string, error) {
	return nil, errs.ErrRedisValueEmpty
}

func (c *SegmentController) Update(key string, value any) error {

	if err := c.repo.Update(key, value); err != nil {
		return err
	}

	return nil
}

func (c *SegmentController) Delete(key string) error {

	if err := c.repo.Delete(key); err != nil {
		return err
	}

	return nil
}
