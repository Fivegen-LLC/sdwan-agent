package appstate

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"

	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type (
	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
	}

	IAppStateService interface {
		ActiveState() entities.AppState
	}

	WSHandler struct {
		publisher       IMessagePublisher
		appStateService IAppStateService
	}
)

func NewWSHandler(publisher IMessagePublisher, appStateService IAppStateService) *WSHandler {
	return &WSHandler{
		publisher:       publisher,
		appStateService: appStateService,
	}
}

// GetActiveState returns current agent state.
func (h *WSHandler) GetActiveState(message wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.publisher.PublishErrorResponse(message, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	response := struct {
		State string `json:"state"`
	}{
		State: h.appStateService.ActiveState().String(),
	}

	if err = h.publisher.PublishResponse(message, response); err != nil {
		return fmt.Errorf("GetActiveState: %w", err)
	}

	return nil
}
