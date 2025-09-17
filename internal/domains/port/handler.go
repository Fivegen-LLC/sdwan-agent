package port

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/validator"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/rs/zerolog/log"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/objects/bo"
	"github.com/Fivegen-LLC/sdwan-agent/internal/objects/dto"
)

type (
	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
	}

	IService interface {
		GetPortRuntimeData() (ports bo.DevicePorts, err error)
		GetPortConfigData() (ports []config.PortConfig, err error)
		FlushPort(portName string) (err error)
		RenewDHCPLease(portName string) (err error)
	}
)

type Handler struct {
	service   IService
	publisher IMessagePublisher
}

func NewHandler(service IService, publisher IMessagePublisher) *Handler {
	return &Handler{
		service:   service,
		publisher: publisher,
	}
}

// FetchPorts handles fetch ports method, returns ports and it's types.
func (h *Handler) FetchPorts(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.publisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	log.Debug().Msg("FetchPorts: handling fetch ports")
	ports, err := h.service.GetPortRuntimeData()
	if err != nil {
		return fmt.Errorf("FetchPorts: %w", err)
	}

	if err = h.publisher.PublishResponse(request, ports.ToDto()); err != nil {
		return fmt.Errorf("FetchPorts: %w", err)
	}

	return nil
}

// FetchPortConfigs fetches device ports configurations.
func (h *Handler) FetchPortConfigs(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.publisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	log.Debug().Msg("FetchPortConfigs: handling fetch port configs")
	portConfigs, err := h.service.GetPortConfigData()
	if err != nil {
		return fmt.Errorf("FetchPortConfigs: %w", err)
	}

	if err = h.publisher.PublishResponse(request, h.convertPortsToDto(portConfigs)); err != nil {
		return fmt.Errorf("FetchPortConfigs: %w", err)
	}

	return nil
}

// FlushPort handles flush port config request.
func (h *Handler) FlushPort(request wschat.WebsocketMessage) (err error) { //nolint:dupl // skip dupl check
	defer func() {
		if err != nil {
			if sendErr := h.publisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	log.Debug().Msg("FlushPort: handling flush port")
	var requestBody struct {
		PortName string `json:"portName" validate:"min=3"`
	}
	if err = json.Unmarshal(request.Body, &requestBody); err != nil {
		return fmt.Errorf("FlushPort: %w", err)
	}

	if err = validator.Validator.Struct(requestBody); err != nil {
		return fmt.Errorf("FlushPort: %w", err)
	}

	if err = h.service.FlushPort(requestBody.PortName); err != nil {
		return fmt.Errorf("FlushPort: %w", err)
	}

	if err = h.publisher.PublishResponse(request, []byte{}); err != nil {
		return fmt.Errorf("FlushPort: %w", err)
	}

	return nil
}

// RenewDHCPLease resets dhcp leases for specified port.
func (h *Handler) RenewDHCPLease(request wschat.WebsocketMessage) (err error) { //nolint:dupl // skip dupl check
	defer func() {
		if err != nil {
			if sendErr := h.publisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	log.Debug().Msg("RenewDHCPLease: handling renew DHCP lease")
	var requestBody struct {
		PortName string `json:"portName" validate:"min=3"`
	}
	if err = json.Unmarshal(request.Body, &requestBody); err != nil {
		return fmt.Errorf("RenewDHCPLease: %w", err)
	}

	if err = validator.Validator.Struct(requestBody); err != nil {
		return fmt.Errorf("RenewDHCPLease: %w", err)
	}

	if err = h.service.RenewDHCPLease(requestBody.PortName); err != nil {
		return fmt.Errorf("RenewDHCPLease: %w", err)
	}

	if err = h.publisher.PublishResponse(request, []byte{}); err != nil {
		return fmt.Errorf("RenewDHCPLease: %w", err)
	}

	return nil
}

func (h *Handler) convertPortsToDto(portConfigs []config.PortConfig) (resultConfigs dto.DevicePortConfigs) {
	for _, p := range portConfigs {
		cfg := dto.DevicePortConfig{
			Name:   p.Name,
			Type:   p.Type,
			Tagged: p.IsTag,
		}

		if p.Wan != nil {
			wanConfig := p.Wan
			cfg.Wan = &dto.PortWANConfig{
				Mode: wanConfig.Mode,
			}

			if wanConfig.Options != nil {
				cfg.Wan.Options = &dto.WanPortOptions{
					StealthICMPMode: wanConfig.Options.StealthICMPMode,
				}
			}

			if wanConfig.Mode == constants.WanModeStatic {
				cfg.Wan.Static = &dto.PortStaticConfig{
					IPAddr:     wanConfig.IPAddr,
					SubnetMask: wanConfig.SubnetMask,
					Gateway:    wanConfig.Gateway,
					DNS:        wanConfig.DNS,
				}
			}

			if p.LTE != nil {
				cfg.LTE = &dto.PortLTEConfig{
					APNServer: p.LTE.APNServer,
				}
			}
		}

		resultConfigs = append(resultConfigs, cfg)
	}

	return resultConfigs
}
