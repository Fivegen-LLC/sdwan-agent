package dto

type DevicePortConfig struct {
	Name   string         `json:"name"`
	Type   string         `json:"type"`
	Tagged bool           `json:"tagged"`
	Wan    *PortWANConfig `json:"wan"`
	LTE    *PortLTEConfig `json:"lte,omitempty"`
}

type PortWANConfig struct {
	Mode    string            `json:"mode"`
	Static  *PortStaticConfig `json:"static,omitempty"`
	Options *WanPortOptions   `json:"options,omitempty"`
}

type PortStaticConfig struct {
	IPAddr     string `json:"ip"`
	SubnetMask string `json:"subnetMask"`
	Gateway    string `json:"gateway"`
	DNS        string `json:"dns,omitempty"`
}

type PortLTEConfig struct {
	APNServer string `json:"apnServer"`
}

type WanPortOptions struct {
	StealthICMPMode bool `json:"stealthICMP"` //nolint:tagliatelle // backend API
}

type DevicePortConfigs []DevicePortConfig

type DevicePort struct {
	Name       string   `json:"name"`
	State      string   `json:"state"`
	Mac        string   `json:"mac"`
	MTU        int      `json:"mtu"`
	IPs        []string `json:"ips"`
	SubnetMask string   `json:"subnetMask"`
	Gateway    string   `json:"gateway"`
	DNS        []string `json:"dns"`
}

type DevicePorts []DevicePort

type LinuxInterface struct {
	Name     string               `json:"ifname"`
	State    string               `json:"operstate"`
	Mac      string               `json:"address"`
	MTU      int                  `json:"mtu"`
	AddrInfo []LinuxInterfaceAddr `json:"addr_info"` //nolint:tagliatelle // linux api
}

type LinuxInterfaceAddr struct {
	Addr   string `json:"local"`
	Prefix int    `json:"prefixlen"`
}

type LinuxInterfaces []LinuxInterface
