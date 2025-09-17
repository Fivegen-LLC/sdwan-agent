package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/go-playground/validator/v10"

	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type (
	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
	}

	IAppStateService interface {
		Perform(transition common.IStateTransition) (err error)
	}

	Handler struct {
		messagePublisher IMessagePublisher
		appStateService  IAppStateService

		validate *validator.Validate
	}
)

func NewHandler(messagePublisher IMessagePublisher, appStateService IAppStateService) *Handler {
	return &Handler{
		messagePublisher: messagePublisher,
		appStateService:  appStateService,

		validate: validator.New(),
	}
}

// UpdateConfig handles update service configuration for device.
func (h *Handler) UpdateConfig(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	var message struct {
		Trunk  *config.TrunkSection         `json:"trunk" validate:"omitempty"`
		L3     *config.L3ServiceSection     `json:"l3" validate:"omitempty"`
		ISB    *config.ISBSection           `json:"isb" validate:"omitempty"`
		Bridge *config.BridgeServiceSection `json:"bridge" validate:"omitempty"`
		P2P    *config.P2PServiceSection    `json:"p2p" validate:"omitempty"`
		FW     *config.FWSection            `json:"fw" validate:"omitempty"`
	}
	if err = json.Unmarshal(request.Body, &message); err != nil {
		return fmt.Errorf("UpdateConfig: %w", err)
	}

	if err = h.validate.Struct(message); err != nil {
		return fmt.Errorf("UpdateConfig: %w", err)
	}

	if err = h.appStateService.Perform(
		entities.NewOnUpdateConfig(
			config.Config{
				Trunk:  message.Trunk,
				L3:     message.L3,
				ISB:    message.ISB,
				Bridge: message.Bridge,
				P2P:    message.P2P,
				FW:     message.FW,
			},
		),
	); err != nil {
		return fmt.Errorf("UpdateConfig: %w", err)
	}

	if err = h.messagePublisher.PublishResponse(request, []byte{}); err != nil {
		return fmt.Errorf("UpdateConfig: %w", err)
	}

	return nil
}
