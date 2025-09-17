package trunk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/wschat"
	"github.com/go-playground/validator/v10"
)

type (
	IMessagePublisher interface {
		PublishResponse(sourceMessage wschat.WebsocketMessage, body any) (err error)
		PublishErrorResponse(sourceMessage wschat.WebsocketMessage, statusCode int, errMsg string) (err error)
	}

	IConfigService interface {
		UpdateConfig(ctx context.Context, cfg config.Config, updateFuncs ...config.UpdateOption) (err error)
	}

	Handler struct {
		messagePublisher IMessagePublisher
		configService    IConfigService

		validate *validator.Validate
	}
)

func NewHandler(messagePublisher IMessagePublisher, configService IConfigService) *Handler {
	return &Handler{
		messagePublisher: messagePublisher,
		configService:    configService,

		validate: validator.New(),
	}
}

// UpdateConfig updates ISB service configuration.
func (h *Handler) UpdateConfig(request wschat.WebsocketMessage) (err error) {
	defer func() {
		if err != nil {
			if sendErr := h.messagePublisher.PublishErrorResponse(request, http.StatusInternalServerError, err.Error()); sendErr != nil {
				err = errors.Join(sendErr, err)
			}
		}
	}()

	var trunkConfig config.TrunkSection
	if err = json.Unmarshal(request.Body, &trunkConfig); err != nil {
		return fmt.Errorf("UpdateConfig: %w", err)
	}

	ctx := context.Background()
	if err = h.configService.UpdateConfig(
		ctx,
		config.Config{
			Trunk: &trunkConfig,
		},
	); err != nil {
		return fmt.Errorf("UpdateConfig: %w", err)
	}

	if err = h.messagePublisher.PublishResponse(request, []byte{}); err != nil {
		return fmt.Errorf("UpdateConfig: %w", err)
	}

	return nil
}
