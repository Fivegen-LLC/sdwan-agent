package handlers

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity/handlers/actcmd"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell/commands"
	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

const (
	defaultHostname = "cpe-default"
)

type ResetStateHandler struct {
	db               *badger.DB
	shellService     IShellService
	configService    IConfigService
	websocketService IWebsocketService
	systemdService   ISystemdService
	firstPortService IFirstPortService
	hostnameService  IHostnameService
	activityService  IActivityService
	deviceType       string
}

func NewResetStateHandler(db *badger.DB, shellService IShellService, configService IConfigService,
	websocketService IWebsocketService, systemdService ISystemdService, firstPortService IFirstPortService,
	hostnameService IHostnameService, activityService IActivityService, deviceType string) *ResetStateHandler {
	return &ResetStateHandler{
		db:               db,
		shellService:     shellService,
		configService:    configService,
		websocketService: websocketService,
		systemdService:   systemdService,
		firstPortService: firstPortService,
		hostnameService:  hostnameService,
		activityService:  activityService,
		deviceType:       deviceType,
	}
}

func (h *ResetStateHandler) StateID() entities.AppState {
	return entities.AppStateReset
}

func (h *ResetStateHandler) ValidateTransition(fromState entities.AppState) (err error) {
	if fromState != entities.AppStateBoot &&
		fromState != entities.AppStateInit &&
		fromState != entities.AppStateActive {
		return fmt.Errorf("ValidateTransition: %w", errs.ErrTransitionNotSupported)
	}

	return nil
}

func (h *ResetStateHandler) Handle(ctx context.Context, tx *activity.Transaction, transition common.IStateTransition) (result common.StateHandleResult, err error) {
	if _, ok := transition.(*entities.OnAfterBoot); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: after boot transition")

		log.Error().
			Msg("Handle: reset operation was interrupted")

		wasInit, err := h.wasInitState()
		if err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		if wasInit {
			result.Transition = entities.NewOnInitFallback()
			return result, nil
		}

		result.Transition = entities.NewOnFallback()
		return result, nil
	}

	if _, ok := transition.(*entities.OnReset); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: on reset transition")

		if h.deviceType == constants.DeviceTypeHub {
			if err = h.resetHub(ctx, tx); err != nil {
				return result, fmt.Errorf("Handle: %w", err)
			}

			// move to init state
			result.Transition = entities.NewOnHubResetFinished()
		} else {
			if err = h.reset(ctx, tx); err != nil {
				return result, fmt.Errorf("Handle: %w", err)
			}
		}

		return result, nil
	}

	return result, fmt.Errorf("Handle: %w", errs.ErrInvalidTransitionType)
}

func (h *ResetStateHandler) OnExit(_ context.Context, _ *activity.Transaction, _ common.IStateTransition) (err error) {
	return nil
}

// reset resets device to init state (ZTP stage).
func (h *ResetStateHandler) reset(ctx context.Context, tx *activity.Transaction) (err error) {
	// stop websocket
	if h.websocketService.IsStarted() {
		if err = h.activityService.ExecuteFunc(
			tx,
			h.websocketService.Stop,
			h.websocketService.Start,
		); err != nil {
			return fmt.Errorf("reset: %w", err)
		}
	}

	// reset hostname
	if err = h.hostnameService.UpdateHostnameWithTx(ctx, tx, defaultHostname); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// reset device config
	if err = h.configService.UpdateConfigWithTx(ctx, tx, config.EmptyConfig()); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// set addr 192.168.1.1
	if err = h.firstPortService.SetupStaticWithTx(ctx, tx); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// enable agent starter
	if err = h.activityService.ExecuteActivity(ctx, tx, actcmd.ActivityExecCommand, "enable agent starter",
		actcmd.NewExecCommandPayload(
			commands.NewEnableServiceCmd(constants.AgentStarterServiceName).String(),
			commands.NewDisableServiceCmd(constants.AgentStarterServiceName).String(),
		),
	); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// enable isc dhcp server
	if err = h.activityService.ExecuteActivity(ctx, tx, actcmd.ActivityExecCommand, "enable isc dhcp server",
		actcmd.NewExecCommandPayload(
			commands.NewEnableServiceCmd(constants.ISCDHCPServiceName).String(),
			commands.NewDisableServiceCmd(constants.ISCDHCPServiceName).String(),
		),
	); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// stop bgp adapter
	if err = h.systemdService.TryStopServiceWithTx(tx, constants.AdapterServiceName); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// stop update manager
	if err = h.systemdService.TryStopServiceWithTx(tx, constants.UpdateManagerServiceName); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	var checkpoinID string
	if checkpoinID, err = h.activityService.AddCheckPoint(ctx, tx); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// reset .env
	if err = h.tryResetEnv(); err != nil {
		log.Error().
			Err(err).
			Msg("reset: reset env error")
	}

	// close badger
	if err = h.db.Close(); err != nil {
		if delErr := h.activityService.DeleteCheckPoint(ctx, tx, checkpoinID); delErr != nil {
			log.Error().
				Err(delErr).
				Msg("reset: delete checkpoint error")
		}

		return fmt.Errorf("reset: %w", err)
	}

	// delete config
	if rmErr := os.RemoveAll(constants.AgentConfigPath); rmErr != nil && !os.IsNotExist(rmErr) {
		log.Error().
			Err(err).
			Msg("reset")
	}

	// reboot device
	go func() {
		if err := h.shellService.Exec(commands.NewRebootDeviceCmd()); err != nil {
			log.Error().
				Err(err).
				Msg("reset")
		}
	}()

	return nil
}

func (h *ResetStateHandler) resetHub(ctx context.Context, tx *activity.Transaction) (err error) {
	oldCfg, err := h.configService.GetConfig()
	if err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// stop websocket
	if h.websocketService.IsStarted() {
		if err = h.activityService.ExecuteFunc(
			tx,
			h.websocketService.Stop,
			h.websocketService.Start,
		); err != nil {
			return fmt.Errorf("reset: %w", err)
		}
	}

	// reset hostname
	if err = h.hostnameService.UpdateHostnameWithTx(ctx, tx, defaultHostname); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// reset device config (skip port section for hub)
	resetConfig := config.EmptyConfig()
	resetConfig.Port = oldCfg.Port
	if err = h.configService.UpdateConfigWithTx(ctx, tx, resetConfig); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// stop bgp adapter
	if err = h.systemdService.TryStopServiceWithTx(tx, constants.AdapterServiceName); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// stop update manager
	if err = h.systemdService.TryStopServiceWithTx(tx, constants.UpdateManagerServiceName); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// reset .env
	if err = h.tryResetEnv(); err != nil {
		log.Error().
			Err(err).
			Msg("reset: reset env error")
	}

	return nil
}

func (h *ResetStateHandler) tryResetEnv() (err error) {
	envFile := constants.AgentEnvPath
	stat, err := os.Stat(envFile)
	if err != nil {
		return fmt.Errorf("tryResetEnv: %w", err)
	}

	oldData, err := os.ReadFile(envFile)
	if err != nil {
		return fmt.Errorf("tryResetEnv: %w", err)
	}

	var (
		sb      strings.Builder
		scanner = bufio.NewScanner(bytes.NewReader(oldData))
	)
	for scanner.Scan() {
		line := scanner.Text()
		if lo.IsEmpty(line) {
			continue
		}

		left, _, found := strings.Cut(line, "=")
		if !found {
			sb.WriteString(line)
			sb.WriteByte('\n')
			continue
		}

		left = strings.TrimSpace(left)
		switch {
		case strings.HasPrefix(left, "AGENT_ENDPOINT"):
			sb.WriteString("AGENT_ENDPOINT=\"\"\n")

		case strings.HasPrefix(left, "AGENT_ID"):
			sb.WriteString("AGENT_ID=\"\"\n")

		default:
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}

	if err = scanner.Err(); err != nil {
		return fmt.Errorf("tryResetEnv: %w", err)
	}

	if err = os.WriteFile(envFile, []byte(sb.String()), stat.Mode().Perm()); err != nil {
		return fmt.Errorf("tryResetEnv: %w", err)
	}

	return nil
}

func (h *ResetStateHandler) wasInitState() (result bool, err error) {
	cfg, err := h.configService.GetConfig()
	if err != nil {
		return result, fmt.Errorf("wasInitState: %w", err)
	}

	return lo.IsEmpty(cfg.App.SerialNumber), nil
}
