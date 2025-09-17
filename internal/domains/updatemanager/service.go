package updatemanager

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/mq"
	"github.com/nats-io/nats.go"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

const (
	downloadDevicePackagesTimeout = time.Second * 30
	getDevicePackagesTimeout      = time.Minute
	getDevicePackagesRetryAmount  = 5
)

type (
	IAppStateService interface {
		Perform(transition common.IStateTransition) (err error)
	}

	IMQService interface {
		Request(subject string, message any, timeout time.Duration, optionFuncs ...mq.RequestOption) (response *nats.Msg, err error)
	}

	Service struct {
		appStateService  IAppStateService
		mqService        IMQService
		cliExtExecutable string
	}
)

func NewService(appStateService IAppStateService, mqService IMQService, cliExtExecutable string) *Service {
	return &Service{
		appStateService:  appStateService,
		mqService:        mqService,
		cliExtExecutable: cliExtExecutable,
	}
}

// SetAptSource sets apt source.
func (s *Service) SetAptSource(aptSource string) (err error) {
	if err = exec.Command(s.cliExtExecutable, "update-manager", "set-apt-source", "-s", aptSource).Run(); err != nil { //nolint:gosec // skip check
		return fmt.Errorf("SetAptSource: %w", err)
	}

	return nil
}

// Download makes download request to update manager.
func (s *Service) Download(request entities.DownloadPackageRequest) (err error) {
	resp, err := s.mqService.Request(constants.MQUpdateManagerDownload, request, downloadDevicePackagesTimeout)
	if err != nil {
		return fmt.Errorf("Download: %w", err)
	}

	var mqResp mq.Response
	if err = json.Unmarshal(resp.Data, &mqResp); err != nil {
		return fmt.Errorf("Download: %w", err)
	}

	if mqResp.IsError() {
		return fmt.Errorf("Download: %w", mqResp.Error())
	}

	return nil
}

// Install changes state to maintenance and installs new packages.
func (s *Service) Install(request entities.InstallPackageRequest) (err error) {
	if err = s.appStateService.Perform(entities.NewOnUpdateDevice(request)); err != nil {
		return fmt.Errorf("Install: %w", err)
	}

	return nil
}

// GetVersions makes request to update manager for get actual versions.
func (s *Service) GetVersions() (versions entities.ActualPackageVersions, err error) {
	resp, err := s.mqService.Request(constants.MQUpdateManagerGetVersions, nil, getDevicePackagesTimeout,
		mq.NewRetryAmountOption(getDevicePackagesRetryAmount))
	if err != nil {
		return versions, fmt.Errorf("GetVersions: %w", err)
	}

	var mqResp struct {
		mq.Response
		Versions entities.ActualPackageVersions `json:"packages"`
	}
	if err = json.Unmarshal(resp.Data, &mqResp); err != nil {
		return versions, fmt.Errorf("GetVersions: %w", err)
	}

	if mqResp.IsError() {
		return versions, fmt.Errorf("GetVersions: %w", mqResp.Error())
	}

	return mqResp.Versions, nil
}
