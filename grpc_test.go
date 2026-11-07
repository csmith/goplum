package goplum

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"chameth.com/goplum/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestGrpcServer_GetCheckIncludesHistory(t *testing.T) {
	p := NewPlum()
	check := &ScheduledCheck{Name: "web"}
	check.AddResult(&Result{State: StateGood, Time: time.Unix(100, 0)})
	check.AddResult(&Result{State: StateFailing, Time: time.Unix(200, 0), Detail: "timed out"})
	p.Checks["web"] = check

	res, err := NewGrpcServer(p).GetCheck(t.Context(), &api.CheckName{Name: "web"})
	require.NoError(t, err)

	require.Len(t, res.GetHistory(), 2)
	assert.Equal(t, int64(200), res.GetHistory()[0].GetTime())
	assert.Equal(t, api.Status_FAILING, res.GetHistory()[0].GetResult())
	assert.Equal(t, "timed out", res.GetHistory()[0].GetDetail())
	assert.Equal(t, int64(100), res.GetHistory()[1].GetTime())
	assert.Equal(t, api.Status_GOOD, res.GetHistory()[1].GetResult())
}

func TestGrpcServer_GetCheckSendsCheckTimeAsInt(t *testing.T) {
	p := NewPlum()
	check := &ScheduledCheck{
		Name:   "web",
		Check:  goodCheck{},
		Config: &CheckSettings{Timeout: time.Second},
	}
	p.Checks["web"] = check
	p.RunCheck(check)

	res, err := NewGrpcServer(p).GetCheck(t.Context(), &api.CheckName{Name: "web"})
	require.NoError(t, err)

	require.Len(t, res.GetHistory(), 1)
	facts := res.GetHistory()[0].GetFacts()
	require.Len(t, facts, 1)
	assert.Equal(t, string(CheckTime), facts[0].GetName())
	require.IsType(t, &api.Fact_Int{}, facts[0].GetValue())
	assert.Equal(t, int64(check.LastResult().Facts[CheckTime].(time.Duration)), facts[0].GetInt())
}

func TestGrpcServer_ResumeCheckSchedulesCheckWithoutWaiting(t *testing.T) {
	synctest.Test(
		t,
		func(t *testing.T) {
			// With no runners, scheduled checks stay queued for the test to inspect.
			defer func(old int) { *runners = old }(*runners)
			*runners = 0

			p := NewPlum()
			p.Checks["web"] = &ScheduledCheck{
				Name:      "web",
				Config:    &CheckSettings{Interval: time.Second},
				Suspended: true,
			}

			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan struct{})
			go func() {
				defer close(done)
				p.Run(ctx)
			}()
			synctest.Wait()
			require.Empty(t, p.scheduled)

			start := time.Now()
			_, err := NewGrpcServer(p).ResumeCheck(t.Context(), &api.CheckName{Name: "web"})
			require.NoError(t, err)
			synctest.Wait()

			assert.Len(t, p.scheduled, 1)
			assert.Equal(t, start, time.Now())

			cancel()
			<-done
		},
	)
}

type goodCheck struct{}

func (goodCheck) Execute(context.Context) Result {
	return GoodResult()
}

type resultsStream struct {
	grpc.ServerStream
	ctx  context.Context
	sent chan *api.Result
}

func (r *resultsStream) Context() context.Context {
	return r.ctx
}

func (r *resultsStream) Send(res *api.Result) error {
	select {
	case r.sent <- res:
	default:
	}
	return nil
}

func TestGrpcServer_ResultsSurvivesClientDisconnectWhileChecksRun(t *testing.T) {
	p := NewPlum()
	server := NewGrpcServer(p)

	ctx, cancel := context.WithCancel(t.Context())
	stream := &resultsStream{ctx: ctx, sent: make(chan *api.Result, 1)}
	done := make(chan error, 1)
	go func() {
		done <- server.Results(&api.Empty{}, stream)
	}()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 8 {
		check := &ScheduledCheck{
			Name:   fmt.Sprintf("check-%d", i),
			Check:  goodCheck{},
			Config: &CheckSettings{Timeout: time.Second},
		}
		wg.Go(
			func() {
				for {
					select {
					case <-stop:
						return
					default:
						p.RunCheck(check)
					}
				}
			},
		)
	}

	select {
	case <-stream.sent:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "no results streamed")
	}
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "Results did not return after the client disconnected")
	}

	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}
