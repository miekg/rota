package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"chromium.googlesource.com/infra/rotang"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/server/router"
)

type calResult struct {
	Success bool
	Message string
}

type testRes struct {
	Service calResult
	Legacy  calResult
}

// HandleCalTest tests calendar access.
func (h *State) HandleCalTest(ctx *router.Context) {
	rota, err := h.rota(ctx)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}

	if rota.Config.Enabled {
		http.Error(ctx.Writer, "tests on enabled rotation not allowed", http.StatusInternalServerError)
		return
	}

	now := clock.Now(ctx.Context)

	var res bytes.Buffer
	if err := json.NewEncoder(&res).Encode(testRes{
		Service: *testCal(ctx, *rota, h.calendar, now),
		Legacy:  *testCal(ctx, *rota, h.legacyCalendar, now),
	}); err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}

	io.Copy(ctx.Writer, &res)
}

const (
	testEvent = " - Test Event generated by RotaNG - https://rota-ng.appspot.com"
)

func testCal(ctx *router.Context, rota rotang.Configuration, cal rotang.Calenderer, start time.Time) *calResult {
	rota.Config.Name += testEvent
	shifts, err := cal.CreateEvent(ctx, &rota, []rotang.ShiftEntry{
		{
			Name:      "Test Event",
			StartTime: start,
			EndTime:   start.Add(fullDay),
			Comment:   "Test Event",
		},
	}, false)
	if err != nil {
		return &calResult{
			Success: false,
			Message: err.Error() + " - create event failed",
		}
	}
	for _, s := range shifts {
		if err := cal.DeleteEvent(ctx, &rota, &s); err != nil {
			return &calResult{
				Success: false,
				Message: err.Error() + " - delete event failed",
			}
		}
	}

	return &calResult{
		Success: true,
		Message: "Calendar permissions OK",
	}
}
