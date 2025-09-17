package fw

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

	ICmdService interface {
		ApplyCommandWithOutput(cmd string) (output []byte, err error)
	}

	Handler struct {
		messagePublisher IMessagePublisher
		cmdService       ICmdService
		cliExecutable    string

		validate *validator.Validate
	}
)

func NewHandler(messagePublisher IMessagePublisher, cmdService ICmdService, cliExecutable string) *Handler {
	return &Handler{
		messagePublisher: messagePublisher,
		cmdService:       cmdService,
		cliExecutable:    cliExecutable,

		validate: validator.New(),
	}
}

// GetFWFlowRules fetches fw flow rules for specified service.
func (h *Handler) GetFWFlowRules(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	var requestBody struct {
		ServiceID int `json:"serviceId"`
		Table     int `json:"table"`
	}
	if err = json.Unmarshal(request.Body, &requestBody); err != nil {
		return fmt.Errorf("GetFWFlowRules: %w", err)
	}

	if err = h.validate.Struct(requestBody); err != nil {
		return fmt.Errorf("GetFWFlowRules: %w", err)
	}

	cmd := buildListFlowRoutesCommand(h.cliExecutable, requestBody.ServiceID, requestBody.Table)
	output, err := h.cmdService.ApplyCommandWithOutput(cmd)
	if err != nil {
		return fmt.Errorf("GetFWFlowRules: %w", err)
	}

	responseBody := struct {
		Output []byte `json:"output"`
	}{
		Output: output,
	}
	if err = h.messagePublisher.PublishResponse(request, responseBody); err != nil {
		return fmt.Errorf("GetFWFlowRules: %w", err)
	}

	return nil
}

func buildListFlowRoutesCommand(cliExecutable string, serviceID, table int) string {
	return fmt.Sprintf("%s fw flow ls -i %d -t %d", cliExecutable, serviceID, table)
}
