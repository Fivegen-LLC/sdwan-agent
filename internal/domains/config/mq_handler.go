package config

import (
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/mq"
	"github.com/nats-io/nats.go"

	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type (
	IConfigService interface {
		GetConfig() (cfg config.Config, err error)
	}

	MQHandler struct {
		configService   IConfigService
		appStateService IAppStateService
	}
)

func NewMQHandler(configService IConfigService, appStateService IAppStateService) *MQHandler {
	return &MQHandler{
		configService:   configService,
		appStateService: appStateService,
	}
}

func (h *MQHandler) GetConfig(_ *nats.Msg) (resp any) {
	cfg, err := h.configService.GetConfig()
	if err != nil {
		return mq.NewInternalErrorResponse(err.Error())
	}

	response := struct {
		mq.Response

		Config config.Config `json:"config"`
	}{
		Response: mq.NewOkResponse(),
		Config:   cfg,
	}

	return response
}

func (h *MQHandler) RebuildServices(_ *nats.Msg) (resp any) {
	if err := h.appStateService.Perform(entities.NewOnRebuildServices()); err != nil {
		return mq.NewInternalErrorResponse(err.Error())
	}

	return mq.NewOkResponse()
}
