package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity/handlers/actnetinit"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/netinit"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/netutils"
	"github.com/rs/zerolog/log"

	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

type ZTPSetupHandler struct {
	configService    IConfigService
	netInitService   INetInitService
	activityService  IActivityService
	cliExtExecutable string
}

func NewZTPSetupHandler(configService IConfigService, netInitService INetInitService, activityService IActivityService,
	cliExtExecutable string) *ZTPSetupHandler {
	return &ZTPSetupHandler{
		configService:    configService,
		netInitService:   netInitService,
		activityService:  activityService,
		cliExtExecutable: cliExtExecutable,
	}
}

func (h *ZTPSetupHandler) StateID() entities.AppState {
	return entities.AppStateZTPSetup
}

func (h *ZTPSetupHandler) ValidateTransition(fromState entities.AppState) (err error) {
	if fromState != entities.AppStateInit && fromState != entities.AppStateBoot {
		return fmt.Errorf("ValidateTransition: %w", errs.ErrTransitionNotSupported)
	}

	return nil
}

func (h *ZTPSetupHandler) Handle(ctx context.Context, tx *activity.Transaction, transition common.IStateTransition) (result common.StateHandleResult, err error) {
	if _, ok := transition.(*entities.OnAfterBoot); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: after boot transition")

		log.Error().
			Msg("Handle: setup ztp was interrupted")

		result.Transition = entities.NewOnZTPSetupInterrupted()
		return result, nil
	}

	if data, ok := transition.(*entities.OnZTPSetupConfig); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: ztp setup config transition")

		if err = h.setupZTPConfig(ctx, tx, data.Config); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		result.Transition = entities.NewOnZTPSetupFinished()
		return result, nil
	}

	if data, ok := transition.(*entities.OnHubSetPort); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: hub set port transition")

		if err = h.setHubPort(ctx, tx, data.PortConfig); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		result.Transition = entities.NewOnZTPSetupFinished()
		return result, nil
	}

	if _, ok := transition.(*entities.OnHubDeletePort); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: hub delete port transition")

		if err = h.deleteHubPort(ctx, tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		result.Transition = entities.NewOnZTPSetupFinished()
		return result, nil
	}

	return result, fmt.Errorf("Handle: %w", errs.ErrInvalidTransitionType)
}

func (h *ZTPSetupHandler) OnExit(_ context.Context, _ *activity.Transaction, _ common.IStateTransition) (err error) {
	return nil
}

func (h *ZTPSetupHandler) setupZTPConfig(ctx context.Context, tx *activity.Transaction, newCfg config.Config) (err error) {
	if err = h.configService.UpdateConfigWithTx(ctx, tx, newCfg); err != nil {
		return fmt.Errorf("setupZTPConfig: %w", err)
	}

	return nil
}

func (h *ZTPSetupHandler) setHubPort(ctx context.Context, tx *activity.Transaction, portConfig config.PortConfig) (err error) {
	oldSection, err := h.netInitService.GetSection(netinit.SectionPortConfig)
	if err != nil {
		return fmt.Errorf("setHubPort :%w", err)
	}

	if err = h.configService.UpdateConfigWithTx(
		ctx, tx,
		config.Config{
			Port: &config.PortSection{
				PortConfigs: []config.PortConfig{portConfig},
			},
		},
		config.NewSkipGeneratorsOption(true),
	); err != nil {
		return fmt.Errorf("setHubPort :%w", err)
	}

	// update net_init
	var (
		ipAddr     = portConfig.Wan.IPAddr
		subnetMask = portConfig.Wan.SubnetMask
		gateway    = portConfig.Wan.Gateway
		dns        = portConfig.Wan.DNS
	)
	portCmd, err := h.buildStaticAddCommand(portConfig.Name, ipAddr, subnetMask, gateway, dns, 100)
	if err != nil {
		return fmt.Errorf("setHubPort: %w", err)
	}

	newSection := netinit.NewSection(netinit.SectionPortConfig)
	if newSection, err = newSection.AppendCommand(portCmd); err != nil {
		return fmt.Errorf("setHubPort: %w", err)
	}

	if err = h.activityService.ExecuteActivity(ctx, tx, actnetinit.ActivitySaveSection, "update port config section",
		actnetinit.NewSaveSectionPayload(newSection, oldSection),
	); err != nil {
		return fmt.Errorf("setHubPort: %w", err)
	}

	return nil
}

func (h *ZTPSetupHandler) deleteHubPort(ctx context.Context, tx *activity.Transaction) (err error) {
	oldSection, err := h.netInitService.GetSection(netinit.SectionPortConfig)
	if err != nil {
		return fmt.Errorf("deleteHubPort :%w", err)
	}

	if err = h.configService.UpdateConfigWithTx(
		ctx, tx,
		config.Config{
			Port: &config.PortSection{},
		},
		config.NewSkipGeneratorsOption(true),
	); err != nil {
		return fmt.Errorf("deleteHubPort: %w", err)
	}

	// update net_init
	newSection := netinit.NewSection(netinit.SectionPortConfig)
	if err = h.activityService.ExecuteActivity(ctx, tx, actnetinit.ActivitySaveSection, "update port config section",
		actnetinit.NewSaveSectionPayload(newSection, oldSection),
	); err != nil {
		return fmt.Errorf("deleteHubPort: %w", err)
	}

	return nil
}

func (h *ZTPSetupHandler) buildStaticAddCommand(portName, ipAddr, subnetMask, gateway, dns string, tableIDs ...int) (command string, err error) {
	mask, err := netutils.ConvertIPMaskToInt(subnetMask)
	if err != nil {
		return command, fmt.Errorf("buildStaticAddCommand: %w", err)
	}

	// example: sdwan-cli-ext port static add -n port1 -i 192.168.10.23/24 -g 192.168.10.1 -t 100 -d 8.8.8.8
	var (
		sb         strings.Builder
		ipWithMask = fmt.Sprintf("%s/%d", ipAddr, mask)
	)
	sb.WriteString(fmt.Sprintf("%s port static add -n %s -i %s -g %s", h.cliExtExecutable, portName, ipWithMask, gateway))
	for _, tableID := range tableIDs {
		sb.WriteString(fmt.Sprintf(" -t %d", tableID))
	}

	if dns != "" {
		sb.WriteString(fmt.Sprintf(" -d %s", dns))
	}

	return sb.String(), nil
}
