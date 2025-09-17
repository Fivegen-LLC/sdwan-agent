package infrastructure

import (
	"fmt"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/badgerutils"
	"github.com/dgraph-io/badger/v4"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/cmd"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/config"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/debug"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/deviceaction"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/deviceinit"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/dhcp"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/fw"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/hub"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/isb"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/l3"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/lte"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/ovs"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/pony"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/port"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/service"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/trunk"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/updatemanager"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/ztp"

	"github.com/Fivegen-LLC/sdwan-agent/internal/environment"
)

type IInjector interface {
	InjectCmdHandler() *cmd.Handler
	InjectISBHandler() *isb.Handler
	InjectPonyHandler() *pony.Handler
	InjectPortHandler() *port.Handler
	InjectConfigHandler() *config.Handler
	InjectDeviceInitHandler() *deviceinit.Handler
	InjectL3Handler() *l3.Handler
	InjectTrunkHandler() *trunk.Handler
	InjectServiceHandler() *service.Handler
	InjectDeviceActionHandler() *deviceaction.Handler
	InjectDHCPHandler() *dhcp.Handler
	InjectOVSHandler() *ovs.Handler
	InjectFWHandler() *fw.Handler
	InjectAppStateWSHandler() *appstate.WSHandler
	InjectUpdateManagerHandler() *updatemanager.Handler
	InjectLTEHandler() *lte.Handler

	// MQ handlers.

	InjectZTPMQHandler() *ztp.MQHandler
	InjectConfigMQHandler() *config.MQHandler
	InjectDeviceActionMQHandler() *deviceaction.MQHandler
	InjectHubMQHandler() *hub.MQHandler
	InjectDebugMQHandler() *debug.MQHandler
}

type Kernel struct {
	env environment.Environment

	DB *badger.DB
}

func Inject(env environment.Environment) (k *Kernel, err error) {
	k = &Kernel{
		env: env,
	}

	options := badger.DefaultOptions(constants.AgentConfigPath).
		WithLogger(badgerutils.NewLogger()).
		WithMemTableSize(64 << 17) // ~8MB

	if k.DB, err = badger.Open(options); err != nil {
		return k, fmt.Errorf("Inject: %w", err)
	}

	return k, nil
}

func (k *Kernel) InjectCmdHandler() *cmd.Handler {
	return cmd.NewHandler(
		k.InjectMessagePublisher(),
	)
}

func (k *Kernel) InjectISBHandler() *isb.Handler {
	return isb.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectConfigService(),
	)
}

func (k *Kernel) InjectPonyHandler() *pony.Handler {
	return pony.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectConfigService(),
	)
}

func (k *Kernel) InjectPortHandler() *port.Handler {
	return port.NewHandler(
		k.InjectPortService(),
		k.InjectMessagePublisher(),
	)
}

func (k *Kernel) InjectConfigHandler() *config.Handler {
	return config.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectAppStateService(),
		k.InjectConfigService(),
	)
}

func (k *Kernel) InjectDeviceInitHandler() *deviceinit.Handler {
	return deviceinit.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectDeviceInitService(),
		k.InjectAppStateService(),
	)
}

func (k *Kernel) InjectL3Handler() *l3.Handler {
	return l3.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectCmdService(),
		k.InjectBGPService(),
		k.InjectShellService(),
		k.InjectAppStateService(),
		constants.CLIExtExecutable,
		constants.BGPExecutable,
	)
}

func (k *Kernel) InjectTrunkHandler() *trunk.Handler {
	return trunk.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectConfigService(),
	)
}

func (k *Kernel) InjectServiceHandler() *service.Handler {
	return service.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectAppStateService(),
	)
}

func (k *Kernel) InjectDeviceActionHandler() *deviceaction.Handler {
	return deviceaction.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectCmdService(),
		k.InjectAppStateService(),
		constants.CLIExtExecutable,
	)
}

func (k *Kernel) InjectDHCPHandler() *dhcp.Handler {
	return dhcp.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectShellService(),
	)
}

func (k *Kernel) InjectOVSHandler() *ovs.Handler {
	return ovs.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectOVSService(),
	)
}

func (k *Kernel) InjectFWHandler() *fw.Handler {
	return fw.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectCmdService(),
		constants.CLIExtExecutable,
	)
}

func (k *Kernel) InjectAppStateWSHandler() *appstate.WSHandler {
	return appstate.NewWSHandler(
		k.InjectMessagePublisher(),
		k.InjectAppStateService(),
	)
}

// MQ handlers.

func (k *Kernel) InjectZTPMQHandler() *ztp.MQHandler {
	return ztp.NewMQHandler(
		k.InjectAppStateService(),
	)
}

func (k *Kernel) InjectConfigMQHandler() *config.MQHandler {
	return config.NewMQHandler(
		k.InjectConfigService(),
		k.InjectAppStateService(),
	)
}

func (k *Kernel) InjectDeviceActionMQHandler() *deviceaction.MQHandler {
	return deviceaction.NewMQHandler(
		k.InjectAppStateService(),
	)
}

func (k *Kernel) InjectUpdateManagerHandler() *updatemanager.Handler {
	return updatemanager.NewHandler(
		k.InjectUpdateManagerService(),
		k.InjectMessagePublisher(),
	)
}

func (k *Kernel) InjectHubMQHandler() *hub.MQHandler {
	return hub.NewMQHandler(
		k.InjectHubService(),
	)
}

func (k *Kernel) InjectLTEHandler() *lte.Handler {
	return lte.NewHandler(
		k.InjectMessagePublisher(),
		k.InjectLTEService(),
	)
}

func (k *Kernel) InjectDebugMQHandler() *debug.MQHandler {
	return debug.NewMQHandler()
}
