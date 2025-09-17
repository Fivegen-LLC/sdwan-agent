package ztp

import (
	"encoding/json"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/go-playground/validator/v10"
	"github.com/nats-io/nats.go"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/mq"

	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type (
	IAppStateService interface {
		Perform(transition common.IStateTransition) (err error)
	}

	MQHandler struct {
		appStateService IAppStateService

		validate *validator.Validate
	}
)

func NewMQHandler(appStateService IAppStateService) *MQHandler {
	return &MQHandler{
		appStateService: appStateService,

		validate: validator.New(),
	}
}

// SetPort updates wan port configuration.
func (h *MQHandler) SetPort(message *nats.Msg) (resp any) {
	var portConfig config.PortConfig
	if err := json.Unmarshal(message.Data, &portConfig); err != nil {
		return mq.NewBadRequestResponse(err.Error())
	}

	if err := h.validate.Struct(portConfig); err != nil {
		return mq.NewBadRequestResponse(err.Error())
	}

	if err := h.appStateService.Perform(
		entities.NewOnZTPSetupConfig(
			config.Config{
				Port: &config.PortSection{
					PortConfigs: []config.PortConfig{
						portConfig,
					},
				},
			},
		),
	); err != nil {
		return mq.NewInternalErrorResponse(err.Error())
	}

	return mq.NewOkResponse()
}

// DeletePort deletes wan port configuration.
func (h *MQHandler) DeletePort(_ *nats.Msg) (resp any) {
	if err := h.appStateService.Perform(
		entities.NewOnZTPSetupConfig(
			config.Config{
				Port: &config.PortSection{},
			},
		),
	); err != nil {
		return mq.NewInternalErrorResponse(err.Error())
	}

	return mq.NewOkResponse()
}

// RunFirstSetup starts device configuration (ZTP step).
func (h *MQHandler) RunFirstSetup(message *nats.Msg) (resp any) {
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

	if err := h.appStateService.Perform(
		entities.NewOnFirstSetup(
			request.SerialNumber,
			request.OrchestratorAddrs,
		),
	); err != nil {
		return mq.NewInternalErrorResponse(err.Error())
	}

	return mq.NewOkResponse()
}
