package ovs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/go-playground/validator/v10"
)

type (
	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
	}

	IOVSService interface {
		SetupOVSManager(ofControllerAddr string) (err error)
	}

	Handler struct {
		messagePublisher IMessagePublisher
		ovsService       IOVSService

		validate *validator.Validate
	}
)

func NewHandler(messagePublisher IMessagePublisher, ovsService IOVSService) *Handler {
	return &Handler{
		messagePublisher: messagePublisher,
		ovsService:       ovsService,

		validate: validator.New(),
	}
}

// SetupOVSManager installs OVS manager on device.
func (h *Handler) SetupOVSManager(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	var requestBody struct {
		OFControllerAddr string `json:"ofControllerAddr" validate:"hostname_port"`
	}
	if err = json.Unmarshal(request.Body, &requestBody); err != nil {
		return fmt.Errorf("SetupOVSManager: %w", err)
	}

	if err = h.validate.Struct(requestBody); err != nil {
		return fmt.Errorf("SetupOVSManager: %w", err)
	}

	if err = h.ovsService.SetupOVSManager(requestBody.OFControllerAddr); err != nil {
		return fmt.Errorf("SetupOVSManager: %w", err)
	}

	if err = h.messagePublisher.PublishResponse(request, []byte{}); err != nil {
		return fmt.Errorf("SetupOVSManager: %w", err)
	}

	return nil
}
