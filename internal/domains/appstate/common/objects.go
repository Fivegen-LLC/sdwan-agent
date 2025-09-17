package common

import (
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type IStateTransition interface {
	ToState() entities.AppState
}

type StateHandleResult struct {
	Transition IStateTransition
}
