package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity/handlers/actcmd"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell/commands"
	"github.com/rs/zerolog/log"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

const (
	deviceInitTimeout = 2 * time.Minute
)

type ActiveStateHandler struct {
	configService     IConfigService
	systemdService    ISystemdService
	mqService         IMQService
	websocketService  IWebsocketService
	firstPortService  IFirstPortService
	deviceInitService IDeviceInitService
	activityService   IActivityService
	deviceType        string

	mqSubjects []string
}

func NewActiveStateHandler(configService IConfigService, systemdService ISystemdService,
	mqService IMQService, websocketService IWebsocketService, firstPortService IFirstPortService,
	deviceInitService IDeviceInitService, activityService IActivityService,
	deviceType string) *ActiveStateHandler {
	return &ActiveStateHandler{
		configService:     configService,
		systemdService:    systemdService,
		mqService:         mqService,
		websocketService:  websocketService,
		firstPortService:  firstPortService,
		deviceInitService: deviceInitService,
		activityService:   activityService,
		deviceType:        deviceType,

		mqSubjects: []string{
			constants.MQAgentReset,
			constants.MQAgentRebuildServices,
		},
	}
}

func (h *ActiveStateHandler) StateID() entities.AppState {
	return entities.AppStateActive
}

func (h *ActiveStateHandler) ValidateTransition(_ entities.AppState) (err error) {
	return nil
}

func (h *ActiveStateHandler) Handle(ctx context.Context, tx *activity.Transaction, transition common.IStateTransition) (result common.StateHandleResult, err error) {
	if _, ok := transition.(*entities.OnAfterBoot); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: after boot transition")

		if err = h.restoreActiveState(tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		return result, nil
	}

	if data, ok := transition.(*entities.OnFirstSetup); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: first setup transition")

		if err = h.runFirstSetup(ctx, tx, data.SerialNumber, data.OrchestratorAddrs); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		return result, nil
	}

	if data, ok := transition.(*entities.OnMigrateFromOldVersion); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: migrate from old version transition")

		if err = h.migrateFromOldVersion(ctx, tx, data.SerialNumber, data.OrchestratorAddr); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		return result, nil
	}

	if _, ok := transition.(*entities.OnFallback); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: fallback transition")

		// something went wrong
		// try to restore everything
		if err = h.restoreActiveState(tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		return result, nil
	}

	if _, ok := transition.(*entities.OnUpdateConfigFinished); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: update config finished transition")

		if err = h.subscribeQueueSubjects(tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		return result, nil
	}

	if data, ok := transition.(*entities.OnUpdateDeviceFinished); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: update device finished transition")

		if data.Err() != nil {
			log.Error().
				Err(data.Err()).
				Msg("Handle: update finished with error")
		}

		if err = h.subscribeQueueSubjects(tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		if err = h.activateServices(tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		return result, nil
	}

	return result, fmt.Errorf("Handle: %w", errs.ErrInvalidTransitionType)
}

func (h *ActiveStateHandler) OnExit(_ context.Context, tx *activity.Transaction, _ common.IStateTransition) (err error) {
	if err = h.unsubscribeQueueSubjects(tx); err != nil {
		return fmt.Errorf("OnExit: %w", err)
	}

	return nil
}

func (h *ActiveStateHandler) runFirstSetup(ctx context.Context, tx *activity.Transaction, serialNumber string, orchestratorAddrs []string) (err error) {
	cfg, err := h.configService.GetConfig()
	if err != nil {
		return fmt.Errorf("runFirstSetup: %w", err)
	}

	// check device has any wan port
	if cfg.Port == nil || len(cfg.Port.PortConfigs) == 0 {
		return fmt.Errorf("runFirstSetup: wan port not configured")
	}

	for _, orchestratorAddr := range orchestratorAddrs {
		if !strings.HasPrefix(orchestratorAddr, "http://") && !strings.HasPrefix(orchestratorAddr, "https://") {
			return fmt.Errorf("runFirstSetup: orchestrator address has no schema")
		}
	}

	if err = h.configService.UpdateConfigWithTx(
		ctx, tx,
		config.Config{
			App: &config.AppSection{
				SerialNumber:      serialNumber,
				OrchestratorAddrs: orchestratorAddrs,
			},
		},
	); err != nil {
		return fmt.Errorf("runFirstSetup: %w", err)
	}

	// activate update manager
	if err = h.activityService.ExecuteActivity(ctx, tx, actcmd.ActivityExecCommand, "enable update manager",
		actcmd.NewExecCommandPayload(
			commands.NewEnableServiceCmd(constants.UpdateManagerServiceName).String(),
			commands.NewDisableServiceCmd(constants.UpdateManagerServiceName).String(),
		),
	); err != nil {
		return fmt.Errorf("runFirstSetup: %w", err)
	}

	if err = h.activityService.ExecuteActivity(ctx, tx, actcmd.ActivityExecCommand, "start update manager",
		actcmd.NewExecCommandPayload(
			commands.NewStartServiceCmd(constants.UpdateManagerServiceName).String(),
			commands.NewStopServiceCmd(constants.UpdateManagerServiceName).String(),
		),
	); err != nil {
		return fmt.Errorf("runFirstSetup: %w", err)
	}

	waitInitChan := h.deviceInitService.WaitFirstInit(tx)

	// start publisher
	if err = h.activityService.ExecuteFunc(
		tx,
		h.websocketService.Start,
		h.websocketService.Stop,
	); err != nil {
		return fmt.Errorf("runFirstSetup: %w", err)
	}

	// wait for device init
	select {
	case err = <-waitInitChan:
	case <-time.After(deviceInitTimeout):
		err = errors.New("device init timeout")
	}
	if err != nil {
		return fmt.Errorf("runFirstSetup: %w", err)
	}

	if h.deviceType != constants.DeviceTypeHub {
		// deactivate dhcp server
		if err = h.systemdService.TryStopServiceWithTx(tx, constants.ISCDHCPServiceName); err != nil {
			return fmt.Errorf("runFirstSetup: %w", err)
		}
	}

	if err = h.subscribeQueueSubjects(tx); err != nil {
		return fmt.Errorf("runFirstSetup: %w", err)
	}

	if h.deviceType != constants.DeviceTypeHub {
		go func() {
			<-time.After(200 * time.Millisecond)
			h.deactivateStarter()
		}()
	}

	return nil
}

func (h *ActiveStateHandler) deactivateStarter() {
	if _, err := h.systemdService.TryStopService(constants.AgentStarterServiceName); err != nil {
		log.Error().
			Err(err).
			Msg("deactivateStarter")
	}

	// reset first port
	if _, err := h.firstPortService.ClearStatic(); err != nil {
		log.Error().
			Err(err).
			Msg("deactivateStarter")
	}
}

func (h *ActiveStateHandler) migrateFromOldVersion(ctx context.Context, tx *activity.Transaction, serialNumber, orchestratorAddr string) (err error) {
	cfg, err := h.configService.GetConfig()
	if err != nil {
		return fmt.Errorf("migrateFromOldVersion: %w", err)
	}

	app := cfg.App
	app.SerialNumber = serialNumber
	app.OrchestratorAddrs = []string{orchestratorAddr}
	if err = h.configService.UpdateConfigWithTx(
		ctx, tx,
		config.Config{
			App: app,
		},
	); err != nil {
		return fmt.Errorf("migrateFromOldVersion: %w", err)
	}

	if err = h.activateServices(tx); err != nil {
		return fmt.Errorf("migrateFromOldVersion: %w", err)
	}

	// start publisher
	if err = h.activityService.ExecuteFunc(
		tx,
		h.websocketService.Start,
		h.websocketService.Stop,
	); err != nil {
		return fmt.Errorf("migrateFromOldVersion: %w", err)
	}

	if err = h.subscribeQueueSubjects(tx); err != nil {
		return fmt.Errorf("migrateFromOldVersion: %w", err)
	}

	return nil
}

// restoreActiveState synchronizes systemd services states with app active state.
func (h *ActiveStateHandler) restoreActiveState(tx *activity.Transaction) (err error) {
	if err = h.activateServices(tx); err != nil {
		return fmt.Errorf("restoreActiveState: %w", err)
	}

	// deactivate starter
	if err = h.systemdService.TryStopServiceWithTx(tx, constants.AgentStarterServiceName); err != nil {
		return fmt.Errorf("restoreActiveState: %w", err)
	}

	// start publisher
	if !h.websocketService.IsStarted() {
		if err = h.activityService.ExecuteFunc(
			tx,
			h.websocketService.Start,
			h.websocketService.Stop,
		); err != nil {
			return fmt.Errorf("restoreActiveState: %w", err)
		}
	}

	if err = h.subscribeQueueSubjects(tx); err != nil {
		return fmt.Errorf("restoreActiveState: %w", err)
	}

	return nil
}

func (h *ActiveStateHandler) subscribeQueueSubjects(tx *activity.Transaction) (err error) {
	for _, subject := range h.mqSubjects {
		if err = h.activityService.ExecuteFunc(
			tx,
			func() error {
				return h.mqService.ActivateHandler(subject)
			},
			func() error {
				return h.mqService.DeactivateHandler(subject)
			},
		); err != nil {
			return fmt.Errorf("subscribeQueueSubjects: %w", err)
		}
	}

	return nil
}

func (h *ActiveStateHandler) unsubscribeQueueSubjects(tx *activity.Transaction) (err error) {
	for _, subject := range h.mqSubjects {
		if err = h.activityService.ExecuteFunc(
			tx,
			func() error {
				return h.mqService.DeactivateHandler(subject)
			},
			func() error {
				return h.mqService.ActivateHandler(subject)
			},
		); err != nil {
			return fmt.Errorf("unsubscribeQueueSubjects: %w", err)
		}
	}

	return nil
}

func (h *ActiveStateHandler) activateServices(tx *activity.Transaction) (err error) {
	// activate update manager
	if err = h.systemdService.TryStartServiceWithTx(tx, constants.UpdateManagerServiceName); err != nil {
		return fmt.Errorf("activateServices: %w", err)
	}

	return nil
}
