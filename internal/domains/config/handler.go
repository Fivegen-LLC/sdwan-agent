package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

const (
	sendUpdateFinishedTimeout  = 5 * time.Second
	sendUpdateFinishedAttempts = 5
)

type (
	IMessagePublisher interface {
		IsActive() bool
		PublishRequest(method, to string, body any, options ...wschat.RequestOptions) (response wschat.WebsocketMessage, err error)
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
		Reconnect()
	}

	IAppStateService interface {
		Perform(transition common.IStateTransition) (err error)
	}

	Handler struct {
		publisher       IMessagePublisher
		appStateService IAppStateService
		configService   IConfigService
	}
)

func NewHandler(publisher IMessagePublisher, appStateService IAppStateService, configService IConfigService) *Handler {
	return &Handler{
		publisher:       publisher,
		appStateService: appStateService,
		configService:   configService,
	}
}

// UpdateWgPeer updates specified wireguard peer.
func (h *Handler) UpdateWgPeer(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.publisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = fmt.Errorf("%w: %w", sendErr, err)
			}
		}
	}()

	var requestBody struct {
		Peer config.WgPeer `json:"peer"`
	}
	if err = json.Unmarshal(request.Body, &requestBody); err != nil {
		return fmt.Errorf("UpdateWgPeer: %w", err)
	}

	log.Debug().
		Any("configs", requestBody).
		Msg("UpdateWgPeer: got wireguard peer to update")

	cfg, err := h.configService.GetConfig()
	if err != nil {
		return fmt.Errorf("UpdateWgPeer: %w", err)
	}

	// search for peer
	var found bool
LOOP:
	for i, wgCfg := range cfg.Wireguard.Configs {
		for j, peer := range wgCfg.Peers {
			if peer.PublicKey != requestBody.Peer.PublicKey {
				continue
			}

			cfg.Wireguard.Configs[i].Peers[j] = requestBody.Peer
			found = true
			break LOOP
		}
	}
	if !found {
		return fmt.Errorf("UpdateWgPeer: peer with key %s not found", requestBody.Peer.PublicKey)
	}

	if err = h.appStateService.Perform(
		entities.NewOnUpdateConfig(
			config.Config{
				Wireguard: &config.WireguardSection{
					Configs: cfg.Wireguard.Configs,
				},
			},
		),
	); err != nil {
		return fmt.Errorf("UpdateWgPeer: %w", err)
	}

	if err = h.publisher.PublishResponse(request, wschat.EmptyBody); err != nil {
		return fmt.Errorf("UpdateWgPeer: %w", err)
	}

	return nil
}

// UpdateAllConfigs refreshes all received configs.
func (h *Handler) UpdateAllConfigs(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.publisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = fmt.Errorf("%w: %w", sendErr, err)
			}
		}
	}()

	var requestBody struct {
		Configs struct {
			Wireguard []config.WgConfig      `json:"wgConfigs"`
			NetInit   entities.NetInitConfig `json:"netInit"`
			Pony      config.PonySection     `json:"pony"`
		} `json:"configs"`
	}

	if err = json.Unmarshal(request.Body, &requestBody); err != nil {
		return fmt.Errorf("UpdateAllConfigs: %w", err)
	}

	log.Debug().
		Any("configs", requestBody).
		Msg("UpdateAllConfigs: got configs to update")

	if err = h.publisher.PublishResponse(request, wschat.EmptyBody); err != nil {
		return fmt.Errorf("UpdateAllConfigs: %w", err)
	}

	go func() {
		updErr := h.appStateService.Perform(
			entities.NewOnUpdateConfig(
				config.Config{
					Wireguard: &config.WireguardSection{
						Configs: requestBody.Configs.Wireguard,
					},
					Port: &config.PortSection{
						PortConfigs: requestBody.Configs.NetInit.PortConfigs,
						PortMTUs:    requestBody.Configs.NetInit.PortMTUs,
					},
					WANProtection: &config.WANProtectionSection{
						PortNames:    requestBody.Configs.NetInit.PortNames,
						AllowedPorts: requestBody.Configs.NetInit.AllowedPorts,
					},
					Loopback: &config.LoopbackSection{
						Addresses: requestBody.Configs.NetInit.LoopbackAddresses,
					},
					IPRule: &config.IPRuleSection{
						IPRules: requestBody.Configs.NetInit.IPRules,
					},
					Pony: &requestBody.Configs.Pony,
					AdminState: &config.AdminStateSection{
						AdminStatePorts: requestBody.Configs.NetInit.AdminStatePorts,
					},
				},
			),
		)
		if updErr != nil {
			log.Error().
				Err(updErr).
				Msg("UpdateAllConfigs: update config error")
		}

		var (
			sErr     error
			attempts = sendUpdateFinishedAttempts
		)
		for {
			if sErr = h.sendUpdateConfigFinished(updErr); sErr == nil {
				// successfully send
				break
			}

			attempts--
			if attempts <= 0 {
				break
			}
		}

		if sErr != nil {
			log.Error().
				Err(sErr).
				Msg("UpdateAllConfigs: update config error")

			h.publisher.Reconnect()
		}
	}()

	return nil
}

func (h *Handler) sendUpdateConfigFinished(updErr error) (err error) {
	body := struct {
		ErrorMessage string `json:"errorMessage"`
	}{}
	if updErr != nil {
		body.ErrorMessage = updErr.Error()
	}

	resp, err := h.publisher.PublishRequest(constants.MethodUpdateAllConfigsFinished, constants.OrchestratorWSID, body,
		wschat.RequestOptions{
			Timeout: lo.ToPtr(sendUpdateFinishedTimeout),
		},
	)
	if err != nil {
		return fmt.Errorf("sendUpdateConfigFinished: %w", err)
	}

	if resp.IsErrorResponse() {
		return fmt.Errorf("sendUpdateConfigFinished: %w", resp.Error())
	}

	return nil
}
