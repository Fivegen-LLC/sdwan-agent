package l3

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell/commands"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"

	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/appstate/common"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

const (
	advASPathType  = 2
	advNexthopType = 3
)

type (
	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
	}

	ICmdService interface {
		ApplyCommandWithOutput(cmd string) (output []byte, err error)
	}

	IBGPService interface {
		ResetPeer(remoteIP, mode string) (err error)
	}

	IShellService interface {
		ExecOutput(command shell.ICommand) (output []byte, err error)
	}

	IAppStateService interface {
		Perform(transition common.IStateTransition) (err error)
	}

	Handler struct {
		messagePublisher IMessagePublisher
		cmdService       ICmdService
		bgpService       IBGPService
		shellService     IShellService
		appStateService  IAppStateService
		cliExecutable    string
		bgpExecutable    string

		validate *validator.Validate
	}
)

func NewHandler(messagePublisher IMessagePublisher, cmdService ICmdService, bgpService IBGPService,
	shellService IShellService, appStateService IAppStateService, cliExecutable, bgpExecutable string) *Handler {
	return &Handler{
		messagePublisher: messagePublisher,
		cmdService:       cmdService,
		bgpService:       bgpService,
		shellService:     shellService,
		appStateService:  appStateService,
		cliExecutable:    cliExecutable,
		bgpExecutable:    bgpExecutable,

		validate: validator.New(),
	}
}

// GetFlowRoutes fetches l3 flow routes for specified service.
func (h *Handler) GetFlowRoutes(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	var requestBody struct {
		ServiceID int `json:"serviceId"`
		Table     int `json:"table"`
	}
	if err = json.Unmarshal(request.Body, &requestBody); err != nil {
		return fmt.Errorf("GetFlowRoutes: %w", err)
	}

	if err = h.validate.Struct(requestBody); err != nil {
		return fmt.Errorf("GetFlowRoutes: %w", err)
	}

	cmd := buildListFlowRoutesCommand(h.cliExecutable, requestBody.ServiceID, requestBody.Table)
	output, err := h.cmdService.ApplyCommandWithOutput(cmd)
	if err != nil {
		return fmt.Errorf("GetFlowRoutes: %w", err)
	}

	responseBody := struct {
		Output []byte `json:"output"`
	}{
		Output: output,
	}
	if err = h.messagePublisher.PublishResponse(request, responseBody); err != nil {
		return fmt.Errorf("GetFlowRoutes: %w", err)
	}

	return nil
}

// UpdateConfig handles update L3 configuration for device.
func (h *Handler) UpdateConfig(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	var l3Config config.L3ServiceSection
	if err = json.Unmarshal(request.Body, &l3Config); err != nil {
		return fmt.Errorf("UpdateConfig: %w", err)
	}

	if err = h.validate.Struct(l3Config); err != nil {
		return fmt.Errorf("UpdateConfig: %w", err)
	}

	if err = h.appStateService.Perform(
		entities.NewOnUpdateConfig(
			config.Config{
				L3: &l3Config,
			},
		),
	); err != nil {
		return fmt.Errorf("UpdateConfig: %w", err)
	}

	if err = h.messagePublisher.PublishResponse(request, []byte{}); err != nil {
		return fmt.Errorf("UpdateConfig: %w", err)
	}

	return nil
}

// ResetBGPPeer resets BGP peer using specified mode.
func (h *Handler) ResetBGPPeer(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	var requestBody struct {
		RemoteIP string `json:"remoteIp" validate:"ipv4"`
		Mode     string `json:"mode" validate:"omitempty,oneof=soft softin softout"`
	}
	if err = json.Unmarshal(request.Body, &requestBody); err != nil {
		return fmt.Errorf("ResetBGPPeer: %w", err)
	}

	if err = h.validate.Struct(requestBody); err != nil {
		return fmt.Errorf("ResetBGPPeer: %w", err)
	}

	if err = h.bgpService.ResetPeer(requestBody.RemoteIP, requestBody.Mode); err != nil {
		return fmt.Errorf("ResetBGPPeer: %w", err)
	}

	if err = h.messagePublisher.PublishResponse(request, []byte{}); err != nil {
		return fmt.Errorf("ResetBGPPeer: %w", err)
	}

	return nil
}

// FetchBGPStats fetches BGP stats from bgpd.
func (h *Handler) FetchBGPStats(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	var requestBody struct {
		Neighbor string `json:"neighbor" validate:"ipv4"`
	}
	if err = json.Unmarshal(request.Body, &requestBody); err != nil {
		return fmt.Errorf("FetchBGPStats: %w", err)
	}
	if err = h.validate.Struct(requestBody); err != nil {
		return fmt.Errorf("FetchBGPStats: %w", err)
	}

	neighbor := requestBody.Neighbor
	lsBGPCmdJSON := commands.NewListBGPNeighborCmd(h.bgpExecutable, neighbor).
		WithJSON(true)
	output, err := h.shellService.ExecOutput(lsBGPCmdJSON)
	if err != nil {
		return fmt.Errorf("FetchBGPStats: %w", err)
	}

	var neighborOutput NeighborOutput
	if err = json.Unmarshal(output, &neighborOutput); err != nil {
		return fmt.Errorf("FetchBGPStats: %w", err)
	}

	var (
		timersState = neighborOutput.Timers.State
		timersConf  = neighborOutput.Timers.Config
		conf        = neighborOutput.Conf
		transport   = neighborOutput.Transport
	)
	result := NeighborInfo{
		LocalAddr:     transport.LocalAddress,
		LocalAS:       conf.LocalASN,
		PeerAddr:      conf.RemoteAddress,
		PeerAS:        conf.RemoteAS,
		State:         h.calculateSessionState(neighborOutput.State.SessionState),
		Holdtime:      timersConf.Holdtime,
		Keepalive:     timersConf.Keepalive,
		OperHoldtime:  timersState.Holdtime,
		OperKeepalive: timersState.Keepalive,
	}

	if timersState.Uptime != nil {
		result.Uptime = &timersState.Uptime.Seconds
	}

	if timersState.Downtime != nil {
		result.Downtime = &timersState.Downtime.Seconds
	}

	// adv-in
	lsAdvInCmd := commands.NewListBGPNeighborAdvInCmd(h.bgpExecutable, neighbor).
		WithJSON(true)
	if result.AdvIns, err = h.fetchAdvInfo(lsAdvInCmd, neighbor); err != nil {
		return fmt.Errorf("FetchBGPStats: %w", err)
	}

	// adv-out
	lsAdvOutCmd := commands.NewListBGPNeighborAdvOutCmd(h.bgpExecutable, neighbor).
		WithJSON(true)
	if result.AdvOuts, err = h.fetchAdvInfo(lsAdvOutCmd, neighbor); err != nil {
		return fmt.Errorf("FetchBGPStats: %w", err)
	}

	// general output
	lsBGPCmd := commands.NewListBGPNeighborCmd(h.bgpExecutable, requestBody.Neighbor)
	data, err := h.shellService.ExecOutput(lsBGPCmd)
	if err != nil {
		return fmt.Errorf("FetchBGPStats: %w", err)
	}
	result.NeighborOutput = base64.StdEncoding.EncodeToString(data)

	if err = h.messagePublisher.PublishResponse(request, result); err != nil {
		return fmt.Errorf("FetchBGPStats: %w", err)
	}

	return nil
}

// calculateSessionState returns string state representation based on int value
// UNKNOWN = 0; IDLE = 1; CONNECT = 2; ACTIVE = 3;
// OPENSENT = 4; OPENCONFIRM = 5; ESTABLISHED = 6.
func (h *Handler) calculateSessionState(state int) string {
	switch state {
	case 1:
		return "Idle"
	case 2:
		return "Connect"
	case 3:
		return "Active"
	case 4:
		return "OpenSent"
	case 5:
		return "OpenConfirm"
	case 6:
		return "Established"
	default:
		return "Unknown"
	}
}

func (h *Handler) fetchAdvInfo(lsAdvCmd shell.ICommand, neighbor string) (advs []NeighborAdvertise, err error) {
	advs = make([]NeighborAdvertise, 0, 10)
	output, err := h.shellService.ExecOutput(lsAdvCmd)
	if err != nil {
		// ok situation
		log.Warn().
			Str("neighbor", neighbor).
			Err(err).
			Msg("fetchAdvInfo: cannot fetch adv-in information for neighbor")
		return advs, nil
	}

	var advOutputs map[string][]NeighborAdvOutput
	if err = json.Unmarshal(output, &advOutputs); err != nil {
		return advs, fmt.Errorf("fetchAdvInfo: %w", err)
	}

	for _, advData := range advOutputs {
		for _, advOutput := range advData {
			result := NeighborAdvertise{
				Network: advOutput.NLRI.Prefix,
				Age:     convertAgeToSeconds(advOutput.Age),
			}

			for _, data := range advOutput.Attrs {
				switch data.Type {
				case advASPathType:
					result.ASPaths = make([]NeighborAdvertiseASPath, 0, len(data.ASPaths))
					for _, asPathData := range data.ASPaths {
						result.ASPaths = append(result.ASPaths, NeighborAdvertiseASPath{
							ASNs: asPathData.ASNs,
						})
					}

				case advNexthopType:
					result.Nexthop = data.Nexthop
				}
			}

			advs = append(advs, result)
		}
	}

	slices.SortFunc(advs, func(a, b NeighborAdvertise) int {
		return strings.Compare(a.Network, b.Network)
	})

	return advs, nil
}

func buildListFlowRoutesCommand(cliExecutable string, serviceID, table int) string {
	return fmt.Sprintf("%s l3 flow route ls -i %d -t %d", cliExecutable, serviceID, table)
}

func convertAgeToSeconds(age int64) int64 {
	if age == 0 {
		return age
	}

	res := time.Now().Unix() - age
	if res < 0 {
		res = -res
	}

	return res
}
