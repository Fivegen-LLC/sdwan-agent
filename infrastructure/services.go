package infrastructure

import (
	"sync"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity/adapter/adbadger"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity/handlers/actbadger"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity/handlers/actcmd"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity/handlers/actfile"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity/handlers/actnetinit"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/cmd"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/cmdbuf"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/cfgactivity/actbgp"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/cfgactivity/actwg"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/iprule"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/loopback"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/migration"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/ponycfg"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/portcfg"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/portcfg/adminstate"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/portcfg/portmtucfg"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/service/bgp"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/service/bridge"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/service/common"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/service/dhcp"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/service/fw"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/service/isb"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/service/l3"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/service/p2p"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/service/trunk"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/wanprotection"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config/wireguard"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/mq"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/net"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/netinit"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/ping"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/pony"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/pony/ponyroute"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/systemd"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat/wsclient"
	"github.com/nats-io/nats.go"

	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/handlers"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/connection"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/deviceinit"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/discovery"
	dMonitoring "github.com/Fivegen-LLC/sdwan-agent/internal/domains/discovery/monitoring"

	dClient "github.com/Fivegen-LLC/sdwan-agent/internal/domains/discovery/httpclient"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/dumpstat"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/firstport"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/grafana"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/hostname"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/hub"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/lte"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/nslookup"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/ovs"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/pony/ponyevent"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/port"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/updatemanager"
	ws "github.com/Fivegen-LLC/sdwan-agent/internal/domains/websocket"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
)

var (
	messagePublisher     *wsclient.MessagePublisher
	messagePublisherOnce sync.Once
)

func (k *Kernel) InjectMessagePublisher() *wsclient.MessagePublisher {
	messagePublisherOnce.Do(func() {
		messagePublisher = wsclient.NewMessagePublisher(
			k.InjectConnectionFactory(),
		)
	})

	return messagePublisher
}

var (
	connectionFactory     *connection.Factory
	connectionFactoryOnce sync.Once
)

func (k *Kernel) InjectConnectionFactory() *connection.Factory {
	connectionFactoryOnce.Do(func() {
		connectionFactory = connection.NewFactory(
			k.InjectConfigService(),
			k.InjectDiscoveryService(),
		)
	})

	return connectionFactory
}

var (
	portService     *port.Service
	portServiceOnce sync.Once
)

func (k *Kernel) InjectPortService() *port.Service {
	portServiceOnce.Do(func() {
		portService = port.NewService(
			k.InjectConfigService(),
			k.InjectNetService(),
			k.InjectPonyService(),
			k.InjectShellService(),
			k.InjectAppStateService(),
		)
	})

	return portService
}

var (
	netInitService     *netinit.Service
	netInitServiceOnce sync.Once
)

func (k *Kernel) InjectNetInitService() *netinit.Service {
	netInitServiceOnce.Do(func() {
		netInitService = netinit.NewService(
			k.InjectCmdService(),
			constants.NetInitPath,
			constants.CLIExtExecutable,
			k.env.Agent.IsDebug(),
		)
	})

	return netInitService
}

var (
	cmdService     *cmd.Service
	cmdServiceOnce sync.Once
)

func (k *Kernel) InjectCmdService() *cmd.Service {
	cmdServiceOnce.Do(func() {
		cmdService = cmd.NewService()
	})

	return cmdService
}

var (
	shellService     *shell.Service
	shellServiceOnce sync.Once
)

func (k *Kernel) InjectShellService() *shell.Service {
	shellServiceOnce.Do(func() {
		shellService = shell.NewService()
	})

	return shellService
}

var (
	ponyService     *pony.Service
	ponyServiceOnce sync.Once
)

func (k *Kernel) InjectPonyService() *pony.Service {
	ponyServiceOnce.Do(func() {
		ponyService = pony.NewService(
			k.InjectConfigService(),
			k.InjectConnectionService(),
			k.InjectPonyRouteService(),
		)
	})

	return ponyService
}

var (
	ponyRouteService     *ponyroute.Service
	ponyRouteServiceOnce sync.Once
)

func (k *Kernel) InjectPonyRouteService() *ponyroute.Service {
	ponyRouteServiceOnce.Do(func() {
		ponyRouteService = ponyroute.NewService(
			k.InjectConfigService(),
			k.InjectCmdService(),
		)
	})

	return ponyRouteService
}

var (
	connectionService     *connection.Service
	connectionServiceOnce sync.Once
)

func (k *Kernel) InjectConnectionService() *connection.Service {
	connectionServiceOnce.Do(func() {
		connectionService = connection.NewService(
			k.InjectMessagePublisher(),
		)
	})

	return connectionService
}

var (
	ovsService     *ovs.Service
	ovsServiceOnce sync.Once
)

func (k *Kernel) InjectOVSService() *ovs.Service {
	ovsServiceOnce.Do(func() {
		ovsService = ovs.NewService()
	})

	return ovsService
}

var (
	hostnameService     *hostname.Service
	hostnameServiceOnce sync.Once
)

func (k *Kernel) InjectHostnameService() *hostname.Service {
	hostnameServiceOnce.Do(func() {
		hostnameService = hostname.NewService(
			k.InjectShellService(),
			k.InjectActivityService(),
			constants.CLIExtExecutable,
		)
	})

	return hostnameService
}

var (
	deviceInitService     *deviceinit.Service
	deviceInitServiceOnce sync.Once
)

func (k *Kernel) InjectDeviceInitService() *deviceinit.Service {
	deviceInitServiceOnce.Do(func() {
		deviceInitService = deviceinit.NewService(
			k.InjectMessagePublisher(),
			k.InjectHostnameService(),
			k.InjectConfigService(),
			k.InjectGrafanaService(),
			k.InjectPonyService(),
			k.InjectUpdateManagerService(),
			k.InjectPingService(),
			k.InjectActivityService(),
			k.env.Agent.DeviceType,
		)
	})

	return deviceInitService
}

var (
	commandBufferService     *cmdbuf.Service
	commandBufferServiceOnce sync.Once
)

func (k *Kernel) InjectCommandBufferService() *cmdbuf.Service {
	commandBufferServiceOnce.Do(func() {
		commandBufferService = cmdbuf.NewService()
	})

	return commandBufferService
}

var (
	grafanaService     *grafana.Service
	grafanaServiceOnce sync.Once
)

func (k *Kernel) InjectGrafanaService() *grafana.Service {
	grafanaServiceOnce.Do(func() {
		grafanaService = grafana.NewService(
			constants.GrafanaConfigPath,
			constants.GrafanaMimirPort,
			constants.GrafanaLokiPort,
		)
	})

	return grafanaService
}

var (
	configService     *config.Service
	configServiceOnce sync.Once
)

func (k *Kernel) InjectConfigService() *config.Service {
	configServiceOnce.Do(func() {
		trunkService := trunk.NewService(
			k.InjectNetInitService(),
			k.InjectCmdService(),
			k.InjectActivityService(),
			constants.CLIExtExecutable,
		)

		l3Service := l3.NewService(
			k.InjectNetInitService(),
			k.InjectCmdService(),
			k.InjectBGPService(),
			k.InjectActivityService(),
			constants.CLIExtExecutable,
		)

		isbService := isb.NewService(
			k.InjectNetInitService(),
			k.InjectCmdService(),
			k.InjectShellService(),
			k.InjectActivityService(),
			constants.CLIExtExecutable,
			constants.SDWANProjectPath,
		)

		dhcpService := dhcp.NewService(
			k.InjectCmdService(),
			k.InjectNetInitService(),
			k.InjectActivityService(),
			constants.DHCPServiceName,
			constants.DHCPHelperServiceName,
			constants.DHCPDirectory,
			constants.CLIExtExecutable,
		)

		bridgeService := bridge.NewService(
			k.InjectNetInitService(),
			k.InjectCmdService(),
			k.InjectActivityService(),
			constants.CLIExtExecutable,
		)

		p2pService := p2p.NewService(
			k.InjectNetInitService(),
			k.InjectCmdService(),
			k.InjectActivityService(),
			constants.CLIExtExecutable,
		)

		fwService := fw.NewService(
			k.InjectNetInitService(),
			k.InjectCmdService(),
			k.InjectActivityService(),
			constants.CLIExtExecutable,
			constants.SDWANProjectPath,
		)

		aminState := adminstate.NewService(
			k.InjectNetInitService(),
			k.InjectCmdService(),
			k.InjectActivityService(),
		)

		mtuService := portmtucfg.NewService(
			k.InjectActivityService(),
		)

		configService = config.NewService(
			k.DB,
			[]config.IRuleGenerator{
				wireguard.NewService(
					k.InjectCmdService(),
					k.InjectWGConfigService(),
					k.InjectActivityService(),
				),
				loopback.NewService(
					k.InjectCmdService(),
					k.InjectNetInitService(),
					k.InjectActivityService(),
				),
				iprule.NewService(
					k.InjectCmdService(),
					k.InjectNetInitService(),
					k.InjectActivityService(),
				),
				wanprotection.NewService(
					k.InjectCmdService(),
					k.InjectNetInitService(),
					k.InjectActivityService(),
				),
				portcfg.NewService(
					k.InjectCmdService(),
					k.InjectNetInitService(),
					aminState,
					k.InjectActivityService(),
					mtuService,
					constants.CLIExtExecutable,
					k.env.Agent.IsDebug(),
				),
				common.NewService(
					// compare
					[]common.ICompareHandler{
						trunkService,
						p2pService,
						bridgeService,
						l3Service,
						isbService,
						fwService,
					},
					// merge
					[]common.IMergeHandler{
						trunkService,
						p2pService,
						bridgeService,
						l3Service,
						isbService,
						fwService,
					},
					// add (sort by priority!)
					[]common.IAddHandler{
						trunkService,
						p2pService,
						bridgeService,
						l3Service,
						dhcpService,
						isbService,
						fwService,
					},
					// delete (sort by priority!)
					[]common.IDeleteHandler{
						fwService,
						isbService,
						dhcpService,
						l3Service,
						bridgeService,
						p2pService,
						trunkService,
					},
				),
				ponycfg.NewService(
					k.InjectCmdService(),
					k.InjectNetInitService(),
					isbService,
					k.InjectActivityService(),
				),
				aminState,
			},
			k.InjectMigrationService(),
			k.InjectActivityService(),
			constants.SDWANProjectPath,
		)
	})

	return configService
}

var (
	bgpService     *bgp.Service
	bgpServiceOnce sync.Once
)

func (k *Kernel) InjectBGPService() *bgp.Service {
	bgpServiceOnce.Do(func() {
		bgpService = bgp.NewService(
			k.InjectMQService(),
			k.InjectActivityService(),
		)
	})

	return bgpService
}

var (
	migrationService     *migration.Service
	migrationServiceOnce sync.Once
)

func (k *Kernel) InjectMigrationService() *migration.Service {
	migrationServiceOnce.Do(func() {
		migrationService = migration.NewService(
			k.InjectNetInitService(),
		)
	})

	return migrationService
}

var (
	pingService     *ping.Service
	pingServiceOnce sync.Once
)

func (k *Kernel) InjectPingService() *ping.Service {
	pingServiceOnce.Do(func() {
		pingService = ping.NewService()
	})

	return pingService
}

var (
	updateManagerService     *updatemanager.Service
	updateManagerServiceOnce sync.Once
)

func (k *Kernel) InjectUpdateManagerService() *updatemanager.Service {
	updateManagerServiceOnce.Do(func() {
		updateManagerService = updatemanager.NewService(
			k.InjectAppStateService(),
			k.InjectMQService(),
			constants.CLIExtExecutable,
		)
	})

	return updateManagerService
}

var (
	appStateService     *appstate.StateService
	appStateServiceOnce sync.Once
)

func (k *Kernel) InjectAppStateService() *appstate.StateService {
	appStateServiceOnce.Do(func() {
		appStateService = appstate.NewService(
			k.InjectConfigService(),
			k.InjectActivityService(),
			entities.AppStateInit,
		)
	})

	return appStateService
}

func (k *Kernel) BuildAppStateService() {
	k.InjectAppStateService().SetStateHandlers(
		[]appstate.IStateHandler{
			handlers.NewBootStateHandler(),
			handlers.NewInitStateHandler(
				k.InjectMQService(),
				k.InjectSystemdService(),
				k.InjectActivityService(),
				k.InjectShellService(),
				k.env.Agent,
			),
			handlers.NewActiveStateHandler(
				k.InjectConfigService(),
				k.InjectSystemdService(),
				k.InjectMQService(),
				k.InjectWebsocketService(),
				k.InjectFirstPortService(),
				k.InjectDeviceInitService(),
				k.InjectActivityService(),
				k.env.Agent.DeviceType,
			),
			handlers.NewUpdateConfigStateHandler(
				k.InjectConfigService(),
				k.InjectPonyService(),
				k.InjectPingService(),
				k.InjectNSLookupService(),
				k.InjectWebsocketService(),
				k.env.Agent.DeviceType,
			),
			handlers.NewMaintenanceStateHandler(
				k.InjectMQService(),
				k.InjectConfigService(),
				k.InjectMessagePublisher(),
				k.InjectWebsocketService(),
				k.InjectActivityService(),
			),
			handlers.NewZTPSetupHandler(
				k.InjectConfigService(),
				k.InjectNetInitService(),
				k.InjectActivityService(),
				constants.CLIExtExecutable,
			),
			handlers.NewResetStateHandler(
				k.DB,
				k.InjectShellService(),
				k.InjectConfigService(),
				k.InjectWebsocketService(),
				k.InjectSystemdService(),
				k.InjectFirstPortService(),
				k.InjectHostnameService(),
				k.InjectActivityService(),
				k.env.Agent.DeviceType,
			),
		},
	)
}

var (
	systemdService     *systemd.Service
	systemdServiceOnce sync.Once
)

func (k *Kernel) InjectSystemdService() *systemd.Service {
	systemdServiceOnce.Do(func() {
		systemdService = systemd.NewService(
			k.InjectShellService(),
			k.InjectActivityService(),
		)
	})

	return systemdService
}

var (
	mqService     *mq.Service
	mqServiceOnce sync.Once
)

func (k *Kernel) InjectMQService() *mq.Service {
	mqServiceOnce.Do(func() {
		mqService = mq.NewService(nats.DefaultURL)
	})

	return mqService
}

var (
	websocketService     *ws.Service
	websocketServiceOnce sync.Once
)

func (k *Kernel) InjectWebsocketService() *ws.Service {
	websocketServiceOnce.Do(func() {
		websocketService = ws.NewService(
			k.InjectMessagePublisher(),
			k.InjectDumpStatService(),
			constants.WSPingPeriod,
		)
	})

	return websocketService
}

var (
	firstPortService     *firstport.Service
	firstPortServiceOnce sync.Once
)

func (k *Kernel) InjectFirstPortService() *firstport.Service {
	firstPortServiceOnce.Do(func() {
		firstPortService = firstport.NewService(
			k.InjectShellService(),
			k.InjectActivityService(),
		)
	})

	return firstPortService
}

var (
	hubService     *hub.Service
	hubServiceOnce sync.Once
)

func (k *Kernel) InjectHubService() *hub.Service {
	hubServiceOnce.Do(func() {
		hubService = hub.NewService(
			k.InjectAppStateService(),
			k.InjectNetService(),
			k.InjectConfigService(),
			k.env.Agent.DeviceType,
		)
	})

	return hubService
}

var (
	netService     *net.Service
	netServiceOnce sync.Once
)

func (k *Kernel) InjectNetService() *net.Service {
	netServiceOnce.Do(func() {
		netService = net.NewService(
			k.InjectShellService(),
		)
	})

	return netService
}

var (
	activityService     *activity.Service
	activityServiceOnce sync.Once
)

func (k *Kernel) InjectActivityService() *activity.Service {
	activityServiceOnce.Do(func() {
		activityService = activity.NewService(
			k.InjectStorageAdapter(),
			[]activity.IActivityHandler{
				actnetinit.NewSaveSectionHandler(
					k.InjectNetInitService(),
				),
				actcmd.NewExecCommandHandler(
					k.InjectShellService(),
				),
				actcmd.NewExecCommandsHandler(
					k.InjectShellService(),
				),
				actbgp.NewBGPUpdateConfigHandler(
					k.InjectMQService(),
				),
				actfile.NewFileHandler(),
				actfile.NewUpdateFileHandler(),
				actwg.NewWGConfigHandler(
					k.InjectWGConfigService(),
				),
				actwg.NewWGServiceHandler(
					k.InjectShellService(),
				),
				actbadger.NewUpdateConfigHandler(
					k.DB,
				),
			},
			activity.NewDeleteFinishedTransactionsOption(),
		)
	})

	return activityService
}

var (
	storageAdapter     *adbadger.Adapter
	storageAdapterOnce sync.Once
)

func (k *Kernel) InjectStorageAdapter() *adbadger.Adapter {
	storageAdapterOnce.Do(func() {
		storageAdapter = adbadger.NewAdapter(
			k.DB,
			constants.TxKey,
		)
	})

	return storageAdapter
}

var (
	wgConfigService     *wireguard.ConfigService
	wgConfigServiceOnce sync.Once
)

func (k *Kernel) InjectWGConfigService() *wireguard.ConfigService {
	wgConfigServiceOnce.Do(func() {
		wgConfigService = wireguard.NewConfigService(k.env.Agent.WgConfigRoot)
	})

	return wgConfigService
}

var (
	lteService     *lte.Service
	lteServiceOnce sync.Once
)

func (k *Kernel) InjectLTEService() *lte.Service {
	lteServiceOnce.Do(func() {
		lteService = lte.NewService(
			k.InjectShellService(),
		)
	})

	return lteService
}

var (
	dumpStatService     *dumpstat.Service
	dumpStatServiceOnce sync.Once
)

func (k *Kernel) InjectDumpStatService() *dumpstat.Service {
	dumpStatServiceOnce.Do(func() {
		dumpStatService = dumpstat.NewService(
			k.InjectConfigService(),
			k.InjectShellService(),
		)
	})

	return dumpStatService
}

var (
	ponyEventService     *ponyevent.Service
	ponyEventServiceOnce sync.Once
)

func (k *Kernel) InjectPonyEventService() *ponyevent.Service {
	ponyEventServiceOnce.Do(func() {
		ponyEventService = ponyevent.NewService(
			k.InjectPonyService(),
			k.InjectDumpStatService(),
		)
	})

	return ponyEventService
}

var (
	discoveryService     *discovery.Service
	discoveryServiceOnce sync.Once
)

func (k *Kernel) InjectDiscoveryService() *discovery.Service {
	discoveryServiceOnce.Do(func() {
		discoveryService = discovery.NewService(
			k.InjectDiscoveryHTTPClientService(),
		)
	})

	return discoveryService
}

var (
	discoveryMonitoringService     *dMonitoring.Service
	discoveryMonitoringServiceOnce sync.Once
)

func (k *Kernel) InjectDiscoveryMonitoringService() *dMonitoring.Service {
	discoveryMonitoringServiceOnce.Do(func() {
		discoveryMonitoringService = dMonitoring.NewService(
			k.InjectMessagePublisher(),
			k.InjectConfigService(),
			k.InjectDiscoveryService(),
		)
	})

	return discoveryMonitoringService
}

var (
	discoveryHTTPClientService     *dClient.Service
	discoveryHTTPClientServiceOnce sync.Once
)

func (k *Kernel) InjectDiscoveryHTTPClientService() *dClient.Service {
	discoveryHTTPClientServiceOnce.Do(func() {
		discoveryHTTPClientService = dClient.NewService()
	})

	return discoveryHTTPClientService
}

var (
	nsLookupService     *nslookup.Service
	nsLookupServiceOnce sync.Once
)

func (k *Kernel) InjectNSLookupService() *nslookup.Service {
	nsLookupServiceOnce.Do(func() {
		nsLookupService = nslookup.NewService(
			k.InjectConfigService(),
			nslookup.NewLookupIPService(),
			constants.EtcHostsPath,
		)
	})

	return nsLookupService
}
