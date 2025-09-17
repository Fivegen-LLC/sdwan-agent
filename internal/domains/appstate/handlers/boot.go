package handlers

import (
	"context"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"

	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type BootStateHandler struct{}

func NewBootStateHandler() *BootStateHandler {
	return new(BootStateHandler)
}

func (h *BootStateHandler) StateID() entities.AppState {
	return entities.AppStateBoot
}

func (h *BootStateHandler) ValidateTransition(_ entities.AppState) (err error) {
	return nil
}

func (h *BootStateHandler) Handle(_ context.Context, _ *activity.Transaction, _ common.IStateTransition) (result common.StateHandleResult, err error) {
	return result, nil
}

func (h *BootStateHandler) OnExit(_ context.Context, _ *activity.Transaction, _ common.IStateTransition) (err error) {
	return nil
}
