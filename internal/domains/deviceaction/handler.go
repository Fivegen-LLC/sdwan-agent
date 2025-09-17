package deviceaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/validator"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/rs/zerolog/log"

	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

type (
	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
		Stop()
	}

	IConfigService interface {
		Close() (err error)
	}

	ICmdService interface {
		ApplyCommandWithOutput(cmd string) (output []byte, err error)
	}

	Handler struct {
		messagePublisher IMessagePublisher
		cmdService       ICmdService
		appStateService  IAppStateService
		cliExtExecutable string
	}
)

func NewHandler(messagePublisher IMessagePublisher, cmdService ICmdService,
	appStateService IAppStateService, cliExtExecutable string) *Handler {
	return &Handler{
		messagePublisher: messagePublisher,
		cmdService:       cmdService,
		appStateService:  appStateService,
		cliExtExecutable: cliExtExecutable,
	}
}

// ExecDeviceAction executes device action command.
func (h *Handler) ExecDeviceAction(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	var execRequest entities.ExecDeviceAction
	if err = json.Unmarshal(request.Body, &execRequest); err != nil {
		return fmt.Errorf("ExecDeviceAction: %w", err)
	}

	if err = validator.Validator.Struct(execRequest); err != nil {
		return fmt.Errorf("ExecDeviceAction: %w", err)
	}

	if err = h.messagePublisher.PublishResponse(request, []byte{}); err != nil {
		return fmt.Errorf("ExecDeviceAction: %w", err)
	}

	go h.execAction(execRequest.Action)
	return nil
}

func (h *Handler) execAction(action entities.Action) {
	switch action {
	case entities.Reset:
		if err := h.appStateService.Perform(entities.NewOnReset()); err != nil {
			log.Error().Err(err).Msg("execAction")
		}

	case entities.Reboot, entities.PowerOff:
		if err := h.exec(action); err != nil {
			log.Error().Err(err).Msg("execAction")
		}

	default:
		log.Error().Err(errs.ErrUnknownDeviceAction).Msg(errs.ErrUnknownDeviceAction.Error())
	}
}

func (h *Handler) exec(action entities.Action) (err error) {
	// exec action.
	if output, err := h.cmdService.ApplyCommandWithOutput(action.String()); err != nil {
		return fmt.Errorf("exec: %w: output: %s", err, string(output))
	}

	return nil
}
