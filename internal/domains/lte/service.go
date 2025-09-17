package lte

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell/commands"
	"github.com/samber/lo"

	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

const (
	idVendorFile  = "idVendor"
	idProductFile = "idProduct"
)

type (
	IShellService interface {
		Exec(command shell.ICommand) (err error)
		ExecOutput(command shell.ICommand) (output []byte, err error)
	}

	Service struct {
		shellService IShellService
	}
)

func NewService(shellService IShellService) *Service {
	return &Service{
		shellService: shellService,
	}
}

func (s *Service) CollectStats() (stats entities.LTEStats, err error) {
	// get list of modems
	modemListCmd := commands.NewCustomCmd("mmcli -L --output-json")
	output, err := s.shellService.ExecOutput(modemListCmd)
	if err != nil {
		return stats, fmt.Errorf("CollectStats: %w", err)
	}

	var modemList struct {
		ModemList []string `json:"modem-list"` //nolint:tagliatelle // network manager API
	}
	if err = json.Unmarshal(output, &modemList); err != nil {
		return stats, fmt.Errorf("CollectStats: %w", err)
	}

	modemIDs := make([]string, 0, len(modemList.ModemList))
	for _, modem := range modemList.ModemList {
		parts := strings.Split(modem, "/")
		if len(parts) > 0 {
			modemIDs = append(modemIDs, parts[len(parts)-1])
		}
	}

	var modemInfo struct {
		Modem struct {
			Field3GPP struct {
				IMEI         string `json:"imei"`
				OperatorCode string `json:"operator-code"` //nolint:tagliatelle // network manager API
				OperatorName string `json:"operator-name"` //nolint:tagliatelle // network manager API
			} `json:"3gpp"`
			Generic struct {
				Model         string   `json:"model"`
				PowerState    string   `json:"power-state"` //nolint:tagliatelle // network manager API
				State         string   `json:"state"`
				Device        string   `json:"device"`
				Ports         []string `json:"ports"`
				SignalQuality struct {
					Recent string `json:"recent"`
					Value  string `json:"value"`
				} `json:"signal-quality"` //nolint:tagliatelle // network manager API
				PrimaryPort string `json:"primary-port"` //nolint:tagliatelle // network manager API
			} `json:"generic"`
		} `json:"modem"`
	}
	for _, modemID := range modemIDs {
		// get modem info
		modemInfoCmd := commands.NewCustomCmd(fmt.Sprintf("mmcli --modem=%s --output-json", modemID))
		if output, err = s.shellService.ExecOutput(modemInfoCmd); err != nil {
			return stats, fmt.Errorf("CollectStats: %w", err)
		}

		if err = json.Unmarshal(output, &modemInfo); err != nil {
			return stats, fmt.Errorf("CollectStats: %w", err)
		}

		// search for port
		var port string
		for _, p := range modemInfo.Modem.Generic.Ports {
			if strings.HasPrefix(p, "wwan") {
				port, _, _ = strings.Cut(p, " ")
				port = strings.TrimSpace(port)
				break
			}
		}

		// save modem info
		stat := entities.LTEStat{
			IMEI:         modemInfo.Modem.Field3GPP.IMEI,
			OperatorCode: modemInfo.Modem.Field3GPP.OperatorCode,
			OperatorName: modemInfo.Modem.Field3GPP.OperatorName,
			Model:        modemInfo.Modem.Generic.Model,
			PowerState:   modemInfo.Modem.Generic.PowerState,
			State:        modemInfo.Modem.Generic.State,
			DevicePath:   modemInfo.Modem.Generic.Device,
			Port:         port,
			SignalQuality: entities.SignalQualityStat{
				Recent: modemInfo.Modem.Generic.SignalQuality.Recent,
				Value:  modemInfo.Modem.Generic.SignalQuality.Value,
			},
		}

		// get interface info
		primaryPort := modemInfo.Modem.Generic.PrimaryPort
		if !lo.IsEmpty(primaryPort) {
			interfaceInfoCmd := commands.NewCustomCmd(fmt.Sprintf("nmcli --terse device show %s", primaryPort))
			if output, err = s.shellService.ExecOutput(interfaceInfoCmd); err != nil {
				return stats, fmt.Errorf("CollectStats: %w", err)
			}

			if err = s.parseInterfaceInfo(output, &stat); err != nil {
				return stats, fmt.Errorf("CollectStats: %w", err)
			}
		}

		stats = append(stats, stat)
	}

	return stats, nil
}

// ResetModem resets specified modem.
func (s *Service) ResetModem(modemSysPath string) (err error) {
	var (
		vendorPath  = filepath.Join(modemSysPath, idVendorFile)
		productPath = filepath.Join(modemSysPath, idProductFile)
	)
	vendorBytes, err := os.ReadFile(vendorPath)
	if err != nil {
		return fmt.Errorf("ResetModem: %w", err)
	}

	productBytes, err := os.ReadFile(productPath)
	if err != nil {
		return fmt.Errorf("ResetModem: %w", err)
	}

	var (
		vendor  = string(bytes.TrimSpace(vendorBytes))
		product = string(bytes.TrimSpace(productBytes))
		cmd     = fmt.Sprintf("usbreset %s:%s", vendor, product)
	)
	if err = s.shellService.Exec(commands.NewCustomCmd(cmd)); err != nil {
		return fmt.Errorf("ResetModem: %w", err)
	}

	return nil
}

func (s *Service) parseInterfaceInfo(cmdOutput []byte, stat *entities.LTEStat) (err error) {
	scanner := bufio.NewScanner(bytes.NewReader(cmdOutput))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if lo.IsEmpty(line) {
			continue
		}

		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "GENERAL.TYPE":
			stat.Type = value
		case "GENERAL.HWADDR":
			stat.HWAddr = value
		case "GENERAL.MTU":
			if stat.MTU, err = strconv.Atoi(value); err != nil {
				return fmt.Errorf("parseInterfaceInfo: %w", err)
			}

		case "GENERAL.STATE":
			stat.GeneralState = value

		case "GENERAL.CONNECTION":
			stat.Connection = value

		case "IP4.GATEWAY":
			stat.IPGateway = value

		default:
			switch {
			case strings.HasPrefix(key, "IP4.ADDRESS"):
				stat.IPAddresses = append(stat.IPAddresses, value)

			case strings.HasPrefix(key, "IP4.DNS"):
				stat.DNS = append(stat.DNS, value)
			}
		}
	}

	if err = scanner.Err(); err != nil {
		return fmt.Errorf("parseInterfaceInfo: %w", err)
	}

	return nil
}
