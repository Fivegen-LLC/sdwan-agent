package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/validator"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/rs/zerolog/log"
)

type (
	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
	}

	Handler struct {
		publisher IMessagePublisher
	}
)

func NewHandler(publisher IMessagePublisher) *Handler {
	return &Handler{
		publisher: publisher,
	}
}

// ExecCommand handles command method from websocket.
func (h *Handler) ExecCommand(message wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.publisher.PublishErrorResponse(message, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = fmt.Errorf("%w: %w", sendErr, err)
			}
		}
	}()

	log.Debug().Msg("Handling command method")
	type requestBody struct {
		Command string   `json:"command"`
		Args    []string `json:"args,omitempty"`
	}

	var request requestBody
	if err = json.Unmarshal(message.Body, &request); err != nil {
		return fmt.Errorf("ExecCommand: %w", err)
	}

	var execCmd *exec.Cmd
	if len(request.Args) > 0 {
		execCmd = exec.Command(request.Command, request.Args...) //nolint:gosec // commands runs only by orchestrator
	} else {
		execCmd = exec.Command(request.Command) //nolint:gosec // commands runs only by orchestrator
	}

	log.Debug().Msgf("Executing cmd: %s", execCmd.String())
	cmdResult, err := execCmd.Output()
	if err != nil {
		log.Error().Err(err).Msg("ExecCommand")
		return fmt.Errorf("ExecCommand: %w", err)
	}

	responseBody := struct {
		Result string `json:"result"`
	}{
		Result: string(cmdResult),
	}
	if err = h.publisher.PublishResponse(message, responseBody); err != nil {
		return fmt.Errorf("ExecCommand: %w", err)
	}

	return nil
}

// FlushMACs flushes ovs mac addresses.
func (h *Handler) FlushMACs(message wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.publisher.PublishErrorResponse(message, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = fmt.Errorf("%w: %w", sendErr, err)
			}
		}
	}()

	log.Debug().Msg("Handling flush MACs method")
	type requestBody struct {
		Bridge string `json:"bridge" validate:"required"`
	}

	var request requestBody
	if err = json.Unmarshal(message.Body, &request); err != nil {
		return fmt.Errorf("FlushMACs: %w", err)
	}

	if err = validator.Validator.Struct(request); err != nil {
		return fmt.Errorf("FlushMACs: %w", err)
	}

	var (
		bridgeName = request.Bridge
		cmd        = exec.Command("ovs-appctl", "fdb/flush", bridgeName)
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Error().Err(err).Msgf("FlushMACs: output: %s", string(output))
		return fmt.Errorf("FlushMACs: %w", err)
	}

	if err = h.publisher.PublishResponse(message, []byte{}); err != nil {
		return fmt.Errorf("FlushMACs: %w", err)
	}

	return nil
}
