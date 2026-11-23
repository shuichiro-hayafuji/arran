package agentadapter

import (
	"context"
	"errors"
	"testing"

	agent "github.com/shuichiro-hayafuji/arran_agent"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
)

type errorModel struct {
	agent.MockClient
	err error
}

func (m errorModel) GenerateAdvice(context.Context, agent.ConsultationInput) (agent.AdviceDraft, error) {
	return agent.AdviceDraft{}, m.err
}
func (m errorModel) GenerateReview(context.Context, agent.ReviewInput) ([]agent.ReviewCandidate, error) {
	return nil, m.err
}
func (m errorModel) ExtractMemoryCandidates(context.Context, agent.MemoryInput) ([]agent.MemoryItem, error) {
	return nil, m.err
}

func TestSelectedFailuresAreNotConvertedToFallback(t *testing.T) {
	blocked := errors.New("blocked")
	a := NewWithFailureCheck(errorModel{err: blocked}, agent.MockClient{}, "openai", func(err error) bool { return errors.Is(err, blocked) })
	ctx := context.Background()
	if _, _, err := a.Run(ctx, domain.ConsultationContext{}, "", nil); !errors.Is(err, blocked) {
		t.Fatalf("consultation: %v", err)
	}
	if _, err := a.Review(ctx, domain.Profile{}, domain.Dashboard{}, nil, nil, nil); !errors.Is(err, blocked) {
		t.Fatalf("review: %v", err)
	}
	if _, err := a.ExtractMemory(ctx, domain.Consultation{}); !errors.Is(err, blocked) {
		t.Fatalf("memory: %v", err)
	}
	ordinary := errors.New("ordinary failure")
	b := NewWithFailureCheck(errorModel{err: ordinary}, agent.MockClient{}, "openai", func(err error) bool { return errors.Is(err, blocked) })
	if _, source, err := b.Run(ctx, domain.ConsultationContext{}, "", nil); err != nil || source != "fallback" {
		t.Fatalf("ordinary fallback: %s %v", source, err)
	}
}
