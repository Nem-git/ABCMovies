package connector

import (
	"context"
	"errors"
	"strconv"

	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/redis/go-redis/v9"
)

func NewRedisConnector(details ConnectionDetails) *RedisConnector {
	c := new(RedisConnector)

	c.context = context.TODO()

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

	// r := c.conn.Ping(c.context)
	// if r == nil {
	// 	return nil
	// }
	// _, err = r.Result()
	// if err != nil {
	// 	return nil
	// }

	return c
}

type RedisConnector struct {
	conn    *redis.Client
	context context.Context
}

func (c *RedisConnector) Create(key string, value any) error {

	switch t := value.(type) {
	case []string:

		out := make([]any, len(t))
		for i, v := range t {
			out[i] = v
		}

		r := c.conn.RPush(c.context, key, out...)

		if r == nil {
			return errors.New("unable to add to redis")
		}

		if err := r.Err(); err != nil {
			return err
		}

	case string:
		r := c.conn.Set(c.context, key, t, redis.KeepTTL) // TODO: Add real TTL
		if r == nil {
			return errors.New("unable to add to redis")
		}

		if err := r.Err(); err != nil {
			return err
		}
	}

	return nil
}

func (c *RedisConnector) FetchSingle(key string) (string, error) {

	r := c.conn.Get(c.context, key)
	if r == nil {
		return "", errors.New("unable to fetch single from redis")
	}

	v, err := r.Result()

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

	r := c.conn.LRange(c.context, key, 0, -1)
	if r == nil {
		return nil, errors.New("unable to fetch collection from redis")
	}

	v, err := r.Result()

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
	return errors.ErrUnsupported
}

func (c *RedisConnector) Delete(key string) error {

	r := c.conn.Del(c.context, key)

	if r == nil {
		return errors.New("unable to delete from redis")
	}

	if err := r.Err(); err != nil {
		return err
	}

	return nil
}
