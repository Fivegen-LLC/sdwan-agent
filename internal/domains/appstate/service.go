package appstate

import (
	"context"
	"errors"
	"fmt"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

type (
	IStateHandler interface {
		StateID() entities.AppState
		ValidateTransition(fromState entities.AppState) (err error)
		Handle(ctx context.Context, tx *activity.Transaction, transition common.IStateTransition) (result common.StateHandleResult, err error)
		OnExit(ctx context.Context, tx *activity.Transaction, transition common.IStateTransition) (err error)
	}

	IConfigService interface {
		GetConfig() (cfg config.Config, err error)
		UpdateConfigWithTx(ctx context.Context, tx *activity.Transaction, cfg config.Config, updateFuncs ...config.UpdateOption) (err error)
	}

	IActivityService interface {
		StartTransaction(ctx context.Context, name string, options ...activity.TransactionOption) (transaction *activity.Transaction, err error)
		FinishTransaction(ctx context.Context, transaction *activity.Transaction, execErr error) (err error)
		ExecuteFunc(transaction *activity.Transaction, fn, rlFn func() error) (err error)
	}
)

type StateService struct {
	configService   IConfigService
	activityService IActivityService
	initState       entities.AppState

	stateHandlers  map[entities.AppState]IStateHandler
	activeState    entities.AppState
	transitionChan chan transitionData
}

func NewService(configService IConfigService, activityService IActivityService, initState entities.AppState) *StateService {
	return &StateService{
		configService:   configService,
		activityService: activityService,
		initState:       initState,

		activeState:    entities.AppStateBoot,
		transitionChan: make(chan transitionData),
	}
}

func (s *StateService) SetStateHandlers(stateHandlers []IStateHandler) {
	handlerMap := make(map[entities.AppState]IStateHandler)
	for _, handler := range stateHandlers {
		if _, exists := handlerMap[handler.StateID()]; exists {
			log.Fatal().
				Any("state", handler.StateID()).
				Msg("NewStateService: duplicate state found")
		}

		handlerMap[handler.StateID()] = handler
	}

	if _, exists := handlerMap[s.activeState]; !exists {
		log.Fatal().Msg("NewStateService: init state handler not found")
	}

	if _, exists := handlerMap[entities.AppStateBoot]; !exists {
		log.Fatal().Msg("NewStateService: boot state handler not found")
	}

	s.stateHandlers = handlerMap
}

// ActiveState returns active app state.
func (s *StateService) ActiveState() entities.AppState {
	return s.activeState
}

// Perform starts transition to new state.
func (s *StateService) Perform(transition common.IStateTransition) (err error) {
	data := newTransitionData(transition)
	defer close(data.resultChan)

	s.transitionChan <- data
	if err = <-data.resultChan; err != nil {
		log.Error().
			Err(err).
			Msg("Perform: perform transition error")

		return fmt.Errorf("Perform: %w", err)
	}

	return nil
}

// Run starts state service.
func (s *StateService) Run(ctx context.Context) {
	defer close(s.transitionChan)

	// move to current state after reboot
	if err := s.setActiveStateFromBoot(ctx); err != nil {
		log.Fatal().
			Err(err).
			Msg("Run")
	}

	// listen for transition commands
	for {
		select {
		case data := <-s.transitionChan:
			var (
				transition = data.transition
				err        error
			)
			tx, err := s.activityService.StartTransaction(ctx, "perform state transition",
				activity.NewRollbackStrategyOption(activity.RollbackStrategySkipOnFail),
			)
			if err != nil {
				data.resultChan <- fmt.Errorf("Run: %w", err)
				break
			}

			for {
				var result common.StateHandleResult
				result, err = s.performTransition(ctx, tx, transition)
				if err != nil {
					break
				}

				if result.Transition == nil {
					break
				}

				transition = result.Transition
			}

			err = s.activityService.FinishTransaction(ctx, tx, err)
			if err != nil {
				data.resultChan <- fmt.Errorf("Run: %w", err)
				break
			}

			data.resultChan <- nil

		case <-ctx.Done():
			return
		}
	}
}

// setActiveStateFromBoot applies transition from boot to active state.
func (s *StateService) setActiveStateFromBoot(ctx context.Context) (err error) {
	tx, err := s.activityService.StartTransaction(ctx, "after boot transition",
		activity.NewRollbackStrategyOption(activity.RollbackStrategySkipOnFail),
	)
	if err != nil {
		return fmt.Errorf("setActiveStateFromBoot: %w", err)
	}
	defer func() {
		err = s.activityService.FinishTransaction(ctx, tx, err)
	}()

	cfg, err := s.configService.GetConfig()
	if err != nil {
		return fmt.Errorf("setActiveStateFromBoot: %w", err)
	}

	toState := s.initState
	if !lo.IsEmpty(cfg.AppState.State) {
		toState = entities.AppState(cfg.AppState.State)
	}

	var transition common.IStateTransition = entities.NewOnAfterBoot(toState)
	for {
		result, err := s.performTransition(ctx, tx, transition)
		if err != nil {
			return fmt.Errorf("setActiveStateFromBoot: %w", err)
		}

		if result.Transition != nil {
			transition = result.Transition
			continue
		}

		break
	}

	return nil
}

func (s *StateService) performTransition(ctx context.Context, tx *activity.Transaction, transition common.IStateTransition) (result common.StateHandleResult, err error) {
	newStateID := transition.ToState()
	// validate transition
	if s.activeState == newStateID {
		return result, fmt.Errorf("performTransition: already in state %s", s.activeState)
	}

	toState, exists := s.stateHandlers[newStateID]
	if !exists {
		return result, fmt.Errorf("performTransition: handler for state %s not found", newStateID)
	}

	if err = toState.ValidateTransition(s.activeState); err != nil {
		if errors.Is(err, errs.ErrTransitionNotSupported) {
			err = fmt.Errorf("%w: transition from %s to %s not supported", err, s.activeState, newStateID)
		}

		return result, fmt.Errorf("performTransition: %w", err)
	}

	// exit active state
	if activeHandler, exists := s.stateHandlers[s.activeState]; exists {
		if err = activeHandler.OnExit(ctx, tx, transition); err != nil {
			return result, fmt.Errorf("performTransition: %w", err)
		}
	} else {
		log.Error().Msgf("performTransition: handler for state %s not found", s.activeState)
	}

	// apply new state
	oldStateID := s.activeState
	if err = s.updateAppState(ctx, tx, newStateID); err != nil {
		return result, fmt.Errorf("performTransition: %w", err)
	}

	handleResult, err := toState.Handle(ctx, tx, transition)
	if err != nil {
		return result, fmt.Errorf("performTransition: %w", err)
	}

	if handleResult.Transition != nil {
		result.Transition = handleResult.Transition
	}

	log.Info().
		Any("old state", oldStateID).
		Any("new state", newStateID).
		Msg("performTransition: transitioned to new state")

	return result, nil
}

// updateAppState updates app state and saves it to config.
func (s *StateService) updateAppState(ctx context.Context, tx *activity.Transaction, newStateID entities.AppState) (err error) {
	if err = s.configService.UpdateConfigWithTx(
		ctx, tx,
		config.Config{
			AppState: &config.AppStateSection{
				State: string(newStateID),
			},
		},
	); err != nil {
		return fmt.Errorf("updateAppState: %w", err)
	}

	// update runtime state
	oldStateID := s.activeState
	if err = s.activityService.ExecuteFunc(
		tx,
		func() error {
			s.activeState = newStateID
			return nil
		},
		func() error {
			if oldStateID != entities.AppStateBoot {
				s.activeState = oldStateID
			}
			return nil
		},
	); err != nil {
		return fmt.Errorf("updateAppState: %w", err)
	}

	return nil
}
