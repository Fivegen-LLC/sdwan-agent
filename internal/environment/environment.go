package environment

import (
	"fmt"

	"github.com/samber/lo"
	"github.com/spf13/viper"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
)

type Environment struct {
	Agent
}

type Agent struct {
	EndPoint     string
	DeviceID     string
	DeviceType   string
	WgConfigRoot string
	LogfilePath  string
	LogLevel     string
}

func New() (e Environment, err error) {
	v := viper.New()
	v.AutomaticEnv()

	// linux env
	e.Agent.DeviceType = v.GetString("SDWAN_DEVICE")
	if lo.IsEmpty(e.Agent.DeviceType) {
		return e, fmt.Errorf("New: device type env is empty")
	}

	// agent settings
	v.SetEnvPrefix("AGENT")
	e.Agent.EndPoint = v.GetString("ENDPOINT")
	e.Agent.DeviceID = v.GetString("ID")
	e.Agent.WgConfigRoot = v.GetString("CFG_ROOT")
	e.Agent.LogfilePath = v.GetString("LOG_FILE")
	if lo.IsEmpty(e.Agent.LogfilePath) {
		e.Agent.LogfilePath = constants.DefaultLogfilePath
	}
	e.Agent.LogLevel = v.GetString("LOG_LEVEL")
	if lo.IsEmpty(e.Agent.LogLevel) {
		e.Agent.LogLevel = "info"
	}

	return e, nil
}

func (e Agent) IsDebug() bool {
	return e.LogLevel == "debug"
}
