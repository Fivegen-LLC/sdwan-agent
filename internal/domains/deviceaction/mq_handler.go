package deviceaction

import (
	"github.com/Fivegen-LLC/sdwan-lib/pkg/mq"
	"github.com/nats-io/nats.go"

	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type (
	IAppStateService interface {
		Perform(transition common.IStateTransition) (err error)
	}

	MQHandler struct {
		appStateService IAppStateService
	}
)

func NewMQHandler(appStateService IAppStateService) *MQHandler {
	return &MQHandler{
		appStateService: appStateService,
	}
}

// Reset resets device to ZTP state.
func (h *MQHandler) Reset(_ *nats.Msg) (resp any) {
	if err := h.appStateService.Perform(entities.NewOnReset()); err != nil {
		return mq.NewInternalErrorResponse(err.Error())
	}

	return mq.NewOkResponse()
}
