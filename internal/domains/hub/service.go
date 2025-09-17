package hub

import (
	"fmt"
	"strings"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/net"
	"github.com/samber/lo"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

var (
	availablePortPrefixes = []string{"lan", "wan", "port", "ens", "enp", "wwan", "lte"}
)

type (
	IAppStateService interface {
		Perform(transition common.IStateTransition) (err error)
	}

	INetService interface {
		GetAllPorts() (ports net.Ports, err error)
		SearchIPAddrAndMask(portName string) (ipAddr, subnetMask string, err error)
		SearchGateway(portName string) (gateway string, err error)
		SearchDNS(portName string) (dns []string, err error)
	}

	IConfigService interface {
		GetConfig() (cfg config.Config, err error)
	}

	Service struct {
		appStateService IAppStateService
		netService      INetService
		configService   IConfigService
		deviceType      string
	}
)

func NewService(appStateService IAppStateService, netService INetService, configService IConfigService, deviceType string) *Service {
	return &Service{
		appStateService: appStateService,
		netService:      netService,
		configService:   configService,
		deviceType:      deviceType,
	}
}

// SetPort sets hub port as wan.
func (s *Service) SetPort(portName string) (err error) {
	if err = s.checkDeviceIsHub(); err != nil {
		return fmt.Errorf("SetPort: %w", err)
	}

	var (
		ipAddr     string
		subnetMask string
		gateway    string
	)
	// ip + mask
	if ipAddr, subnetMask, err = s.netService.SearchIPAddrAndMask(portName); err != nil {
		return fmt.Errorf("SetPort: %w", err)
	}

	if lo.IsEmpty(ipAddr) {
		return fmt.Errorf("SetPort: ip address not set for port %s", portName)
	}

	if lo.IsEmpty(subnetMask) {
		return fmt.Errorf("SetPort: subnet mask not set for port %s", portName)
	}

	// gateway
	if gateway, err = s.netService.SearchGateway(portName); err != nil {
		return fmt.Errorf("SetPort: %w", err)
	}

	if lo.IsEmpty(gateway) {
		return fmt.Errorf("SetPort: gateway not set for port %s", portName)
	}

	// dns
	dnsAddrs, err := s.netService.SearchDNS(portName)
	if err != nil {
		return fmt.Errorf("SetPort: %w", err)
	}

	if len(dnsAddrs) == 0 {
		return fmt.Errorf("SetPort: dns not set for port %s", portName)
	}

	portConfig := config.PortConfig{
		Name:     portName,
		Type:     constants.DevicePortTypeWAN,
		TableIDs: []int{100},
		Wan: &config.WanConfig{
			Mode:       constants.WanModeStatic,
			IPAddr:     ipAddr,
			SubnetMask: subnetMask,
			Gateway:    gateway,
			DNS:        dnsAddrs[0],
		},
	}

	if err = s.appStateService.Perform(
		entities.NewOnHubSetPort(portConfig),
	); err != nil {
		return fmt.Errorf("SetPort: %w", err)
	}

	return nil
}

// DeletePort deletes port configuration from hub.
func (s *Service) DeletePort(portName string) (err error) {
	if err = s.checkDeviceIsHub(); err != nil {
		return fmt.Errorf("DeletePort: %w", err)
	}

	if err = s.appStateService.Perform(
		entities.NewOnHubDeletePort(portName),
	); err != nil {
		return fmt.Errorf("DeletePort: %w", err)
	}

	return nil
}

// ListPorts queries hub ports with configuration.
func (s *Service) ListPorts() (ports entities.HubPorts, err error) {
	if err = s.checkDeviceIsHub(); err != nil {
		return ports, fmt.Errorf("ListPorts: %w", err)
	}

	cfg, err := s.configService.GetConfig()
	if err != nil {
		return ports, fmt.Errorf("ListPorts: %w", err)
	}

	linuxPorts, err := s.netService.GetAllPorts()
	if err != nil {
		return ports, fmt.Errorf("ListPorts: %w", err)
	}

	for _, p := range linuxPorts {
		if _, found := lo.Find(availablePortPrefixes, func(prefix string) bool {
			return strings.HasPrefix(p.IfName, prefix)
		}); !found {
			continue
		}

		ports = append(ports, entities.HubPort{
			Name:      p.IfName,
			OperState: p.OperState,
			MTU:       p.MTU,
			Addresses: lo.Map(p.AddrInfo, func(item net.AddrInfo, _ int) string {
				return fmt.Sprintf("%s/%d", item.Local, item.PrefixLen)
			}),
		})

		if cfg.Port == nil {
			continue
		}

		if portConfig, found := lo.Find(cfg.Port.PortConfigs, func(item config.PortConfig) bool {
			return item.Name == p.IfName
		}); found {
			// attach port configuration
			lastIndex := len(ports) - 1
			ports[lastIndex].Config = &portConfig
		}
	}

	return ports, nil
}

// Init connects hub to orchestrator (ZTP for hub).
func (s *Service) Init(serialNumber string, orchestratorAddrs []string) (err error) {
	if err = s.checkDeviceIsHub(); err != nil {
		return fmt.Errorf("Init: %w", err)
	}

	if err = s.appStateService.Perform(
		entities.NewOnFirstSetup(serialNumber, orchestratorAddrs),
	); err != nil {
		return fmt.Errorf("Init: %w", err)
	}

	return nil
}

func (s *Service) checkDeviceIsHub() (err error) {
	if s.deviceType != constants.DeviceTypeHub {
		return fmt.Errorf("checkDeviceIsHub: device is not hub (current: %s)", s.deviceType)
	}

	return nil
}
