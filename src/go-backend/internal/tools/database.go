package tools

import (
	"errors"
	"os"
	"strconv"

	"github.com/nem-git/abcmovies/internal/controllers"
)

func NewController() (*controllers.BaseController, error) {

	address, ok := os.LookupEnv("DB_ADDR")
	if !ok {
		return nil, errors.New("couldn't find database address in environment variables")
	}
	password, ok := os.LookupEnv("DB_PW")
	if !ok {
		return nil, errors.New("couldn't find database password in environment variables")
	}
	d, ok := os.LookupEnv("DB_ID")
	if !ok {
		return nil, errors.New("couldn't find database identifier in environment variables")
	}
	db, err := strconv.Atoi(d)
	if err != nil {
		return nil, errors.New("database identifier in envionment variables not an integer")
	}
	p, ok := os.LookupEnv("DB_PROTOCOL")
	if !ok {
		return nil, errors.New("couldn't find database protocol in environment variables")
	}
	protocol, err := strconv.Atoi(p)
	if err != nil {
		return nil, errors.New("protocol in envionment variables not an integer")
	}

	ld := controllers.LoginDetails{
		Addr:     address,
		Password: password,
		DB:       db,
		Protocol: protocol,
	}

	var controller controllers.BaseController = &controllers.RedisController{}

	if err := controller.SetupDatabase(ld); err != nil {
		return nil, errors.New("couldn't setup database")
	}

	return &controller, nil
}
