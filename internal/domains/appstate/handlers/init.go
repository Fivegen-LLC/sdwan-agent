package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/mq"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/netinit"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/ping"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/rollback"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell/commands"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
	"github.com/Fivegen-LLC/sdwan-agent/internal/environment"
	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

type (
	IShellService interface {
		Exec(command shell.ICommand) (err error)
		ExecOutput(command shell.ICommand) (output []byte, err error)
	}

	IConfigService interface {
		GetConfig() (cfg config.Config, err error)
		UpdateConfigWithTx(ctx context.Context, tx *activity.Transaction, cfg config.Config, updateFuncs ...config.UpdateOption) (err error)
	}

	IWebsocketService interface {
		IsStarted() bool
		Start() (err error)
		Stop() (err error)
	}

	IHostnameService interface {
		UpdateHostnameWithTx(ctx context.Context, tx *activity.Transaction, hostname string) (err error)
	}

	ISystemdService interface {
		TryStartServiceWithTx(tx *activity.Transaction, serviceName string) (err error)
		TryStopService(serviceName string) (rollbacks rollback.Rollbacks, err error)
		TryStopServiceWithTx(tx *activity.Transaction, serviceName string) (err error)
	}

	IMQService interface {
		ActivateHandler(subject string) (err error)
		DeactivateHandler(subject string) (err error)

		Request(subject string, message any, timeout time.Duration, optionFuncs ...mq.RequestOption) (response *nats.Msg, err error)
		ChanSubscribe(subject string, ch chan *nats.Msg) (sub *nats.Subscription, err error)
	}

	IFirstPortService interface {
		SetupStaticWithTx(ctx context.Context, tx *activity.Transaction) (err error)
		ClearStatic() (rollbacks rollback.Rollbacks, err error)
	}

	IDeviceInitService interface {
		WaitFirstInit(tx *activity.Transaction) <-chan error
	}

	IMessagePublisher interface {
		PublishRequest(method, to string, body any, options ...wschat.RequestOptions) (response wschat.WebsocketMessage, err error)
		Reconnect()
		IsActive() bool
	}

	IPonyService interface {
		Pause()
		Resume()
	}

	IPingService interface {
		PingIP(options *ping.Options) (results ping.Results, err error)
	}

	IActivityService interface {
		ExecuteActivity(ctx context.Context, transaction *activity.Transaction, activityType, name string, payload any, options ...activity.ExecActivityOption) (err error)
		ExecuteFunc(transaction *activity.Transaction, fn, rlFn func() error) (err error)
		AddCheckPoint(ctx context.Context, transaction *activity.Transaction) (checkpointID string, err error)
		DeleteCheckPoint(ctx context.Context, transaction *activity.Transaction, checkpointID string) (err error)
	}

	INetInitService interface {
		GetSection(sectionName string) (section netinit.Section, err error)
	}

	INSLookupService interface {
		SyncHosts() (err error)
	}
)

type InitStateHandler struct {
	mqService       IMQService
	systemdService  ISystemdService
	activityService IActivityService
	shellService    IShellService
	agentEnv        environment.Agent

	mqSubjects []string
}

func NewInitStateHandler(mqService IMQService, systemdService ISystemdService, activityService IActivityService,
	shellService IShellService, agentEnv environment.Agent) *InitStateHandler {
	return &InitStateHandler{
		mqService:       mqService,
		systemdService:  systemdService,
		activityService: activityService,
		shellService:    shellService,
		agentEnv:        agentEnv,

		mqSubjects: []string{
			constants.MQAgentZTPFirstSetup,
			constants.MQAgentZTPSetPort,
			constants.MQAgentZTPDelPort,
			constants.MQAgentHubSetPort,
			constants.MQAgentHubDelPort,
			constants.MQAgentHubInit,
			constants.MQAgentReset,
		},
	}
}

func (h *InitStateHandler) StateID() entities.AppState {
	return entities.AppStateInit
}

func (h *InitStateHandler) ValidateTransition(fromState entities.AppState) (err error) {
	if fromState != entities.AppStateBoot &&
		fromState != entities.AppStateZTPSetup &&
		fromState != entities.AppStateReset {
		return fmt.Errorf("ValidateTransition: %w", errs.ErrTransitionNotSupported)
	}

	return nil
}

func (h *InitStateHandler) Handle(_ context.Context, tx *activity.Transaction, transition common.IStateTransition) (result common.StateHandleResult, err error) {
	if _, ok := transition.(*entities.OnAfterBoot); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: after boot transition")

		if h.checkMigrateFromOldVersion() {
			result.Transition = entities.NewOnMigrateFromOldVersion(
				h.agentEnv.DeviceID,
				h.agentEnv.EndPoint,
			)
			return result, nil
		}

		if err = h.restoreInitState(tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		return result, nil
	}

	if _, ok := transition.(*entities.OnInitFallback); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: init fallback transition")

		if err = h.restoreInitState(tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		return result, nil
	}

	if _, ok := transition.(*entities.OnZTPSetupInterrupted); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: ztp interrupted transition")

		if err = h.restoreInitState(tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		return result, nil
	}

	if _, ok := transition.(*entities.OnZTPSetupFinished); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: ztp setup finished transition")

		if err = h.subscribeQueueSubjects(tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		return result, nil
	}

	if _, ok := transition.(*entities.OnHubResetFinished); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: hub reset finished transition")

		if err = h.subscribeQueueSubjects(tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		return result, nil
	}

	return result, fmt.Errorf("Handle: %w", errs.ErrInvalidTransitionType)
}

func (h *InitStateHandler) OnExit(_ context.Context, tx *activity.Transaction, transition common.IStateTransition) (err error) {
	if _, ok := transition.(*entities.OnMigrateFromOldVersion); ok {
		// mq routes not activated
		return nil
	}

	if err = h.unsubscribeQueueSubjects(tx); err != nil {
		return fmt.Errorf("OnExit: %w", err)
	}

	return nil
}

// restoreInitState validates and restores init agent state.
func (h *InitStateHandler) restoreInitState(tx *activity.Transaction) (err error) {
	// activate mq routes
	if err = h.subscribeQueueSubjects(tx); err != nil {
		return fmt.Errorf("restoreInitState: %w", err)
	}

	// activate/deactivate daemons
	// bgp adapter
	if cmdErr := h.shellService.Exec(commands.NewDisableServiceCmd(constants.AdapterServiceName)); cmdErr != nil {
		log.Error().
			Err(cmdErr).
			Str("service name", constants.AdapterServiceName).
			Msg("restoreInitState: disable service error")
	}

	if cmdErr := h.shellService.Exec(commands.NewStopServiceCmd(constants.AdapterServiceName)); cmdErr != nil {
		log.Error().
			Err(cmdErr).
			Str("service name", constants.AdapterServiceName).
			Msg("restoreInitState: stop service error")
	}

	// update manager
	if cmdErr := h.shellService.Exec(commands.NewDisableServiceCmd(constants.UpdateManagerServiceName)); cmdErr != nil {
		log.Error().
			Err(cmdErr).
			Str("service name", constants.UpdateManagerServiceName).
			Msg("restoreInitState: disable service error")
	}

	if cmdErr := h.shellService.Exec(commands.NewStopServiceCmd(constants.UpdateManagerServiceName)); cmdErr != nil {
		log.Error().
			Err(cmdErr).
			Str("service name", constants.UpdateManagerServiceName).
			Msg("restoreInitState: stop service error")
	}

	if h.agentEnv.DeviceType != constants.DeviceTypeHub {
		// agent starter
		if err = h.systemdService.TryStartServiceWithTx(tx, constants.AgentStarterServiceName); err != nil {
			return fmt.Errorf("restoreInitState: %w", err)
		}

		// isc dhcp server
		if err = h.systemdService.TryStartServiceWithTx(tx, constants.ISCDHCPServiceName); err != nil {
			return fmt.Errorf("restoreInitState: %w", err)
		}
	}

	return nil
}

// checkMigrateFromOldVersion checks whether the agent needs to be switched to the active state after migration.
func (h *InitStateHandler) checkMigrateFromOldVersion() (needRestore bool) {
	deviceID := h.agentEnv.DeviceID
	if lo.IsEmpty(deviceID) {
		return false
	}

	if strings.ContainsAny(deviceID, "xX") {
		return false
	}

	return true
}

func (h *InitStateHandler) subscribeQueueSubjects(tx *activity.Transaction) (err error) {
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

func (h *InitStateHandler) unsubscribeQueueSubjects(tx *activity.Transaction) (err error) {
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
