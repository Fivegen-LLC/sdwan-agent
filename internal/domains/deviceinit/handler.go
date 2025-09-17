package deviceinit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type (
	IDeviceInitService interface {
		InitDevice(initConfig entities.InitConfig) (err error)
		IsInitializing() bool
	}

	IAppStateService interface {
		ActiveState() entities.AppState
	}
)

type Handler struct {
	messagePublisher  IMessagePublisher
	deviceInitService IDeviceInitService
	appStateService   IAppStateService
}

func NewHandler(messagePublisher IMessagePublisher, deviceInitService IDeviceInitService, appStateService IAppStateService) *Handler {
	return &Handler{
		messagePublisher:  messagePublisher,
		deviceInitService: deviceInitService,
		appStateService:   appStateService,
	}
}

// InitDevice handles init device websocket request.
func (h *Handler) InitDevice(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = fmt.Errorf("%w: %w", sendErr, err)
			}
		}
	}()

	var initConfig entities.InitConfig
	if err = json.Unmarshal(request.Body, &initConfig); err != nil {
		return fmt.Errorf("InitDevice: %w", err)
	}

	log.Debug().
		Any("configs", initConfig).
		Msg("got init config")

	activeState := h.appStateService.ActiveState()
	if activeState != entities.AppStateActive {
		return fmt.Errorf("InitDevice: device not in active state (current state: %s)", activeState)
	}

	if h.deviceInitService.IsInitializing() {
		return fmt.Errorf("InitDevice: device already initializing")
	}

	if err = h.messagePublisher.PublishResponse(request, wschat.EmptyBody); err != nil {
		return fmt.Errorf("InitDevice: %w", err)
	}

	go func() {
		initErr := h.deviceInitService.InitDevice(initConfig)
		if initErr != nil {
			log.Error().
				Err(initErr).
				Msg("InitDevice: init device error")
		}

		if sErr := h.sendInitDeviceFinished(initErr); sErr != nil {
			log.Error().
				Err(sErr).
				Msg("InitDevice: init device error")
		}
	}()

	return nil
}

func (h *Handler) sendInitDeviceFinished(initErr error) (err error) {
	body := struct {
		ErrorMessage string `json:"errorMessage"`
	}{}
	if initErr != nil {
		body.ErrorMessage = initErr.Error()
	}

	const attemptInterval = 2 * time.Second
	var (
		resp     wschat.WebsocketMessage
		attempts = 5
	)
LOOP:
	for {
		resp, err = h.messagePublisher.PublishRequest(
			constants.MethodInitDeviceFinished,
			constants.OrchestratorWSID,
			body,
			wschat.RequestOptions{
				Timeout: lo.ToPtr(5 * time.Second),
			},
		)
		switch {
		case err == nil && !resp.IsErrorResponse():
			break LOOP

		case err != nil:
			// retry

		case resp.IsErrorResponse():
			if resp.ResponseParams.StatusCode != http.StatusServiceUnavailable {
				break LOOP
			}
			// retry
		}

		attempts--
		if attempts == 0 {
			log.Error().Msg("sendInitDeviceFinished: no more connect attempts")
			break
		}

		<-time.After(attemptInterval)
	}
	if err != nil {
		h.messagePublisher.Reconnect()
		return fmt.Errorf("sendInitDeviceFinished: %w", err)
	}

	if resp.IsErrorResponse() {
		h.messagePublisher.Reconnect()
		return fmt.Errorf("sendInitDeviceFinished: %w", resp.Error())
	}

	return nil
}
