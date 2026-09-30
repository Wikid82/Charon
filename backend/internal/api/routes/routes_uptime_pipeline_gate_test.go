package routes

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/dbmaint"
)

// pipelineProbe records when each of the six uptime goroutines actually starts.
type pipelineProbe struct {
	started map[string]chan struct{}
}

func newPipelineProbe() *pipelineProbe {
	p := &pipelineProbe{started: map[string]chan struct{}{}}
	for _, name := range []string{"bootstrap", "ingester", "pool", "scheduler", "sync loop", "pruner"} {
		p.started[name] = make(chan struct{})
	}
	return p
}

func (p *pipelineProbe) starter(name string) func(context.Context) {
	return func(context.Context) { close(p.started[name]) }
}

func (p *pipelineProbe) starters() uptimeStarters {
	return uptimeStarters{
		Bootstrap: p.starter("bootstrap"),
		Ingester:  p.starter("ingester"),
		Pool:      p.starter("pool"),
		Scheduler: p.starter("scheduler"),
		SyncLoop:  p.starter("sync loop"),
		Pruner:    p.starter("pruner"),
	}
}

func (p *pipelineProbe) hasStarted(name string) bool {
	select {
	case <-p.started[name]:
		return true
	default:
		return false
	}
}

func (p *pipelineProbe) awaitAll(t *testing.T) {
	t.Helper()
	for name, ch := range p.started {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s did not start after the gate became idle", name)
		}
	}
}

// Each of the six goroutines of the uptime pipeline waits for the maintenance
// gate and starts promptly once it is idle (GH #1422, 3.4 step 5).
func TestStartUptimePipeline_EachGoroutineWaitsForTheGate(t *testing.T) {
	for _, hold := range []struct {
		name string
		hold func(t *testing.T, g *dbmaint.Gate)
	}{
		{"planned", func(_ *testing.T, g *dbmaint.Gate) { g.MarkPlanned() }},
		{"converting", func(t *testing.T, g *dbmaint.Gate) {
			g.MarkPlanned()
			require.NoError(t, g.BeginChecking())
			require.NoError(t, g.BeginConverting(1))
		}},
	} {
		t.Run(hold.name, func(t *testing.T) {
			gate := dbmaint.NewGate()
			hold.hold(t, gate)
			probe := newPipelineProbe()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			done := startUptimePipeline(ctx, gate, probe.starters())

			time.Sleep(60 * time.Millisecond)
			for name := range probe.started {
				assert.False(t, probe.hasStarted(name), "%s must wait while the gate is not idle", name)
			}
			select {
			case <-done:
				t.Fatal("the ingester-done channel must stay open while the ingester has not run")
			default:
			}

			gate.Release()

			probe.awaitAll(t)
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("the ingester-done channel did not close after the ingester returned")
			}
		})
	}
}

func TestStartUptimePipeline_StartsAtOnceOnAnIdleOrNilGate(t *testing.T) {
	for name, gate := range map[string]*dbmaint.Gate{"idle": dbmaint.NewGate(), "nil": nil} {
		t.Run(name, func(t *testing.T) {
			probe := newPipelineProbe()
			startUptimePipeline(context.Background(), gate, probe.starters())
			probe.awaitAll(t)
		})
	}
}

func TestStartUptimePipeline_ShutdownWhileDeferredStartsNothingAndStillClosesIngesterDone(t *testing.T) {
	gate := dbmaint.NewGate()
	gate.MarkPlanned()
	probe := newPipelineProbe()
	ctx, cancel := context.WithCancel(context.Background())

	done := startUptimePipeline(ctx, gate, probe.starters())
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the ingester-done channel must close on shutdown even though the ingester never ran")
	}
	time.Sleep(40 * time.Millisecond)
	for name := range probe.started {
		assert.False(t, probe.hasStarted(name), "%s must not start after shutdown", name)
	}
}
