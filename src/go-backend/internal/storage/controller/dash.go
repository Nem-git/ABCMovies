package controller

import "github.com/google/uuid"

type DashController struct {
}

func (c *DashController) GetDashManifest(id uuid.UUID) (string, error) {
	return "", nil
}
