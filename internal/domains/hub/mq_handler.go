package hub

import (
	"encoding/json"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/mq"
	"github.com/go-playground/validator/v10"
	"github.com/nats-io/nats.go"

	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type (
	IHubService interface {
		SetPort(portName string) (err error)
		DeletePort(portName string) (err error)
		ListPorts() (ports entities.HubPorts, err error)
		Init(serialNumber string, orchestratorAddrs []string) (err error)
	}

	MQHandler struct {
		hubService IHubService

		validate *validator.Validate
	}
)

func NewMQHandler(hubService IHubService) *MQHandler {
	return &MQHandler{
		hubService: hubService,

		validate: validator.New(),
	}
}

// SetPort sets hub port as wan.
func (h *MQHandler) SetPort(message *nats.Msg) (resp any) {
	var request struct {
		PortName string `json:"portName" validate:"required"`
	}
	if err := json.Unmarshal(message.Data, &request); err != nil {
		return mq.NewBadRequestResponse(err.Error())
	}

	if err := h.validate.Struct(request); err != nil {
		return mq.NewBadRequestResponse(err.Error())
	}

	if err := h.hubService.SetPort(request.PortName); err != nil {
		return mq.NewInternalErrorResponse(err.Error())
	}

	return mq.NewOkResponse()
}

// DeletePort deletes port configuration from hub.
func (h *MQHandler) DeletePort(message *nats.Msg) (resp any) {
	var request struct {
		PortName string `json:"portName" validate:"required"`
	}
	if err := json.Unmarshal(message.Data, &request); err != nil {
		return mq.NewBadRequestResponse(err.Error())
	}

	if err := h.validate.Struct(request); err != nil {
		return mq.NewBadRequestResponse(err.Error())
	}

	if err := h.hubService.DeletePort(request.PortName); err != nil {
		return mq.NewInternalErrorResponse(err.Error())
	}

	return mq.NewOkResponse()
}

// ListPorts queries hub ports with configuration.
func (h *MQHandler) ListPorts(_ *nats.Msg) (resp any) {
	ports, err := h.hubService.ListPorts()
	if err != nil {
		return mq.NewInternalErrorResponse(err.Error())
	}

	response := struct {
		mq.Response
		Ports entities.HubPorts `json:"ports"`
	}{
		Response: mq.NewOkResponse(),
		Ports:    ports,
	}

	return response
}

// Init connects hub to orchestrator (ZTP for hub).
func (h *MQHandler) Init(message *nats.Msg) (resp any) {
	var request struct {
		SerialNumber      string   `json:"serialNumber" validate:"required"`
		OrchestratorAddrs []string `json:"orchestratorAddrs" validate:"required"`
	}
	if err := json.Unmarshal(message.Data, &request); err != nil {
		return mq.NewBadRequestResponse(err.Error())
	}

	if err := h.validate.Struct(request); err != nil {
		return mq.NewBadRequestResponse(err.Error())
	}

	if err := h.hubService.Init(request.SerialNumber, request.OrchestratorAddrs); err != nil {
		return mq.NewInternalErrorResponse(err.Error())
	}

	return mq.NewOkResponse()
}
