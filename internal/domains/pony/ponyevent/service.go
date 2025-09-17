package ponyevent

import (
	"context"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/observable"
)

type (
	IPonyService interface {
		AnyTunnelFailed() *observable.Observable[struct{}]
	}

	IDumpStatService interface {
		DumpStats(dumpKey string)
	}

	Service struct {
		ponyService     IPonyService
		dumpStatService IDumpStatService
	}
)

func NewService(ponyService IPonyService, dumpStatService IDumpStatService) *Service {
	return &Service{
		ponyService:     ponyService,
		dumpStatService: dumpStatService,
	}
}

func (s *Service) StartListenEvents(ctx context.Context) {
	failedTunnelListener := s.ponyService.AnyTunnelFailed().Subscribe()
	defer s.ponyService.AnyTunnelFailed().Unsubscribe(failedTunnelListener)

	for {
		select {
		case <-ctx.Done():
			return
		case <-failedTunnelListener.C():
			s.dumpStatService.DumpStats("tunnel failed")
		}
	}
}
