package controllers

import "github.com/google/uuid"

type RedisController struct {
}

func (r *RedisController) SetupDatabase(ld LoginDetails) error {
	return nil
}

func (r *RedisController) GetDashManifest(id uuid.UUID) (string, error) {
	return "", nil
}

func (r *RedisController) GetWidevineKeys(id uuid.UUID) ([]string, error) {
	return nil, nil
}

func (r *RedisController) GetWidevinePssh(id uuid.UUID) ([]byte, error) {
	return nil, nil
}

func (r *RedisController) GetWidevineInit(id uuid.UUID) ([]byte, error) {
	return nil, nil
}
