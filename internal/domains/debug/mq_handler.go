package debug

import (
	"bytes"
	"runtime/pprof"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/mq"
	"github.com/nats-io/nats.go"
)

type MQHandler struct{}

func NewMQHandler() *MQHandler {
	return new(MQHandler)
}

// DumpHeap dumps heap memory using pprof.
func (h *MQHandler) DumpHeap(_ *nats.Msg) (resp any) {
	var buf bytes.Buffer
	if err := pprof.WriteHeapProfile(&buf); err != nil {
		return mq.NewInternalErrorResponse(err.Error())
	}

	response := struct {
		mq.Response

		Data []byte `json:"data"`
	}{
		Response: mq.NewOkResponse(),
		Data:     buf.Bytes(),
	}

	return response
}
