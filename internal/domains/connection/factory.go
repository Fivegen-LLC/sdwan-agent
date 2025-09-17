package connection

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
)

type (
	IConfigService interface {
		GetConfig() (cfg config.Config, err error)
		UpdateConfig(ctx context.Context, cfg config.Config, updateFuncs ...config.UpdateOption) (err error)
	}

	IDiscoveryService interface {
		FetchPrimary(hosts []string) (primary string, err error)
	}
)

type Factory struct {
	configService    IConfigService
	discoveryService IDiscoveryService
}

func NewFactory(configService IConfigService, discoveryService IDiscoveryService) *Factory {
	return &Factory{
		configService:    configService,
		discoveryService: discoveryService,
	}
}

// SenderID returns sender id from device config.
func (f *Factory) SenderID() (senderID string, err error) {
	cfg, err := f.configService.GetConfig()
	if err != nil {
		return senderID, fmt.Errorf("SenderID: %w", err)
	}

	if cfg.App == nil {
		return senderID, fmt.Errorf("SenderID: device app configuration is missing")
	}

	if lo.IsEmpty(cfg.App.SerialNumber) {
		return senderID, fmt.Errorf("SenderID: serial number for device is not set")
	}

	return cfg.App.SerialNumber, nil
}

// BuildConn creates new websocket connection.
func (f *Factory) BuildConn() (conn wschat.IWebsocketConnection, err error) { //nolint:ireturn // skip interface check
	// load connection data from config
	cfg, err := f.configService.GetConfig()
	if err != nil {
		return conn, fmt.Errorf("BuildConn: %w", err)
	}

	if cfg.App == nil {
		return conn, fmt.Errorf("BuildConn: device app configuration is missing")
	}

	primaryHost, err := f.discoveryService.FetchPrimary(cfg.App.OrchestratorAddrs)
	if err != nil {
		return conn, fmt.Errorf("BuildConn: %w", err)
	}

	var (
		deviceID         = cfg.App.SerialNumber
		orchestratorAddr = primaryHost
		scheme           = "ws"
	)
	if lo.IsEmpty(deviceID) {
		return conn, fmt.Errorf("BuildConn: serial number for device is not set")
	}

	if lo.IsEmpty(orchestratorAddr) {
		return conn, fmt.Errorf("BuildConn: orchestrator address for device is not set")
	}

	if strings.HasPrefix(orchestratorAddr, "https") {
		scheme = "wss"
	}

	endpoint := strings.ReplaceAll(orchestratorAddr, "http://", "")
	endpoint = strings.ReplaceAll(endpoint, "https://", "")

	reqPath := fmt.Sprintf("/api/v1/ws/agents/%s", deviceID)
	wsURL := url.URL{
		Scheme: scheme,
		Host:   endpoint,
		Path:   reqPath,
	}

	// build connection
	websocket.DefaultDialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // we are using self-signed certificate
	wsConn, response, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		return conn, fmt.Errorf("BuildConn: %w", err)
	}
	defer response.Body.Close()

	wsConn.SetPongHandler(func(_ string) error {
		if err = wsConn.SetReadDeadline(time.Now().Add(constants.WSPongWait)); err != nil {
			log.Error().Msgf("BuildConn: set read deadline error: %s", err)
		}

		return nil
	})

	oldConfig, err := f.configService.GetConfig()
	if err != nil {
		return conn, fmt.Errorf("BuildConn: %w", err)
	}

	oldConfig.App.ActiveOrchestratorAddr = orchestratorAddr
	if err = f.configService.UpdateConfig(context.Background(),
		config.Config{
			App: oldConfig.App,
		}); err != nil {
		return conn, fmt.Errorf("BuildConn: %w", err)
	}

	return wsConn, nil
}
