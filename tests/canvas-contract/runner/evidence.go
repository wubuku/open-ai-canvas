package runner

import (
	"context"
	"encoding/json"
	"testing"

	"canvas-contract/contract"
)

type evidenceContextKey struct{}

type faultContextKey struct{}

type requestFault struct {
	ID       string
	Declared int64
	Sent     int
	Loss     func() ResponseLossWitness
}

type requestEvidence struct {
	Method         string                 `json:"method"`
	Path           string                 `json:"path"`
	Status         int                    `json:"status"`
	BodyComplete   bool                   `json:"body_complete"`
	TransportFail  bool                   `json:"transport_failure"`
	Wire           *contract.WireEvidence `json:"wire,omitempty"`
	RequestVariant string                 `json:"request_variant,omitempty"`
	RequestFault   string                 `json:"request_fault,omitempty"`
	DeclaredBytes  int64                  `json:"declared_body_bytes,omitempty"`
	SentBytes      int                    `json:"sent_body_bytes,omitempty"`
	ResponseLoss   *ResponseLossWitness   `json:"response_loss,omitempty"`
}

func Context(test *testing.T) context.Context {
	return context.WithValue(test.Context(), evidenceContextKey{}, test)
}

func recordRequest(ctx context.Context, method, path string, status int, bodyComplete, transportFail bool, wire ...contract.WireEvidence) {
	logger, ok := ctx.Value(evidenceContextKey{}).(interface{ Logf(string, ...any) })
	if !ok {
		return
	}
	fact := requestEvidence{
		Method:         method,
		Path:           safeRequestPath(path),
		Status:         status,
		BodyComplete:   bodyComplete,
		TransportFail:  transportFail,
		RequestVariant: contract.RequestVariant(method, path),
	}
	if fault, ok := ctx.Value(faultContextKey{}).(requestFault); ok {
		fact.RequestFault = fault.ID
		fact.DeclaredBytes = fault.Declared
		fact.SentBytes = fault.Sent
		if fault.Loss != nil {
			witness := fault.Loss()
			fact.ResponseLoss = &witness
		}
	}
	if len(wire) != 0 {
		fact.Wire = &wire[0]
	}
	encoded, err := json.Marshal(fact)
	if err == nil {
		logger.Logf("CANVAS_CONTRACT_HTTP %s", encoded)
	}
}
