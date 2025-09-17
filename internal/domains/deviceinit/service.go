package deviceinit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/observable"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/ping"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/rs/zerolog/log"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

const (
	checkWSConnectionAttempts = 10
	checkWSConnectionInterval = 2 * time.Second
)

type (
	IMessagePublisher interface {
		IsActive() bool
		PublishRequest(method, to string, body any, options ...wschat.RequestOptions) (response wschat.WebsocketMessage, err error)
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
		Reconnect()
	}

	IHostnameService interface {
		UpdateHostnameWithTx(ctx context.Context, tx *activity.Transaction, hostname string) (err error)
	}

	IConfigService interface {
		GetConfig() (cfg config.Config, err error)
		UpdateConfigWithTx(ctx context.Context, tx *activity.Transaction, cfg config.Config, updateFuncs ...config.UpdateOption) (err error)
	}

	IGrafanaService interface {
		ConfigureAgent(serverAddr string) (err error)
	}

	IPonyService interface {
		Pause()
		Resume()
	}

	IUpdateManagerService interface {
		SetAptSource(aptSource string) (err error)
	}

	IPingService interface {
		PingIP(options *ping.Options) (results ping.Results, err error)
	}

	IActivityService interface {
		StartTransaction(ctx context.Context, name string, options ...activity.TransactionOption) (transaction *activity.Transaction, err error)
		FinishTransaction(ctx context.Context, transaction *activity.Transaction, execErr error) (err error)
	}
)

type Service struct {
	messagePublisher     IMessagePublisher
	hostnameService      IHostnameService
	configService        IConfigService
	grafanaService       IGrafanaService
	ponyService          IPonyService
	updateManagerService IUpdateManagerService
	pingService          IPingService
	activityService      IActivityService
	deviceType           string

	firstInitData     *entities.FirstInitData
	waitFirstInitChan chan error
	isInitializing    *atomic.Bool

	// events
	initStarted  *observable.Observable[entities.EmptyData]
	initFinished *observable.Observable[entities.InitFinishedData]

	mx sync.RWMutex
}

func NewService(messagePublisher IMessagePublisher, hostnameService IHostnameService,
	configService IConfigService, grafanaService IGrafanaService, ponyService IPonyService,
	updateManagerService IUpdateManagerService, pingService IPingService,
	activityService IActivityService, deviceType string) *Service {
	isInitializing := new(atomic.Bool)
	isInitializing.Store(false)
	return &Service{
		messagePublisher:     messagePublisher,
		hostnameService:      hostnameService,
		configService:        configService,
		grafanaService:       grafanaService,
		ponyService:          ponyService,
		updateManagerService: updateManagerService,
		pingService:          pingService,
		activityService:      activityService,
		deviceType:           deviceType,

		isInitializing: isInitializing,

		// events
		initStarted:  observable.NewObservable[entities.EmptyData](),
		initFinished: observable.NewObservable[entities.InitFinishedData](),
	}
}

func (s *Service) InitStarted() *observable.Observable[entities.EmptyData] {
	return s.initStarted
}

func (s *Service) InitFinished() *observable.Observable[entities.InitFinishedData] {
	return s.initFinished
}

func (s *Service) IsInitializing() bool {
	return s.isInitializing.Load()
}

func (s *Service) WaitFirstInit(tx *activity.Transaction) <-chan error {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.firstInitData != nil {
		errChan := make(chan error, 1)
		errChan <- errors.New("WaitFirstInit: already waiting first init")
		return errChan
	}

	s.firstInitData = entities.NewFirstInitData(tx)
	s.waitFirstInitChan = make(chan error)
	return s.waitFirstInitChan
}

// InitDevice initializes device with specified config.
func (s *Service) InitDevice(initConfig entities.InitConfig) (err error) {
	s.mx.Lock()
	defer s.mx.Unlock()

	var (
		ctx = context.Background()
		tx  *activity.Transaction
	)
	if s.firstInitData != nil {
		tx = s.firstInitData.Tx()
	} else {
		tx, err = s.activityService.StartTransaction(ctx, "init device transaction",
			activity.NewRollbackStrategyOption(activity.RollbackStrategySkipOnFail),
		)
		if err != nil {
			return fmt.Errorf("InitDevice: %w", err)
		}

		defer func() {
			err = s.activityService.FinishTransaction(ctx, tx, err)
		}()
	}

	defer func() {
		if s.firstInitData == nil {
			return
		}

		s.waitFirstInitChan <- err
		close(s.waitFirstInitChan)
		s.firstInitData = nil
		s.waitFirstInitChan = nil
	}()

	if s.isInitializing.Load() {
		return fmt.Errorf("InitDevice: device already initializing")
	}

	s.isInitializing.Store(true)
	defer s.isInitializing.Store(false)

	s.initStarted.Emit(entities.NewEmptyData())
	defer func() {
		s.initFinished.Emit(entities.NewInitFinishedData(err))
	}()

	s.ponyService.Pause()
	defer s.ponyService.Resume()

	oldConfig, err := s.configService.GetConfig()
	if err != nil {
		return fmt.Errorf("InitDevice: %w", err)
	}

	var (
		app            = *oldConfig.App
		deviceSerial   = app.SerialNumber
		orchTunnelAddr = strings.Split(initConfig.OFControllerAddr, ":")[0]
	)
	app.OrchestratorTunnelAddr = orchTunnelAddr
	app.OrchestratorAddrs = defineNewAddrs(initConfig.OrchestratorAddrs, app.OrchestratorAddrs)
	if err = s.configService.UpdateConfigWithTx(
		ctx, tx,
		config.Config{
			App: &app,
		},
	); err != nil {
		return fmt.Errorf("InitDevice: %w", err)
	}

	if err = s.hostnameService.UpdateHostnameWithTx(ctx, tx, deviceSerial); err != nil {
		return fmt.Errorf("InitDevice: %w", err)
	}

	if err = s.updateManagerService.SetAptSource(initConfig.AptSource); err != nil {
		return fmt.Errorf("InitDevice: %w", err)
	}

	// install rules
	if err = s.configService.UpdateConfigWithTx(
		ctx, tx,
		config.Config{
			Wireguard: &config.WireguardSection{
				Configs: initConfig.Wireguard,
			},
			Port: &config.PortSection{
				PortConfigs: initConfig.NetInit.PortConfigs,
				PortMTUs:    initConfig.NetInit.PortMTUs,
			},
			WANProtection: &config.WANProtectionSection{
				PortNames:    initConfig.NetInit.PortNames,
				AllowedPorts: initConfig.NetInit.AllowedPorts,
			},
			Loopback: &config.LoopbackSection{
				Addresses: initConfig.NetInit.LoopbackAddresses,
			},
			IPRule: &config.IPRuleSection{
				IPRules: initConfig.NetInit.IPRules,
			},
			Pony: &initConfig.Pony,
			AdminState: &config.AdminStateSection{
				AdminStatePorts: initConfig.NetInit.AdminStatePorts,
			},
		},
	); err != nil {
		return fmt.Errorf("InitDevice: %w", err)
	}

	if err = s.grafanaService.ConfigureAgent(fmt.Sprintf("http://%s", orchTunnelAddr)); err != nil {
		return fmt.Errorf("InitDevice: %w", err)
	}

	// check connection to hubs via tunnels
	if s.deviceType == constants.DeviceTypeCPE {
		if err = s.checkHubTunnels(initConfig.Pony); err != nil {
			return fmt.Errorf("InitDevice: %w", err)
		}
	}

	// check connection via websocket
	if s.deviceType == constants.DeviceTypeCPE || s.deviceType == constants.DeviceTypeHub {
		if err = s.checkWebsocketConnection(ctx); err != nil {
			return fmt.Errorf("InitDevice: %w", err)
		}
	}

	// install services
	if err = s.configService.UpdateConfigWithTx(
		ctx, tx,
		config.Config{
			Trunk:  &initConfig.Services.Trunk,
			L3:     &initConfig.Services.L3,
			ISB:    &initConfig.Services.ISB,
			Bridge: &initConfig.Services.Bridge,
			P2P:    &initConfig.Services.P2P,
			FW:     &initConfig.Services.FW,
		}); err != nil {
		return fmt.Errorf("InitDevice: %w", err)
	}

	return nil
}

func (s *Service) checkHubTunnels(ponyCfg config.PonySection) (err error) {
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
				WithThreshold(time.Second).
				WithInterruptWhenSucceed(true)

			results, err := s.pingService.PingIP(pingOptions)
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

func defineNewAddrs(newAddrs, oldAddrs []string) []string {
	if len(newAddrs) == 0 {
		return oldAddrs
	}

	return newAddrs
}

func (s *Service) checkWebsocketConnection(ctx context.Context) (err error) {
	log.Info().
		Msg("checkWebsocketConnection: check websocket connection")

	attempts := checkWSConnectionAttempts
	for {
		select {
		case <-ctx.Done():
			log.Error().
				Msg("checkWebsocketConnection: context canceled")
			return nil

		case <-time.After(checkWSConnectionInterval):
			if s.messagePublisher.IsActive() {
				log.Info().
					Msg("checkWebsocketConnection: websocket connection is active")

				return nil
			}

			attempts--
			if attempts <= 0 {
				return fmt.Errorf("checkWebsocketConnection: websocket connection is not available")
			}
		}
	}
}
