package bo

import (
	"fmt"
	"net"
	"slices"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/netutils"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/objects/dto"
)

type DevicePortConfig struct {
	Name  string     `json:"name"`
	Type  string     `json:"type"`
	IsTag bool       `json:"isTag"`
	Tag   *TagConfig `json:"tag,omitempty"`
	Wan   *WanConfig `json:"wan,omitempty"`
}

type TagConfig struct {
	ID         int    `json:"id"`
	ParentPort string `json:"parentPort"`
}

type WanConfig struct {
	Mode       string `json:"mode"`
	IPAddr     string `json:"ipAddr"`
	SubnetMask string `json:"subnetMask"`
	Gateway    string `json:"gateway"`
	DNS        string `json:"dns,omitempty"`
}

type DevicePortConfigs []DevicePortConfig

func (p DevicePortConfig) ToDto() dto.DevicePortConfig {
	config := dto.DevicePortConfig{
		Name:   p.Name,
		Type:   p.Type,
		Tagged: p.IsTag,
	}

	if p.Wan != nil {
		wanConfig := p.Wan
		config.Wan = &dto.PortWANConfig{
			Mode: wanConfig.Mode,
		}

		if wanConfig.Mode == constants.WanModeStatic {
			config.Wan.Static = &dto.PortStaticConfig{
				IPAddr:     wanConfig.IPAddr,
				SubnetMask: wanConfig.SubnetMask,
				Gateway:    wanConfig.Gateway,
				DNS:        wanConfig.DNS,
			}
		}
	}

	return config
}

func (p DevicePortConfigs) ToDto() dto.DevicePortConfigs {
	ports := make(dto.DevicePortConfigs, 0, len(p))
	for _, port := range p {
		ports = append(ports, port.ToDto())
	}

	return ports
}

type DevicePort struct {
	Name       string
	State      string
	Mac        string
	MTU        int
	IPs        []string
	SubnetMask string
	Gateway    string
	DNS        []string
}

type DevicePorts []DevicePort

func (p DevicePort) ToDto() dto.DevicePort {
	ips := make([]string, 0, len(p.IPs))
	ips = append(ips, p.IPs...)

	return dto.DevicePort{
		Name:       p.Name,
		State:      p.State,
		Mac:        p.Mac,
		MTU:        p.MTU,
		IPs:        ips,
		SubnetMask: p.SubnetMask,
		Gateway:    p.Gateway,
		DNS:        slices.Clone(p.DNS),
	}
}

func (p DevicePorts) ToDto() dto.DevicePorts {
	ports := make(dto.DevicePorts, 0, len(p))
	for _, port := range p {
		ports = append(ports, port.ToDto())
	}

	return ports
}

func NewDevicePortFromLinuxInterface(linuxInterface dto.LinuxInterface) DevicePort {
	var (
		subnetMask string
		ips        = make([]string, 0, len(linuxInterface.AddrInfo))
	)
	for _, addr := range linuxInterface.AddrInfo {
		ips = append(ips, fmt.Sprintf("%s/%d", addr.Addr, addr.Prefix))

		if net.ParseIP(addr.Addr).To4() != nil {
			subnetMask = netutils.NetmaskToString(addr.Prefix)
		}
	}

	return DevicePort{
		Name:       linuxInterface.Name,
		State:      linuxInterface.State,
		Mac:        linuxInterface.Mac,
		MTU:        linuxInterface.MTU,
		IPs:        ips,
		SubnetMask: subnetMask,
	}
}

func NewDevicePortsFromLinuxInterfaces(linuxInterfaces dto.LinuxInterfaces) DevicePorts {
	ports := make(DevicePorts, 0, len(linuxInterfaces))
	for _, linuxInterface := range linuxInterfaces {
		ports = append(ports, NewDevicePortFromLinuxInterface(linuxInterface))
	}

	return ports
}
