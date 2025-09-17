package appstate

import (
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
)

type transitionData struct {
	transition common.IStateTransition
	resultChan chan error
}

func newTransitionData(transition common.IStateTransition) transitionData {
	return transitionData{
		transition: transition,
		resultChan: make(chan error),
	}
}
