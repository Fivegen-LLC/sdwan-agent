package entities

import (
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
)

type AppState string

const (
	AppStateBoot         AppState = "boot"
	AppStateInit         AppState = "init" // ztp
	AppStateActive       AppState = "active"
	AppStateUpdateConfig AppState = "update_config"
	AppStateMaintenance  AppState = "maintenance" // update daemons
	AppStateZTPSetup     AppState = "ztp_setup"   // update config for init
	AppStateReset        AppState = "reset"
)

func (s AppState) String() string {
	return string(s)
}

// to any state after boot.

type OnAfterBoot struct {
	toState AppState
}

func NewOnAfterBoot(toState AppState) *OnAfterBoot {
	return &OnAfterBoot{
		toState: toState,
	}
}

func (e *OnAfterBoot) ToState() AppState {
	return e.toState
}

// to active state.

type OnFirstSetup struct {
	SerialNumber      string
	OrchestratorAddrs []string
}

func NewOnFirstSetup(serialNumber string, orchestratorAddrs []string) *OnFirstSetup {
	return &OnFirstSetup{
		SerialNumber:      serialNumber,
		OrchestratorAddrs: orchestratorAddrs,
	}
}

func (e *OnFirstSetup) ToState() AppState {
	return AppStateActive
}

type OnMigrateFromOldVersion struct {
	SerialNumber     string
	OrchestratorAddr string
}

func NewOnMigrateFromOldVersion(serialNumber, orchestratorAddr string) *OnMigrateFromOldVersion {
	return &OnMigrateFromOldVersion{
		SerialNumber:     serialNumber,
		OrchestratorAddr: orchestratorAddr,
	}
}

func (e *OnMigrateFromOldVersion) ToState() AppState {
	return AppStateActive
}

type OnFallback struct{}

func NewOnFallback() *OnFallback {
	return new(OnFallback)
}

func (e *OnFallback) ToState() AppState {
	return AppStateActive
}

type OnUpdateConfigFinished struct{}

func NewOnUpdateConfigFinished() *OnUpdateConfigFinished {
	return new(OnUpdateConfigFinished)
}

func (e *OnUpdateConfigFinished) ToState() AppState {
	return AppStateActive
}

type OnUpdateDeviceFinished struct {
	err error
}

func NewOnUpdateDeviceFinished(err error) *OnUpdateDeviceFinished {
	return &OnUpdateDeviceFinished{
		err: err,
	}
}

func (e *OnUpdateDeviceFinished) ToState() AppState {
	return AppStateActive
}

func (e *OnUpdateDeviceFinished) Err() error {
	return e.err
}

// to update config state.

type OnUpdateConfig struct {
	Config config.Config
}

func NewOnUpdateConfig(cfg config.Config) *OnUpdateConfig {
	return &OnUpdateConfig{
		Config: cfg,
	}
}

func (e *OnUpdateConfig) ToState() AppState {
	return AppStateUpdateConfig
}

type OnRebuildServices struct{}

func NewOnRebuildServices() *OnRebuildServices {
	return new(OnRebuildServices)
}

func (e *OnRebuildServices) ToState() AppState {
	return AppStateUpdateConfig
}

// to maintenance state.

type OnUpdateDevice struct {
	InstallPackages InstallPackageRequest
}

func NewOnUpdateDevice(installPackages InstallPackageRequest) *OnUpdateDevice {
	return &OnUpdateDevice{
		InstallPackages: installPackages,
	}
}

func (e *OnUpdateDevice) ToState() AppState {
	return AppStateMaintenance
}

// to reset state.

type OnReset struct{}

func NewOnReset() *OnReset {
	return new(OnReset)
}

func (e *OnReset) ToState() AppState {
	return AppStateReset
}

// to init state.

type OnZTPSetupFinished struct{}

func NewOnZTPSetupFinished() *OnZTPSetupFinished {
	return new(OnZTPSetupFinished)
}

func (e *OnZTPSetupFinished) ToState() AppState {
	return AppStateInit
}

type OnZTPSetupInterrupted struct{}

func NewOnZTPSetupInterrupted() *OnZTPSetupInterrupted {
	return new(OnZTPSetupInterrupted)
}

func (e *OnZTPSetupInterrupted) ToState() AppState {
	return AppStateInit
}

type OnInitFallback struct{}

func NewOnInitFallback() *OnInitFallback {
	return new(OnInitFallback)
}

func (e *OnInitFallback) ToState() AppState {
	return AppStateInit
}

type OnHubResetFinished struct{}

func NewOnHubResetFinished() *OnHubResetFinished {
	return new(OnHubResetFinished)
}

func (e *OnHubResetFinished) ToState() AppState {
	return AppStateInit
}

// to ztp setup state.

type OnZTPSetupConfig struct {
	Config config.Config
}

func NewOnZTPSetupConfig(cfg config.Config) *OnZTPSetupConfig {
	return &OnZTPSetupConfig{
		Config: cfg,
	}
}

func (e *OnZTPSetupConfig) ToState() AppState {
	return AppStateZTPSetup
}

type OnHubSetPort struct {
	PortConfig config.PortConfig
}

func NewOnHubSetPort(portConfig config.PortConfig) *OnHubSetPort {
	return &OnHubSetPort{
		PortConfig: portConfig,
	}
}

func (e *OnHubSetPort) ToState() AppState {
	return AppStateZTPSetup
}

type OnHubDeletePort struct {
	PortName string
}

func NewOnHubDeletePort(portName string) *OnHubDeletePort {
	return &OnHubDeletePort{
		PortName: portName,
	}
}

func (e *OnHubDeletePort) ToState() AppState {
	return AppStateZTPSetup
}
