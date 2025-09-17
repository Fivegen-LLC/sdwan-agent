package entities

import (
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
)

type HubPort struct {
	Name      string             `json:"name"`
	OperState string             `json:"operState"`
	MTU       int                `json:"mtu"`
	Addresses []string           `json:"addresses"`
	Config    *config.PortConfig `json:"config,omitempty"`
}

type HubPorts []HubPort
