package discovery

import (
	"fmt"
	"slices"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"

	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

type (
	IHTTPClientService interface {
		CheckPrimary(host string) (isPrimary bool, err error)
	}
)

type Service struct {
	httpClientService IHTTPClientService
	hosts             []string
	mx                sync.Mutex
}

func NewService(httpClientService IHTTPClientService) *Service {
	return &Service{
		httpClientService: httpClientService,
		mx:                sync.Mutex{},
	}
}

func (s *Service) GetHosts() []string {
	s.mx.Lock()
	defer s.mx.Unlock()

	return s.hosts
}

func (s *Service) setHosts(hosts []string) {
	s.mx.Lock()
	defer s.mx.Unlock()

	if slices.Equal(s.hosts, hosts) {
		return
	}

	s.hosts = hosts
}

func (s *Service) FetchPrimary(hosts []string) (primary string, err error) {
	s.setHosts(hosts)

	p := pool.NewWithResults[*discoveryResult]().WithMaxGoroutines(len(hosts))
	for _, host := range hosts {
		p.Go(func() *discoveryResult {
			isPrimary, err := s.httpClientService.CheckPrimary(host)
			if err != nil {
				return &discoveryResult{
					Host: host,
					Err:  fmt.Errorf("check primary error: %w", err),
				}
			}

			return &discoveryResult{
				Host:      host,
				IsPrimary: isPrimary,
			}
		})
	}

	results := p.Wait()
	for _, result := range results {
		if result.Err != nil {
			log.Warn().
				Err(result.Err).
				Any("host", result.Host).
				Msg("FetchPrimary")
		}

		if result.IsPrimary {
			if lo.IsNotEmpty(primary) {
				return primary, errs.ErrSplitBrain
			}

			primary = result.Host
		}
	}

	if lo.IsEmpty(primary) {
		return "", fmt.Errorf("FetchPrimary: %w", errs.ErrPrimaryNotFound)
	}

	return primary, nil
}
