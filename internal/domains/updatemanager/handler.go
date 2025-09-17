package updatemanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/validator"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/rs/zerolog/log"

	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type (
	IService interface {
		Download(request entities.DownloadPackageRequest) (err error)
		Install(request entities.InstallPackageRequest) (err error)
		GetVersions() (versions entities.ActualPackageVersions, err error)
	}

	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
		PublishRequest(method, to string, body any, options ...wschat.RequestOptions) (response wschat.WebsocketMessage, err error)
	}

	Handler struct {
		service   IService
		publisher IMessagePublisher
	}
)

func NewHandler(service IService, publisher IMessagePublisher) *Handler {
	return &Handler{
		service:   service,
		publisher: publisher,
	}
}

func (h *Handler) DownloadDevicePackages(message wschat.WebsocketMessage) (err error) {
	defer func() {
		err = h.handleErrorForPublisher(message, err)
	}()

	var request entities.DownloadPackageRequest
	if err = json.Unmarshal(message.Body, &request); err != nil {
		return fmt.Errorf("DownloadDevicePackages: %w", err)
	}

	if err = validator.Validator.Struct(request); err != nil {
		return fmt.Errorf("DownloadDevicePackages: %w", err)
	}

	log.Debug().Any("request", request).Msg("DownloadDevicePackages")

	if err = h.service.Download(request); err != nil {
		return fmt.Errorf("DownloadDevicePackages: %w", err)
	}

	if err = h.publisher.PublishResponse(message, []byte{}); err != nil {
		return fmt.Errorf("DownloadDevicePackages: %w", err)
	}

	return nil
}

func (h *Handler) InstallDevicePackages(message wschat.WebsocketMessage) (err error) {
	defer func() {
		err = h.handleErrorForPublisher(message, err)
	}()

	var request entities.InstallPackageRequest
	if err = json.Unmarshal(message.Body, &request); err != nil {
		return fmt.Errorf("InstallDevicePackages: %w", err)
	}

	if err = validator.Validator.Struct(request); err != nil {
		return fmt.Errorf("InstallDevicePackages: %w", err)
	}

	log.Debug().Any("request", request).Msg("InstallDevicePackages")

	if err = h.publisher.PublishResponse(message, wschat.EmptyBody); err != nil {
		return fmt.Errorf("InstallDevicePackages: %w", err)
	}

	if err = h.service.Install(request); err != nil {
		return fmt.Errorf("InstallDevicePackages: %w", err)
	}

	return nil
}

func (h *Handler) GetPackagesVersions(message wschat.WebsocketMessage) (err error) {
	defer func() {
		err = h.handleErrorForPublisher(message, err)
	}()

	versions, err := h.service.GetVersions()
	if err != nil {
		return fmt.Errorf("GetPackagesVersions: %w", err)
	}

	if err = h.publisher.PublishResponse(message, versions); err != nil {
		return fmt.Errorf("GetPackagesVersions: %w", err)
	}
	return nil
}

func (h *Handler) handleErrorForPublisher(message wschat.WebsocketMessage, err error) error {
	if err != nil {
		if sendErr := h.publisher.PublishErrorResponse(message, http.StatusInternalServerError, err.Error()); sendErr != nil {
			err = errors.Join(sendErr, err)
		}
	}
	return err
}
