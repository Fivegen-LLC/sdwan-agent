package main

import (
	"github.com/nats-io/nats.go"

	"github.com/Fivegen-LLC/sdwan-agent/infrastructure"
	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/websocket"
)

func getWebsocketRoutes(injector infrastructure.IInjector) map[string]websocket.WsHandler {
	cmdHandler := injector.InjectCmdHandler()
	ponyHandler := injector.InjectPonyHandler()
	isbHandler := injector.InjectISBHandler()
	portHandler := injector.InjectPortHandler()
	configHandler := injector.InjectConfigHandler()
	deviceInitHandler := injector.InjectDeviceInitHandler()
	l3Handler := injector.InjectL3Handler()
	trunkHandler := injector.InjectTrunkHandler()
	serviceHandler := injector.InjectServiceHandler()
	deviceActionHandler := injector.InjectDeviceActionHandler()
	dhcpHandler := injector.InjectDHCPHandler()
	ovsHandler := injector.InjectOVSHandler()
	fwHandler := injector.InjectFWHandler()
	appStateHandler := injector.InjectAppStateWSHandler()
	updateManagerHandler := injector.InjectUpdateManagerHandler()
	lteHandler := injector.InjectLTEHandler()

	return map[string]websocket.WsHandler{
		constants.MethodCommand:                cmdHandler.ExecCommand,
		constants.MethodUpdateAllConfigs:       configHandler.UpdateAllConfigs,
		constants.MethodUpdateWgPeer:           configHandler.UpdateWgPeer,
		constants.MethodFetchPorts:             portHandler.FetchPorts,
		constants.MethodFetchPortConfigs:       portHandler.FetchPortConfigs,
		constants.MethodFetchTunnelStates:      ponyHandler.FetchTunnelStates,
		constants.MethodISBUpdateConfig:        isbHandler.UpdateConfig,
		constants.MethodTrunkUpdateConfig:      trunkHandler.UpdateConfig,
		constants.MethodInitDevice:             deviceInitHandler.InitDevice,
		constants.MethodListFlowRoutes:         l3Handler.GetFlowRoutes,
		constants.MethodL3UpdateConfig:         l3Handler.UpdateConfig,
		constants.MethodServiceUpdateConfig:    serviceHandler.UpdateConfig,
		constants.MethodPortFlush:              portHandler.FlushPort,
		constants.MethodPortRenewDHCPLease:     portHandler.RenewDHCPLease,
		constants.MethodExecDeviceAction:       deviceActionHandler.ExecDeviceAction,
		constants.MethodFlushMACs:              cmdHandler.FlushMACs,
		constants.MethodGetAgentState:          appStateHandler.GetActiveState,
		constants.MethodResetBGPPeer:           l3Handler.ResetBGPPeer,
		constants.MethodFetchBGPPeer:           l3Handler.FetchBGPStats,
		constants.MethodFetchDHCPLeases:        dhcpHandler.FetchDHCPLeases,
		constants.MethodSetupOVSManager:        ovsHandler.SetupOVSManager,
		constants.MethodListFWFlowRules:        fwHandler.GetFWFlowRules,
		constants.MethodDownloadDevicePackages: updateManagerHandler.DownloadDevicePackages,
		constants.MethodInstallDevicePackages:  updateManagerHandler.InstallDevicePackages,
		constants.MethodGetPackagesVersions:    updateManagerHandler.GetPackagesVersions,
		constants.MethodLTEFetchStats:          lteHandler.FetchStats,
		constants.MethodLTEResetModem:          lteHandler.ResetModem,
	}
}

func getMQRoutes(injector infrastructure.IInjector) map[string]func(m *nats.Msg) (resp any) {
	ztpMQHandler := injector.InjectZTPMQHandler()
	configMQHandler := injector.InjectConfigMQHandler()
	deviceActionMQHandler := injector.InjectDeviceActionMQHandler()
	hubMQHandler := injector.InjectHubMQHandler()
	debugMQHandler := injector.InjectDebugMQHandler()

	return map[string]func(m *nats.Msg) (resp any){
		constants.MQAgentZTPFirstSetup:   ztpMQHandler.RunFirstSetup,
		constants.MQAgentZTPSetPort:      ztpMQHandler.SetPort,
		constants.MQAgentZTPDelPort:      ztpMQHandler.DeletePort,
		constants.MQAgentGetConfig:       configMQHandler.GetConfig,
		constants.MQAgentRebuildServices: configMQHandler.RebuildServices,
		constants.MQAgentReset:           deviceActionMQHandler.Reset,
		constants.MQAgentHubSetPort:      hubMQHandler.SetPort,
		constants.MQAgentHubDelPort:      hubMQHandler.DeletePort,
		constants.MQAgentHubListPorts:    hubMQHandler.ListPorts,
		constants.MQAgentHubInit:         hubMQHandler.Init,
		constants.MQAgentDebugDumpHeap:   debugMQHandler.DumpHeap,
	}
}
