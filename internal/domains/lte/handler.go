package lte

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/go-playground/validator/v10"

	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type (
	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
	}

	ILTEService interface {
		CollectStats() (stats entities.LTEStats, err error)
		ResetModem(modemSysPath string) (err error)
	}

	Handler struct {
		messagePublisher IMessagePublisher
		lteService       ILTEService

		validate *validator.Validate
	}
)

func NewHandler(messagePublisher IMessagePublisher, lteService ILTEService) *Handler {
	return &Handler{
		messagePublisher: messagePublisher,
		lteService:       lteService,

		validate: validator.New(),
	}
}

// FetchStats collects LTE info from device.
func (h *Handler) FetchStats(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	stats, err := h.lteService.CollectStats()
	if err != nil {
		return fmt.Errorf("FetchStats: %w", err)
	}

	if err = h.messagePublisher.PublishResponse(request, stats); err != nil {
		return fmt.Errorf("FetchStats: %w", err)
	}

	return nil
}

// ResetModem resets LTE modem.
func (h *Handler) ResetModem(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	var requestBody struct {
		ModemSysPath string `json:"modemSysPath" validate:"required"`
	}
	if err = json.Unmarshal(request.Body, &requestBody); err != nil {
		return fmt.Errorf("ResetModem: %w", err)
	}

	if err = h.validate.Struct(requestBody); err != nil {
		return fmt.Errorf("ResetModem: %w", err)
	}

	if err = h.lteService.ResetModem(requestBody.ModemSysPath); err != nil {
		return fmt.Errorf("ResetModem: %w", err)
	}

	if err = h.messagePublisher.PublishResponse(request, wschat.EmptyBody); err != nil {
		return fmt.Errorf("ResetModem: %w", err)
	}

	return nil
}
