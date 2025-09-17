package pony

import (
	"fmt"
	"net/http"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
)

type (
	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
	}

	IConfigService interface {
		GetConfig() (cfg config.Config, err error)
	}

	Handler struct {
		publisher     IMessagePublisher
		configService IConfigService
	}
)

func NewHandler(publisher IMessagePublisher, configService IConfigService) *Handler {
	return &Handler{
		publisher:     publisher,
		configService: configService,
	}
}

// FetchTunnelStates returns actual tunnel states.
func (h *Handler) FetchTunnelStates(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.publisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = fmt.Errorf("%w: %w", sendErr, err)
			}
		}
	}()

	cfg, err := h.configService.GetConfig()
	if err != nil {
		return fmt.Errorf("FetchTunnelStates: %w", err)
	}

	if cfg.Pony == nil {
		return fmt.Errorf("FetchTunnelStates: pony configuration not set")
	}

	body := make(clusters, 0, len(cfg.Pony.Clusters))
	for _, c := range cfg.Pony.Clusters {
		states := make([]tunnelState, 0, len(c.Uplinks))
		for _, uplink := range c.Uplinks {
			state := c.State.LocalStates[uplink.MonitorAddr]
			states = append(states, tunnelState{
				Tunnel:    uplink.MonitorAddr,
				IsActive:  state,
				HubSerial: uplink.HubSerial,
				TableID:   uplink.TableID,
			})
		}

		body = append(body, cluster{
			Network: c.Network,
			States:  states,
		})
	}

	if err = h.publisher.PublishResponse(request, body); err != nil {
		return fmt.Errorf("FetchTunnelStates: %w", err)
	}

	return nil
}
