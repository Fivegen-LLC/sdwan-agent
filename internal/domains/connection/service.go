package connection

import (
	"fmt"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/pony"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/samber/lo"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/entities"
)

type (
	IHealthcheckService interface {
		GetStatus() (status entities.HealthcheckStatus)
	}

	IMessagePublisher interface {
		IsActive() bool
		PublishRequest(method, to string, body any, options ...wschat.RequestOptions) (response wschat.WebsocketMessage, err error)
		Reconnect()
	}

	Service struct {
		publisher IMessagePublisher
	}
)

func NewService(publisher IMessagePublisher) *Service {
	return &Service{
		publisher: publisher,
	}
}

func (s *Service) IsConnectionAlive() (isAlive bool) {
	return s.publisher.IsActive()
}

func (s *Service) OnTunnelStateChanged(data pony.StateChangedInfo) (err error) {
	response, err := s.publisher.PublishRequest(
		constants.MethodUplinkStateChanged, constants.OrchestratorWSID, data,
		wschat.RequestOptions{
			Timeout: lo.ToPtr(5 * time.Second),
		},
	)
	if err != nil {
		return fmt.Errorf("OnTunnelStateChanged: %w", err)
	}

	if response.IsErrorResponse() {
		return fmt.Errorf("OnTunnelStateChanged: %w", response.Error())
	}

	return nil
}

func (s *Service) Reconnect() {
	s.publisher.Reconnect()
}
