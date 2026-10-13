package slotwiring

import (
	"context"
	"log/slog"

	"github.com/nem-git/abcmovies/adapters/drmlocal"
	"github.com/nem-git/abcmovies/adapters/drmremote"
	"github.com/nem-git/abcmovies/core/drm"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

const (
	adapterDRMLocal  = "local-cdm"
	adapterDRMRemote = "remote"
)

// SetupDRM reads DRM slots from the store and builds a drm.Client.
// Returns nil if no DRM slot is configured (DRM content will be refused).
//
// The DRM adapter is wired into the engine's Options.DRMClient, not the sink
// factory. It is a single client, not a factory or resolver.
func SetupDRM(ctx context.Context, slots []*corev1.Slot, logger *slog.Logger) drm.Client {
	if logger == nil {
		logger = slog.Default()
	}
	for _, slot := range slots {
		if slot.GetKind() != corev1.SlotKind_SLOT_KIND_DRM {
			continue
		}
		drmCfg := slot.GetDrm()
		if drmCfg == nil {
			continue
		}
		var client drm.Client
		var err error
		switch drmCfg.GetAdapter() {
		case adapterDRMLocal:
			wvdPath := drmCfg.GetOptions()["wvd_path"]
			if wvdPath == "" {
				logger.Error("slotwiring: local-cdm: options.wvd_path is required", "slot", slot.GetId())
				continue
			}
			client, err = NewDRMLocal(wvdPath, logger)
		case adapterDRMRemote:
			serviceURL := drmCfg.GetOptions()["service_url"]
			deviceName := drmCfg.GetOptions()["device_name"]
			secretKey := drmCfg.GetOptions()["secret_key"]
			if serviceURL == "" {
				logger.Error("slotwiring: remote: options.service_url is required", "slot", slot.GetId())
				continue
			}
			if deviceName == "" {
				logger.Error("slotwiring: remote: options.device_name is required", "slot", slot.GetId())
				continue
			}
			client, err = NewDRMRemote(serviceURL, secretKey, deviceName, logger)
		default:
			logger.Warn("slotwiring: unknown DRM adapter", "adapter", drmCfg.GetAdapter(), "slot", slot.GetId())
			continue
		}
		if err != nil {
			logger.Error("slotwiring: failed to build DRM adapter", "slot", slot.GetId(), "err", err)
			continue
		}
		if client != nil {
			logger.Info("slotwiring: DRM adapter ready", "slot", slot.GetId(), "adapter", drmCfg.GetAdapter())
			return client
		}
	}
	logger.Info("slotwiring: no DRM slot configured; DRM content will be refused")
	return nil
}

// NewDRMLocal builds a local CDM adapter from the given .wvd path.
func NewDRMLocal(wvdPath string, logger *slog.Logger) (drm.Client, error) {
	return drmlocal.NewAdapter(wvdPath, logger)
}

// NewDRMRemote builds a remote key provider adapter from the given service
// URL, secret key, and device name.
func NewDRMRemote(serviceURL, secretKey, deviceName string, logger *slog.Logger) (drm.Client, error) {
	return drmremote.NewAdapter(serviceURL, secretKey, deviceName, logger)
}
