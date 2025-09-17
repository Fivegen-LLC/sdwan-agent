package dumpstat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell/commands"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

type (
	IConfigService interface {
		GetConfig() (cfg config.Config, err error)
	}

	IShellService interface {
		ExecWithStdout(command shell.ICommand, stdout io.Writer) (err error)
	}

	Service struct {
		configService IConfigService
		shellService  IShellService

		ipRouteCmd *commands.ListIPRoutesCmd
		ipRuleCmd  *commands.CustomCmd
	}
)

func NewService(configService IConfigService, shellService IShellService) *Service {
	return &Service{
		configService: configService,
		shellService:  shellService,

		ipRouteCmd: commands.NewListIPRoutesCmd(),
		ipRuleCmd:  commands.NewCustomCmd("ip rule"),
	}
}

func (s *Service) DumpStats(dumpKey string) {
	cfg, err := s.configService.GetConfig()
	if err != nil {
		log.Error().Err(err).Msg("DumpStats: read config error")
		return
	}

	var (
		buf bytes.Buffer
		msg = log.Info()
		c   = newCounter()
	)
	// dump ip routes
	if err = s.shellService.ExecWithStdout(s.ipRouteCmd, &buf); err == nil {
		if err = s.parseLineByLine(&buf, func(line string) {
			msg.Str(fmt.Sprintf("%d. ip route", c.get()), line)
		}); err != nil {
			log.Error().Err(err).Msg("DumpStats: parse ip routes error")
		}
	} else {
		log.Error().Err(err).Msg("DumpStats: get ip routes error")
	}

	// dump ip rules
	buf.Reset()
	if err = s.shellService.ExecWithStdout(s.ipRuleCmd, &buf); err == nil {
		if err = s.parseLineByLine(&buf, func(line string) {
			msg.Str(fmt.Sprintf("%d. ip rule", c.get()), line)
		}); err != nil {
			log.Error().Err(err).Msg("DumpStats: parse ip rule error")
		}
	} else {
		log.Error().Err(err).Msg("DumpStats: get ip rules error")
	}

	if cfg.Port != nil {
		for _, port := range cfg.Port.PortConfigs {
			// dump admin state
			if cfg.AdminState != nil {
				if adminState, found := lo.Find(cfg.AdminState.AdminStatePorts, func(item config.AdminStatePort) bool {
					return item.PortName == port.Name
				}); found {
					stateStr := "up"
					if adminState.IsDown {
						stateStr = "down"
					}

					msg.Str(fmt.Sprintf("%d. admin state %s", c.get(), port.Name), stateStr)
				}
			}

			// dump ip address
			buf.Reset()
			showIPAddrCmd := commands.NewCustomCmd(fmt.Sprintf("ip --json a show %s", port.Name))
			if err = s.shellService.ExecWithStdout(showIPAddrCmd, &buf); err == nil {
				if err = s.parseOperState(&buf, func(key, value string) {
					msg.Str(fmt.Sprintf("%d. %s %s", c.get(), key, port.Name), value)
				}); err != nil {
					log.Error().
						Err(err).
						Str("port", port.Name).
						Msg("DumpStats: parse ip address error")
				}
			} else {
				log.Error().Err(err).Msg("DumpStats: get ip address error")
			}

			// get ip route from tables
			for _, tableID := range port.TableIDs {
				buf.Reset()
				showRouteCmd := commands.NewListIPRoutesCmd().WithTable(tableID)
				if err = s.shellService.ExecWithStdout(showRouteCmd, &buf); err == nil {
					if err = s.parseLineByLine(&buf, func(line string) {
						msg.Str(fmt.Sprintf("%d. ip route table %d (%s)", c.get(), tableID, port.Name), line)
					}); err != nil {
						log.Error().
							Err(err).
							Int("table", tableID).
							Msg("DumpStats: parse ip routes for table error")
					}
				} else {
					log.Error().
						Err(err).
						Int("table", tableID).
						Msg("DumpStats: get ip routes for table error")
				}
			}
		}
	}

	// dump tunnel status
	if cfg.Pony != nil {
		for _, cluster := range cfg.Pony.Clusters {
			msg.Str(fmt.Sprintf("%d. active tunnel (cluster: %s)", c.get(), cluster.Network), cluster.State.ActiveTunnel)
			for ip, state := range cluster.State.LocalStates {
				stateStr := "up"
				if !state {
					stateStr = "down"
				}

				msg.Str(fmt.Sprintf("%d. tunnel state %s (cluster: %s)", c.get(), ip, cluster.Network), stateStr)
			}
		}
	}

	msg.Str("dump key", dumpKey).
		Msg("DumpStats: system network dump")
}

func (s *Service) parseLineByLine(buf *bytes.Buffer, fn func(line string)) (err error) {
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.ReplaceAll(line, "\t", " ")
		fn(line)
	}

	if err = scanner.Err(); err != nil {
		return fmt.Errorf("parseLineByLine: %w", err)
	}

	return nil
}

func (s *Service) parseOperState(buf *bytes.Buffer, fn func(key, value string)) (err error) {
	var stats LinuxInterfaces
	if err = json.Unmarshal(buf.Bytes(), &stats); err != nil {
		return fmt.Errorf("parseOperState: %w", err)
	}

	for _, stat := range stats {
		fn("link state", strings.ToLower(stat.State))
		fn("mtu", strconv.Itoa(stat.MTU))

		for _, addr := range stat.AddrInfo {
			fn("ip address", fmt.Sprintf("%s/%d", addr.Addr, addr.Prefix))
		}
	}

	return nil
}
