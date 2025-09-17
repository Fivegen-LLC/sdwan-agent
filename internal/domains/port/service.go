package port

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell"
	"github.com/rs/zerolog/log"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/objects/bo"
	"github.com/Fivegen-LLC/sdwan-agent/internal/objects/dto"
)

const (
	dhcpLeaseDirectory = "/var/lib/dhcp"
)

type (
	IConfigService interface {
		GetConfig() (cfg config.Config, err error)
	}

	INetService interface {
		SearchGatewayInTables(portName string, tableIDs ...int) (gateway string, err error)
		SearchDNS(portName string) (dns []string, err error)
	}

	IPonyService interface {
		Pause()
		Resume()
	}

	IShellService interface {
		ExecOutput(command shell.ICommand) (output []byte, err error)
	}

	IAppStateService interface {
		Perform(transition common.IStateTransition) (err error)
	}
)

type Service struct {
	configService   IConfigService
	netService      INetService
	ponyService     IPonyService
	shellService    IShellService
	appStateService IAppStateService
}

func NewService(configService IConfigService, netService INetService, ponyService IPonyService,
	shellService IShellService, appStateService IAppStateService) *Service {
	return &Service{
		configService:   configService,
		netService:      netService,
		ponyService:     ponyService,
		shellService:    shellService,
		appStateService: appStateService,
	}
}

// GetPortRuntimeData returns port runtime data.
func (s *Service) GetPortRuntimeData() (ports bo.DevicePorts, err error) {
	// fetch system ports
	execCmd := exec.Command("ip", "--json", "address", "show")
	cmdOutput, err := execCmd.Output()
	if err != nil {
		return ports, fmt.Errorf("GetPortRuntimeData: %w", err)
	}

	var deviceInterfaces dto.LinuxInterfaces
	if err = json.Unmarshal(cmdOutput, &deviceInterfaces); err != nil {
		return ports, fmt.Errorf("GetPortRuntimeData: %w", err)
	}

	ports = bo.NewDevicePortsFromLinuxInterfaces(deviceInterfaces)
	s.fillGatewaysAndDNS(ports)

	return ports, nil
}

// GetPortConfigData returns port user configuration data.
func (s *Service) GetPortConfigData() (ports []config.PortConfig, err error) {
	runtimePorts, err := s.GetPortRuntimeData()
	if err != nil {
		return ports, fmt.Errorf("GetPortConfigData: %w", err)
	}

	agentConfig, err := s.configService.GetConfig()
	if err != nil {
		return ports, fmt.Errorf("GetPortConfigData: %w", err)
	}

	var portConfigs []config.PortConfig
	if agentConfig.Port != nil {
		portConfigs = agentConfig.Port.PortConfigs
	}

	var (
		configuredPorts = make(map[string]config.PortConfig)
		tagExistMap     = make(map[string]bool)
	)
	for _, portConfig := range portConfigs {
		configuredPorts[portConfig.Name] = portConfig
		if portConfig.IsTag {
			tagExistMap[portConfig.Tag.ParentPort] = true
		}
	}

	for _, port := range runtimePorts {
		if cfg, ok := configuredPorts[port.Name]; ok {
			ports = append(ports, cfg)
			continue
		}

		cfg := config.PortConfig{
			Name:  port.Name,
			Type:  constants.DevicePortTypeLAN,
			IsTag: false,
		}
		if tagExistMap[cfg.Name] {
			cfg.Type = constants.DevicePortTypeWAN
		}

		ports = append(ports, cfg)
	}

	return ports, nil
}

// FlushPort flushes port config.
func (s *Service) FlushPort(portName string) (err error) {
	// example: ip addr flush port1
	cmd := exec.Command("ip", "addr", "flush", portName)
	if err = execCommand(cmd); err != nil {
		return fmt.Errorf("FlushPort: %w", err)
	}

	return nil
}

// RenewDHCPLease resets dhcp leases for specified port.
func (s *Service) RenewDHCPLease(portName string) (err error) {
	s.ponyService.Pause()
	defer s.ponyService.Resume()

	cmd := exec.Command("dhclient", "-r", "-v", portName)
	if err = execCommand(cmd); err != nil {
		return fmt.Errorf("RenewDHCPLease: %w", err)
	}

	// path example: /var/lib/dhcp/dhclient.port5.leases
	leaseFile := fmt.Sprintf("dhclient.%s.leases", portName)
	leaseFile = filepath.Join(dhcpLeaseDirectory, leaseFile)
	if err = os.Remove(leaseFile); err != nil {
		log.Error().
			Err(err).
			Msg("RenewDHCPLease")
	}

	cmd = exec.Command("dhclient", "-v", portName)
	if err = execCommand(cmd); err != nil {
		return fmt.Errorf("RenewDHCPLease: %w", err)
	}

	return nil
}

func (s *Service) fillGatewaysAndDNS(ports bo.DevicePorts) {
	var err error
	for i, port := range ports {
		if len(port.IPs) == 0 {
			continue
		}

		if ports[i].Gateway, err = s.netService.SearchGatewayInTables(port.Name, 100, 101, 102, 103); err != nil {
			log.Warn().
				Err(err).
				Str("port name", port.Name).
				Msg("fillGatewaysAndDNS: search gateway error")
		}

		if ports[i].DNS, err = s.netService.SearchDNS(port.Name); err != nil {
			log.Warn().
				Err(err).
				Str("port name", port.Name).
				Msg("fillGatewaysAndDNS: search DNS error")
		}
	}
}

func execCommand(cmd *exec.Cmd) (err error) {
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Error().Err(err).Msgf("execCommand: exec error, output: %s", string(output))
		return fmt.Errorf("execCommand: %w", err)
	}

	return nil
}
