package controllers

import "github.com/google/uuid"

type RedisController struct {
}

func (r *RedisController) SetupDatabase(ld LoginDetails) error {

}

func (r *RedisController) GetDashManifest(id uuid.UUID) (string, error) {

}

func (r *RedisController) GetWidevineKeys(id uuid.UUID) ([]string, error) {

}

func (r *RedisController) GetWidevinePssh(id uuid.UUID) ([]byte, error) {

}

func (r *RedisController) GetWidevineInit(id uuid.UUID) ([]byte, error) {

}
