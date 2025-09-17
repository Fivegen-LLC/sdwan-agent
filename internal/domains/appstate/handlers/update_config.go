package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/ping"
	"github.com/rs/zerolog/log"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

type UpdateConfigStateHandler struct {
	configService    IConfigService
	ponyService      IPonyService
	pingService      IPingService
	nsLookupService  INSLookupService
	websocketService IWebsocketService
	deviceType       string
}

func NewUpdateConfigStateHandler(configService IConfigService, ponyService IPonyService, pingService IPingService,
	nsLookupService INSLookupService, websocketService IWebsocketService, deviceType string) *UpdateConfigStateHandler {
	return &UpdateConfigStateHandler{
		configService:    configService,
		ponyService:      ponyService,
		pingService:      pingService,
		nsLookupService:  nsLookupService,
		websocketService: websocketService,
		deviceType:       deviceType,
	}
}

func (h *UpdateConfigStateHandler) StateID() entities.AppState {
	return entities.AppStateUpdateConfig
}

func (h *UpdateConfigStateHandler) ValidateTransition(fromState entities.AppState) (err error) {
	if fromState != entities.AppStateActive && fromState != entities.AppStateBoot {
		return fmt.Errorf("ValidateTransition: %w", errs.ErrTransitionNotSupported)
	}

	return nil
}

func (h *UpdateConfigStateHandler) Handle(ctx context.Context, tx *activity.Transaction, transition common.IStateTransition) (result common.StateHandleResult, err error) {
	if _, ok := transition.(*entities.OnAfterBoot); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: after boot transition")

		log.Error().
			Msg("Handle: update config was interrupted")

		result.Transition = entities.NewOnFallback()
		return result, nil
	}

	if data, ok := transition.(*entities.OnUpdateConfig); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: update config transition")

		if err = h.updateConfig(ctx, tx, data.Config); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		result.Transition = entities.NewOnUpdateConfigFinished()
		return result, nil
	}

	if _, ok := transition.(*entities.OnRebuildServices); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: rebuild services transition")

		if err = h.rebuildServices(ctx, tx); err != nil {
			return result, fmt.Errorf("Handle: %w", err)
		}

		result.Transition = entities.NewOnUpdateConfigFinished()
		return result, nil
	}

	return result, fmt.Errorf("Handle: %w", errs.ErrInvalidTransitionType)
}

func (h *UpdateConfigStateHandler) OnExit(_ context.Context, _ *activity.Transaction, _ common.IStateTransition) (err error) {
	return nil
}

func (h *UpdateConfigStateHandler) updateConfig(ctx context.Context, tx *activity.Transaction, newCfg config.Config) (err error) {
	oldCfg, err := h.configService.GetConfig()
	if err != nil {
		return fmt.Errorf("updateConfig: %w", err)
	}

	isPortConfigChanged := h.isPortConfigurationChanged(oldCfg, newCfg)
	if isPortConfigChanged {
		h.ponyService.Pause()
		if err = h.websocketService.Stop(); err != nil {
			log.Error().Err(err).Msg("updateConfig: stop publisher error")
		}
	}

	if err = h.configService.UpdateConfigWithTx(ctx, tx, newCfg); err != nil {
		if isPortConfigChanged {
			h.ponyService.Resume()
			if sErr := h.websocketService.Start(); sErr != nil {
				log.Error().Err(sErr).Msg("updateConfig: start publisher error")
			}
		}

		return fmt.Errorf("updateConfig: %w", err)
	}

	// sync hosts
	if newCfg.App != nil {
		if !oldCfg.App.Compare(newCfg.App) {
			if err = h.nsLookupService.SyncHosts(); err != nil {
				log.Error().Err(err).Msg("updateConfig: sync hosts error")
			}
		}
	}

	if !isPortConfigChanged {
		return nil
	}

	// try to ping orch tunnel
	defer func() {
		if sErr := h.websocketService.Start(); sErr != nil {
			log.Error().
				Err(sErr).
				Msg("updateConfig: start publisher error")
		}
	}()
	h.ponyService.Resume()

	// check connection to hubs via tunnels
	if h.deviceType == constants.DeviceTypeCPE && newCfg.Pony != nil {
		if err = h.checkHubTunnels(*newCfg.Pony); err != nil {
			return fmt.Errorf("updateConfig: %w", err)
		}
	}

	return nil
}

func (h *UpdateConfigStateHandler) rebuildServices(ctx context.Context, tx *activity.Transaction) (err error) {
	cfg, err := h.configService.GetConfig()
	if err != nil {
		return fmt.Errorf("rebuildServices: %w", err)
	}

	if err = h.configService.UpdateConfigWithTx(
		ctx, tx,
		config.Config{
			Trunk:  &config.TrunkSection{},
			P2P:    &config.P2PServiceSection{},
			Bridge: &config.BridgeServiceSection{},
			L3:     &config.L3ServiceSection{},
			ISB:    &config.ISBSection{},
			FW:     &config.FWSection{},
		},
	); err != nil {
		return fmt.Errorf("rebuildServices: %w", err)
	}

	if err = h.configService.UpdateConfigWithTx(
		ctx, tx,
		config.Config{
			Trunk:  cfg.Trunk,
			P2P:    cfg.P2P,
			Bridge: cfg.Bridge,
			L3:     cfg.L3,
			ISB:    cfg.ISB,
			FW:     cfg.FW,
		},
	); err != nil {
		return fmt.Errorf("rebuildServices: %w", err)
	}

	return nil
}

func (h *UpdateConfigStateHandler) isPortConfigurationChanged(oldCfg, newCfg config.Config) bool {
	if newCfg.Port != nil && !oldCfg.Port.Compare(newCfg.Port) {
		return true
	}

	if newCfg.AdminState != nil && !oldCfg.AdminState.Compare(newCfg.AdminState) {
		return true
	}

	return false
}

func (h *UpdateConfigStateHandler) checkHubTunnels(ponyCfg config.PonySection) (err error) {
	if len(ponyCfg.Clusters) == 0 {
		return nil
	}

	var (
		cluster             = ponyCfg.Clusters[0]
		anyActiveTunnelChan = make(chan struct{})
	)
	for _, uplink := range cluster.Uplinks {
		tunnelAddr := uplink.MonitorAddr
		log.Info().
			Str("tunnel address", tunnelAddr).
			Str("cluster", cluster.Network).
			Msg("checkHubTunnels: check tunnel address availability")

		go func(tunnelAddr string) {
			pingOptions := ping.NewOptions(tunnelAddr).
				WithAttempts(30).
				WithThreshold(5 * time.Second).
				WithInterruptWhenSucceed(true)

			results, err := h.pingService.PingIP(pingOptions)
			if err != nil {
				log.Warn().
					Str("tunnel address", tunnelAddr).
					Msg("checkHubTunnels: ping tunnel address error")

				return
			}

			if !results.IsLastSucceed() {
				log.Warn().
					Str("tunnel address", tunnelAddr).
					Msg("checkHubTunnels: tunnel address not available")

				return
			}

			log.Info().
				Str("tunnel address", tunnelAddr).
				Msg("checkHubTunnels: tunnel address active")

			anyActiveTunnelChan <- struct{}{}
		}(tunnelAddr)
	}

	select {
	case <-anyActiveTunnelChan:
		return nil

	case <-time.After(40 * time.Second):
		close(anyActiveTunnelChan)
		return fmt.Errorf("checkHubTunnels: all tunnels down")
	}
}
