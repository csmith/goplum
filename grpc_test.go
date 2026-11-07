package goplum

import (
	"testing"
	"time"

	"chameth.com/goplum/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
