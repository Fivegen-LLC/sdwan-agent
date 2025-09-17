package constants

const (
	// in requests.
	MQAgentZTPFirstSetup   = "agent.ztp.first_setup"
	MQAgentZTPSetPort      = "agent.ztp.set_port"
	MQAgentZTPDelPort      = "agent.ztp.del_port"
	MQAgentGetConfig       = "agent.get_config"
	MQAgentRebuildServices = "agent.rebuild_services"
	MQAgentInstallFinished = "agent.install_finished"
	MQAgentReset           = "agent.reset"
	MQAgentHubSetPort      = "agent.hub.set_port"
	MQAgentHubDelPort      = "agent.hub.del_port"
	MQAgentHubListPorts    = "agent.hub.list_ports"
	MQAgentHubInit         = "agent.hub.init"
	MQAgentDebugDumpHeap   = "agent.debug.dump_heap"

	// out requests.
	MQUpdateManagerDownload    = "update_manager.download"
	MQUpdateManagerInstall     = "update_manager.install"
	MQUpdateManagerGetVersions = "update_manager.get_versions"
)
