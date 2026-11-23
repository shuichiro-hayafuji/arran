package agentadapter

import (
	"context"
	agent "github.com/shuichiro-hayafuji/arran_agent"
)

type failureKey struct{}
type failureCapture struct{ err error }

func captureFailure(ctx context.Context) (context.Context, *failureCapture) {
	f := &failureCapture{}
	return context.WithValue(ctx, failureKey{}, f), f
}

type trackedModel struct {
	model agent.Model
	check func(error) bool
}

func (m trackedModel) save(ctx context.Context, err error) {
	if err != nil && m.check != nil && m.check(err) {
		if f, ok := ctx.Value(failureKey{}).(*failureCapture); ok {
			f.err = err
		}
	}
}
func (m trackedModel) GenerateAdvice(ctx context.Context, in agent.ConsultationInput) (agent.AdviceDraft, error) {
	out, err := m.model.GenerateAdvice(ctx, in)
	m.save(ctx, err)
	return out, err
}

type revisionModel interface {
	GenerateAdviceRevision(context.Context, agent.AdviceRevisionInput) (agent.AdviceDraft, error)
}
type trackedRevision struct {
	trackedModel
	revision revisionModel
}

func (m trackedRevision) GenerateAdviceRevision(ctx context.Context, in agent.AdviceRevisionInput) (agent.AdviceDraft, error) {
	out, err := m.revision.GenerateAdviceRevision(ctx, in)
	m.save(ctx, err)
	return out, err
}
func (m trackedModel) InferCategory(ctx context.Context, text string, categories []string) (string, error) {
	return m.model.InferCategory(ctx, text, categories)
}
func (m trackedModel) GenerateReview(ctx context.Context, in agent.ReviewInput) ([]agent.ReviewCandidate, error) {
	out, err := m.model.GenerateReview(ctx, in)
	m.save(ctx, err)
	return out, err
}
func (m trackedModel) ExtractMemoryCandidates(ctx context.Context, in agent.MemoryInput) ([]agent.MemoryItem, error) {
	out, err := m.model.ExtractMemoryCandidates(ctx, in)
	m.save(ctx, err)
	return out, err
}
