package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/mq"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

const (
	installDevicePackagesTimeout = 30 * time.Second
	installRequestTimeout        = 2 * time.Second
)

type MaintenanceStateHandler struct {
	mqService        IMQService
	configService    IConfigService
	messagePublisher IMessagePublisher
	websocketService IWebsocketService
	activityService  IActivityService
}

func NewMaintenanceStateHandler(mqService IMQService, configService IConfigService, messagePublisher IMessagePublisher,
	websocketService IWebsocketService, activityService IActivityService) *MaintenanceStateHandler {
	return &MaintenanceStateHandler{
		mqService:        mqService,
		configService:    configService,
		messagePublisher: messagePublisher,
		websocketService: websocketService,
		activityService:  activityService,
	}
}

func (h *MaintenanceStateHandler) StateID() entities.AppState {
	return entities.AppStateMaintenance
}

func (h *MaintenanceStateHandler) ValidateTransition(fromState entities.AppState) (err error) {
	if fromState != entities.AppStateActive && fromState != entities.AppStateBoot {
		return fmt.Errorf("ValidateTransition: %w", errs.ErrTransitionNotSupported)
	}

	return nil
}

func (h *MaintenanceStateHandler) Handle(ctx context.Context, tx *activity.Transaction, transition common.IStateTransition) (result common.StateHandleResult, err error) {
	if _, ok := transition.(*entities.OnAfterBoot); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: after boot transition")

		updErr := h.waitInstallFinishedAfterBoot()
		if sErr := h.sendInstallFinished(updErr); sErr != nil {
			log.Error().
				Err(sErr).
				Msg("Handle: send update error")
			updErr = errors.Join(updErr, sErr)
		}

		result.Transition = entities.NewOnUpdateDeviceFinished(updErr)
		return result, nil
	}

	if data, ok := transition.(*entities.OnUpdateDevice); ok {
		log.Info().
			Any("target state", h.StateID()).
			Msg("Handle: update device transition")

		updErr := h.updateDevice(ctx, tx, data.InstallPackages)
		if sErr := h.sendInstallFinished(updErr); sErr != nil {
			log.Error().
				Err(sErr).
				Msg("Handle: send update error")
			updErr = errors.Join(updErr, sErr)
		}

		result.Transition = entities.NewOnUpdateDeviceFinished(updErr)
		return result, nil
	}

	return result, fmt.Errorf("Handle: %w", errs.ErrInvalidTransitionType)
}

func (h *MaintenanceStateHandler) OnExit(_ context.Context, _ *activity.Transaction, _ common.IStateTransition) (err error) {
	return nil
}

func (h *MaintenanceStateHandler) installUpdateManager(request entities.InstallPackageRequest) (result entities.InstallPackageRequest, err error) {
	if pkg, ok := lo.Find(request.PackagesToInstall, func(item entities.PackageItem) (ok bool) {
		return item.Name == constants.SdwanUpdateManagerPackageName
	}); ok {
		newVersion, currentVersion := pkg.Version, pkg.PreviousVersion
		output, err := exec.Command("bash", "-c", entities.InstallUpdateManagerScript, "-", newVersion, currentVersion).CombinedOutput() //nolint:gosec // all arguments are valid
		if err != nil {
			return result, fmt.Errorf("installUpdateManager: %w: %s", err, output)
		}

		request.PackagesToInstall = slices.DeleteFunc(request.PackagesToInstall, func(item entities.PackageItem) bool {
			return item.Name == constants.SdwanUpdateManagerPackageName
		})
	}

	return request, nil
}

func (h *MaintenanceStateHandler) updateDevice(ctx context.Context, tx *activity.Transaction, request entities.InstallPackageRequest) (err error) {
	request, err = h.installUpdateManager(request)
	if err != nil {
		return fmt.Errorf("updateDevice: %w", err)
	}

	if lo.ContainsBy(request.PackagesToInstall, func(item entities.PackageItem) bool {
		return item.Name == constants.SdwanAgentPackageName ||
			item.Name == constants.SdwanBGPAdapterPackageName ||
			item.Name == constants.SdwanBGPDPackageName
	}) {
		if err = h.deleteServices(ctx, tx); err != nil {
			return fmt.Errorf("updateDevice: %w", err)
		}
	}

	if lo.ContainsBy(request.PackagesToInstall, func(item entities.PackageItem) bool {
		return item.Name == constants.SdwanAgentPackageName
	}) {
		// commit changes and wait for restart agent service
		var checkpointID string
		if checkpointID, err = h.activityService.AddCheckPoint(ctx, tx); err != nil {
			return fmt.Errorf("updateDevice: %w", err)
		}

		defer func() {
			if err == nil {
				return
			}

			if delErr := h.activityService.DeleteCheckPoint(ctx, tx, checkpointID); delErr != nil {
				log.Error().
					Err(delErr).
					Msg("updateDevice: cleanup checkpoint error")
			}
		}()
	}

	resp, err := h.mqService.Request(constants.MQUpdateManagerInstall, request, installRequestTimeout)
	if err != nil {
		return fmt.Errorf("updateDevice: %w", err)
	}

	var data mq.Response
	if err = json.Unmarshal(resp.Data, &data); err != nil {
		return fmt.Errorf("updateDevice: %w", err)
	}

	if data.IsError() {
		return fmt.Errorf("updateDevice: %w", data.Error())
	}

	if err = h.waitInstallFinished(); err != nil {
		return fmt.Errorf("updateDevice: %w", err)
	}

	return nil
}

func (h *MaintenanceStateHandler) deleteServices(ctx context.Context, tx *activity.Transaction) (err error) {
	if err = h.configService.UpdateConfigWithTx(
		ctx, tx,
		config.Config{
			Trunk:  &config.TrunkSection{},
			P2P:    &config.P2PServiceSection{},
			Bridge: &config.BridgeServiceSection{},
			ISB:    &config.ISBSection{},
			L3:     &config.L3ServiceSection{},
			FW:     &config.FWSection{},
		},
	); err != nil {
		return fmt.Errorf("deleteServices: %w", err)
	}

	return nil
}

func (h *MaintenanceStateHandler) waitInstallFinished() (err error) {
	installFinishedChan := make(chan *nats.Msg)
	defer close(installFinishedChan)

	sub, err := h.mqService.ChanSubscribe(constants.MQAgentInstallFinished, installFinishedChan)
	if err != nil {
		return fmt.Errorf("waitInstallFinished: %w", err)
	}
	defer func() {
		if uErr := sub.Unsubscribe(); uErr != nil {
			log.Error().
				Err(uErr).
				Msg("waitInstallFinished")
		}
	}()

	select {
	case msg := <-installFinishedChan:
		if err = msg.Respond(nil); err != nil {
			return fmt.Errorf("waitInstallFinished: %w", err)
		}

		var resp struct {
			ErrorMessage string `json:"errorMessage"`
		}
		if err = json.Unmarshal(msg.Data, &resp); err != nil {
			return fmt.Errorf("waitInstallFinished: %w", err)
		}

		if !lo.IsEmpty(resp.ErrorMessage) {
			return fmt.Errorf("waitInstallFinished: %s", resp.ErrorMessage)
		}

		return nil

	case <-time.After(installDevicePackagesTimeout):
		return errors.New("waitInstallFinished: install timeout")
	}
}

func (h *MaintenanceStateHandler) waitInstallFinishedAfterBoot() (err error) {
	// start publisher
	if err = h.websocketService.Start(); err != nil {
		return fmt.Errorf("waitInstallFinishedAfterBoot: %w", err)
	}

	// wait install finished
	installFinishedChan := make(chan *nats.Msg)
	defer close(installFinishedChan)

	sub, err := h.mqService.ChanSubscribe(constants.MQAgentInstallFinished, installFinishedChan)
	if err != nil {
		return fmt.Errorf("waitInstallFinishedAfterBoot: %w", err)
	}
	defer func() {
		if uErr := sub.Unsubscribe(); uErr != nil {
			log.Error().
				Err(uErr).
				Msg("waitInstallFinishedAfterBoot")
		}
	}()

	select {
	case msg := <-installFinishedChan:
		if err = msg.Respond(nil); err != nil {
			return fmt.Errorf("waitInstallFinishedAfterBoot: %w", err)
		}

		var resp struct {
			ErrorMessage string `json:"errorMessage"`
		}
		if err = json.Unmarshal(msg.Data, &resp); err != nil {
			return fmt.Errorf("waitInstallFinishedAfterBoot: %w", err)
		}

		if !lo.IsEmpty(resp.ErrorMessage) {
			return fmt.Errorf("waitInstallFinishedAfterBoot: %s", resp.ErrorMessage)
		}

		return nil

	case <-time.After(installDevicePackagesTimeout):
		log.Error().
			Msg("waitInstallFinishedAfterBoot: install device packages timeout")

		return errors.New("waitInstallFinishedAfterBoot: install device packages timeout")
	}
}

func (h *MaintenanceStateHandler) sendInstallFinished(updErr error) (err error) {
	body := struct {
		ErrorMessage string `json:"errorMessage"`
	}{}
	if updErr != nil {
		body.ErrorMessage = updErr.Error()
	}

	defer func() {
		h.messagePublisher.Reconnect()
	}()

	resp, err := h.messagePublisher.PublishRequest(constants.MethodInstallDevicePackagesFinished, constants.OrchestratorWSID, body,
		wschat.RequestOptions{
			Timeout: lo.ToPtr(5 * time.Second),
		},
	)
	if err != nil {
		return fmt.Errorf("sendInstallFinished: %w", err)
	}

	if resp.IsErrorResponse() {
		return fmt.Errorf("sendInstallFinished: %w", resp.Error())
	}

	return nil
}
