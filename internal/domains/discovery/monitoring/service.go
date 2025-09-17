package monitoring

import (
	"context"
	"errors"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

const (
	monitoringInterval = time.Second * 20
)

type (
	IMessagePublisher interface {
		IsActive() bool
		Reconnect()
	}

	IConfigService interface {
		GetConfig() (cfg config.Config, err error)
	}

	IDiscoveryService interface {
		FetchPrimary(hosts []string) (primary string, err error)
		GetHosts() []string
	}
)

type Service struct {
	messagePublisher IMessagePublisher
	configService    IConfigService
	discoveryService IDiscoveryService
}

func NewService(messagePublisher IMessagePublisher, configService IConfigService, discoveryService IDiscoveryService) *Service {
	return &Service{
		messagePublisher: messagePublisher,
		configService:    configService,
		discoveryService: discoveryService,
	}
}

func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(monitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cfg, err := s.configService.GetConfig()
			if err != nil {
				log.Error().Err(err).Msg("StartMonitoring: read config error")
				continue
			}

			if lo.IsEmpty(cfg.App.ActiveOrchestratorAddr) {
				log.Debug().Msg("StartMonitoring: no active orchestrator")
				continue
			}

			hosts := s.discoveryService.GetHosts()
			if len(hosts) == 0 {
				log.Debug().Msg("StartMonitoring: no hosts")
				continue
			}

			primary, err := s.discoveryService.FetchPrimary(hosts)
			if err != nil {
				log.Error().Err(err).Msg("StartMonitoring: fetch primary error")
				if errors.Is(err, errs.ErrSplitBrain) {
					s.messagePublisher.Reconnect()
				}
				continue
			}

			if primary != cfg.App.ActiveOrchestratorAddr && s.messagePublisher.IsActive() {
				// try to reconnect to another host
				s.messagePublisher.Reconnect()
			}
		}
	}
}
