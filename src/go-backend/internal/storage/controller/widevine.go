package controller

import "github.com/google/uuid"

type WidevineController struct {
}

func (c *WidevineController) GetWidevineKeys(id uuid.UUID) ([]string, error) {
	return nil, nil
}

func (c *WidevineController) GetWidevinePssh(id uuid.UUID) ([]byte, error) {
	return nil, nil
}

func (c *WidevineController) GetWidevineInit(id uuid.UUID) ([]byte, error) {
	return nil, nil
}
