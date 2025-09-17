package nslookup

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/rs/zerolog/log"
)

const (
	sdwanSection = "# SDWAN: generated section"
)

type (
	IConfigService interface {
		GetConfig() (cfg config.Config, err error)
	}

	ILookupIPService interface {
		LookupIP(address string) (ips []net.IP, err error)
	}
)

type Service struct {
	configService   IConfigService
	lookupIPService ILookupIPService
	etcHostsPath    string
}

func NewService(configService IConfigService, lookupIPService ILookupIPService, etcHostsPath string) *Service {
	return &Service{
		configService:   configService,
		lookupIPService: lookupIPService,
		etcHostsPath:    etcHostsPath,
	}
}

// SyncHosts refreshes /etc/hosts with actual orchestrator addresses.
func (s *Service) SyncHosts() (err error) {
	log.Info().Msg("SyncHosts: start syncing /etc/hosts")

	cfg, err := s.configService.GetConfig()
	if err != nil {
		return fmt.Errorf("SyncHosts: %w", err)
	}

	if cfg.App == nil || len(cfg.App.OrchestratorAddrs) == 0 {
		// skip if no config
		return nil
	}

	// resolve ips
	var (
		newHosts        = make(hostPairs, 0, len(cfg.App.OrchestratorAddrs))
		orchestratorMap = make(map[string]bool)
	)
	for _, addr := range cfg.App.OrchestratorAddrs {
		orchestratorAddress := strings.ReplaceAll(addr, "http://", "")
		orchestratorAddress = strings.ReplaceAll(orchestratorAddress, "https://", "")
		ips, err := s.lookupIPService.LookupIP(orchestratorAddress)
		if err != nil {
			return fmt.Errorf("SyncHosts: %w", err)
		}

		for _, ip := range ips {
			newHosts = append(newHosts, newHostPair(orchestratorAddress, ip.String()))
		}

		orchestratorMap[orchestratorAddress] = true
	}

	if len(newHosts) == 0 {
		return nil
	}

	// read /etc/hosts file
	stat, err := os.Stat(s.etcHostsPath)
	if err != nil {
		return fmt.Errorf("SyncHosts: %w", err)
	}

	data, err := os.ReadFile(s.etcHostsPath)
	if err != nil {
		return fmt.Errorf("SyncHosts: %w", err)
	}

	// parse existing addresses
	var (
		buf           bytes.Buffer
		scanner       = bufio.NewScanner(bytes.NewReader(data))
		existingHosts = make(map[string]bool)
	)
	for scanner.Scan() {
		// host line example:
		// 127.0.0.1 localhost
		line := strings.TrimSpace(scanner.Text())
		if line == sdwanSection {
			continue
		}

		parts := strings.Split(line, " ")
		if len(parts) > 1 {
			var (
				ip   = strings.TrimSpace(parts[0])
				fqdn = strings.TrimSpace(parts[1])
			)
			// collect only orchestrator addresses
			if orchestratorMap[fqdn] {
				key := newHostPair(fqdn, ip).buildHostLine()
				existingHosts[key] = true
				continue
			}
		}

		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	if err = scanner.Err(); err != nil {
		return fmt.Errorf("SyncHosts: %w", err)
	}

	// search for changes
	hasChanges := len(existingHosts) != len(newHosts)
	if !hasChanges {
		for _, newHost := range newHosts {
			delete(existingHosts, newHost.buildHostLine())
		}

		hasChanges = len(existingHosts) > 0
	}

	if !hasChanges {
		return nil
	}

	// append comment
	buf.WriteByte('\n')
	buf.WriteString(sdwanSection)
	buf.WriteByte('\n')

	// append new hosts
	for _, newHost := range newHosts {
		buf.WriteString(newHost.buildHostLine())
		buf.WriteByte('\n')
	}

	// save updates to file
	if err = os.WriteFile(s.etcHostsPath, buf.Bytes(), stat.Mode().Perm()); err != nil {
		return fmt.Errorf("SyncHosts: %w", err)
	}

	return nil
}
