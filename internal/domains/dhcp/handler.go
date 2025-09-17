package dhcp

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell/commands"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/go-playground/validator/v10"
)

const (
	dhcpLeaseFolder = "/var/lib/dhcp"
)

type (
	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
	}

	IShellService interface {
		ExecOutput(command shell.ICommand) (output []byte, err error)
	}

	Handler struct {
		messagePublisher IMessagePublisher
		shellService     IShellService

		validate *validator.Validate
	}
)

func NewHandler(messagePublisher IMessagePublisher, shellService IShellService) *Handler {
	return &Handler{
		messagePublisher: messagePublisher,
		shellService:     shellService,

		validate: validator.New(),
	}
}

// FetchDHCPLeases fetches DHCP leases for vfr table.
func (h *Handler) FetchDHCPLeases(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	var requestBody struct {
		VrfName      string  `json:"vrfName" validate:"required"`
		FilterLinkIP *string `json:"filterLinkIp" validate:"omitempty,cidr"`
	}
	if err = json.Unmarshal(request.Body, &requestBody); err != nil {
		return fmt.Errorf("FetchDHCPLeases: %w", err)
	}
	if err = h.validate.Struct(requestBody); err != nil {
		return fmt.Errorf("FetchDHCPLeases: %w", err)
	}

	var result struct {
		Output string `json:"output"`
	}
	leaseFile := filepath.Join(dhcpLeaseFolder, fmt.Sprintf("%s.leases", requestBody.VrfName))
	lsDHCPLeaseCmd := commands.NewListDHCPLeaseCmd(leaseFile)
	data, err := h.shellService.ExecOutput(lsDHCPLeaseCmd)
	if err != nil {
		return fmt.Errorf("FetchDHCPLeases: %w", err)
	}

	formattedData, err := formatDHCPLeaseToTable(string(data), requestBody.FilterLinkIP)
	if err != nil {
		return fmt.Errorf("FetchDHCPLeases: %w", err)
	}

	result.Output = base64.StdEncoding.EncodeToString([]byte(formattedData))

	if err = h.messagePublisher.PublishResponse(request, result); err != nil {
		return fmt.Errorf("FetchDHCPLeases: %w", err)
	}

	return nil
}
