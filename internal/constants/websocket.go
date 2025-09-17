package constants

import (
	"time"
)

const (
	// in requests.
	MethodCommand                = "command"
	MethodFetchPorts             = "fetch_ports"
	MethodFetchPortConfigs       = "fetch_port_configs"
	MethodFetchTunnelStates      = "fetch_tunnel_states"
	MethodUpdateAllConfigs       = "update_all_configs"
	MethodUpdateWgPeer           = "update_wg_peer"
	MethodInitDevice             = "init_device"
	MethodListFlowRoutes         = "list_flow_routes"
	MethodL3UpdateConfig         = "l3_update_config"
	MethodISBUpdateConfig        = "isb_update_config"
	MethodTrunkUpdateConfig      = "trunk_update_config"
	MethodServiceUpdateConfig    = "service_update_config"
	MethodPortFlush              = "method_port_flush"
	MethodPortRenewDHCPLease     = "method_port_renew_dhcp_lease"
	MethodExecDeviceAction       = "exec_device_action"
	MethodFlushMACs              = "flush_macs"
	MethodGetAgentState          = "get_agent_state"
	MethodResetBGPPeer           = "reset_bgp_peer"
	MethodFetchBGPPeer           = "fetch_bgp_stats"
	MethodFetchDHCPLeases        = "fetch_dhcp_leases"
	MethodSetupOVSManager        = "setup_ovs_manager"
	MethodListFWFlowRules        = "list_fw_flow_rules"
	MethodDownloadDevicePackages = "download_device_packages"
	MethodInstallDevicePackages  = "install_device_packages"
	MethodGetPackagesVersions    = "get_packages_versions"
	MethodLTEFetchStats          = "lte_fetch_stats"
	MethodLTEResetModem          = "lte_reset_modem"

	// out requests.
	MethodUplinkStateChanged            = "uplink_state_changed"
	MethodInitDeviceFinished            = "init_device_finished"
	MethodUpdateAllConfigsFinished      = "update_all_configs_finished"
	MethodInstallDevicePackagesFinished = "install_device_packages_finished"
)

const (
	WSPingPeriod = 4 * time.Second
	WSPongWait   = 6 * time.Second
)
