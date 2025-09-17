package websocket

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/observable"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type (
	IMessagePublisher interface {
		IsActive() bool
		IsClosed() bool

		Start()
		Stop()
		ListenRequests() <-chan wschat.WebsocketMessage
		ConnectionStateChanged() *observable.Observable[wschat.ConnectionState]
		GotUnhandledErrors() *observable.Observable[error]

		PublishRequest(method, to string, body any, options ...wschat.RequestOptions) (response wschat.WebsocketMessage, err error)
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
		PublishControl(messageType int, data []byte, deadline time.Time) (err error)
	}

	IDumpStatService interface {
		DumpStats(dumpKey string)
	}

	WsHandler func(request wschat.WebsocketMessage) error

	Service struct {
		messagePublisher IMessagePublisher
		dumpStatService  IDumpStatService
		pingPeriod       time.Duration

		routes map[string]WsHandler
	}
)

func NewService(messagePublisher IMessagePublisher, dumpStatService IDumpStatService, pingPeriod time.Duration) *Service {
	service := &Service{
		messagePublisher: messagePublisher,
		dumpStatService:  dumpStatService,
		pingPeriod:       pingPeriod,

		routes: map[string]WsHandler{},
	}

	go service.run()
	return service
}

func (s *Service) SetRoutes(routes map[string]WsHandler) {
	s.routes = routes
}

func (s *Service) IsStarted() bool {
	return !s.messagePublisher.IsClosed()
}

// Start starts message publisher (websocket connection).
func (s *Service) Start() (err error) {
	if s.IsStarted() {
		return fmt.Errorf("Start: publisher already started")
	}

	s.messagePublisher.Start()
	return nil
}

// Stop stops message publisher (websocket connection).
func (s *Service) Stop() (err error) {
	if !s.IsStarted() {
		return fmt.Errorf("Stop: publisher already stopped")
	}

	s.messagePublisher.Stop()
	return nil
}

func (s *Service) run() {
	ticker := time.NewTicker(s.pingPeriod)
	defer ticker.Stop()

	connectionStateChanged := s.messagePublisher.ConnectionStateChanged().Subscribe()
	gotUnhandledErrors := s.messagePublisher.GotUnhandledErrors().Subscribe()
	defer func() {
		s.messagePublisher.ConnectionStateChanged().Unsubscribe(connectionStateChanged)
		s.messagePublisher.GotUnhandledErrors().Unsubscribe(gotUnhandledErrors)
	}()

	for {
		select {
		case request := <-s.messagePublisher.ListenRequests():
			log.Trace().
				Any("websocket message", request).
				Msg("run: got websocket message")

			handler, ok := s.routes[request.Method]
			if !ok {
				if err := s.messagePublisher.PublishErrorResponse(request, http.StatusMethodNotAllowed, "method not allowed"); err != nil {
					log.Error().Err(err).Msg("run")
				}

				break
			}

			go func() {
				if err := handler(request); err != nil {
					log.Error().
						Err(err).
						Msg("run")
				}
			}()

		case newState := <-connectionStateChanged.C():
			log.Debug().
				Any("new state", newState).
				Msg("run: connection state changed")

			if newState == wschat.ConnectionStateClosed {
				s.dumpStatService.DumpStats("websocket connection closed")
			}

		case err := <-gotUnhandledErrors.C():
			log.Error().
				Err(err).
				Msg("run")

		case <-ticker.C:
			if !s.messagePublisher.IsActive() {
				break
			}

			if err := s.messagePublisher.PublishControl(websocket.PingMessage, nil, time.Now().Add(s.pingPeriod)); err != nil {
				log.Error().
					Err(err).
					Msg("run: ping websocket failed")
			}
		}
	}
}
