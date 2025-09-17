package entities

import (
	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
)

type InitConfig struct {
	OFControllerAddr  string                  `json:"ofControllerAddr" validate:"hostname_port"`
	Wireguard         []config.WgConfig       `json:"wgConfigs" validate:"dive"`
	NetInit           NetInitConfig           `json:"netInit"`
	Pony              config.PonySection      `json:"pony"`
	Services          DeviceInitServiceConfig `json:"services"`
	AptSource         string                  `json:"aptSource"`
	OrchestratorAddrs []string                `json:"orchestratorAddrs"`
}

type DeviceInitServiceConfig struct {
	Trunk  config.TrunkSection         `json:"trunk"`
	L3     config.L3ServiceSection     `json:"l3"`
	ISB    config.ISBSection           `json:"isb"`
	Bridge config.BridgeServiceSection `json:"bridge"`
	P2P    config.P2PServiceSection    `json:"p2p"`
	FW     config.FWSection            `json:"fw"`
}

type NetInitConfig struct {
	LoopbackAddresses []string                `json:"loopbackAddresses" validate:"dive,cidr"`
	PortNames         []string                `json:"portNames" validate:"dive,required"`
	AllowedPorts      []config.NetAllowedPort `json:"allowedPorts" validate:"dive"`
	IPRules           []config.IPRule         `json:"ipRules" validate:"dive"`
	PortConfigs       []config.PortConfig     `json:"portConfigs" validate:"dive"`
	PortMTUs          []config.PortMTU        `json:"portMtus"`
	AdminStatePorts   []config.AdminStatePort `json:"adminStatePorts" validate:"dive"`
}

type FirstInitData struct {
	tx *activity.Transaction
}

func NewFirstInitData(tx *activity.Transaction) *FirstInitData {
	return &FirstInitData{
		tx: tx,
	}
}

func (d *FirstInitData) Tx() *activity.Transaction {
	return d.tx
}
