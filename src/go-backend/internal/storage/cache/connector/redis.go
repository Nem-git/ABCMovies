package connector

import (
	"context"
	"strconv"

	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/redis/go-redis/v9"
)

func NewRedisConnector(details ConnectionDetails) *RedisConnector {
	c := new(RedisConnector)

	ctx := context.TODO()

	db, err := strconv.Atoi(details.DB)
	if err != nil {
		return nil
	}

	c.conn = redis.NewClient(&redis.Options{
		Addr:     details.Address,
		Username: details.User,
		Password: details.Password,
		DB:       db,
	})

	_, err = c.conn.Ping(ctx).Result()
	if err != nil {
		return nil
	}

	return c
}

type RedisConnector struct {
	conn    *redis.Client
	context context.Context
}

func (c *RedisConnector) Create(key string) error {
	return nil
}

func (c *RedisConnector) FetchSingle(key string) (string, error) {

	v, err := c.conn.Get(c.context, key).Result()
	switch {
	case err == redis.Nil:
		return "", errs.ErrRedisKeyDoesNotExist
	case err != nil:
		return "", err
	case v == "":
		return "", errs.ErrRedisValueEmpty
	}

	return v, nil
}

func (c *RedisConnector) FetchCollection(key string) ([]string, error) {

	v, err := c.conn.LRange(c.context, key, 0, -1).Result()
	switch {
	case err == redis.Nil:
		return nil, errs.ErrRedisKeyDoesNotExist
	case err != nil:
		return nil, err
	case len(v) == 0:
		return nil, errs.ErrRedisValueEmpty
	}

	return v, nil
}

func (c *RedisConnector) Update(key string, value any) error {
	return nil
}

func (c *RedisConnector) Delete(key string) error {
	return nil
}
