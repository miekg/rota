package handlers

import (
	"chromium.googlesource.com/infra/rotang"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"context"

	"github.com/julienschmidt/httprouter"
	"github.com/kylelemons/godebug/pretty"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/server/router"
)

func TestHandleLegacy(t *testing.T) {
	ctx := newTestContext()
	ctxCancel, cancel := context.WithCancel(ctx)
	cancel()

	var f trooperFake

	tests := []struct {
		name       string
		fail       bool
		ctx        *router.Context
		lm         map[string]func(*router.Context, string) (string, error)
		fakeFail   bool
		fakeReturn string
	}{{
		name: "Canceled Context",
		fail: true,
		ctx: &router.Context{
			Context: ctxCancel,
			Writer:  httptest.NewRecorder(),
		},
	}, {
		name: "Success",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Params: httprouter.Params{
				{
					Key:   "name",
					Value: "trooper.js",
				},
			},
		},
		lm: map[string]func(*router.Context, string) (string, error){
			"trooper.js": f.troopers,
		},
	}, {
		name: "Name not in the map",
		fail: true,
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Params: httprouter.Params{
				{
					Key:   "name",
					Value: "trooper.js",
				},
			},
		},
		lm: map[string]func(*router.Context, string) (string, error){
			"not_trooper.js": f.troopers,
		},
	}, {
		name:     "Func fail",
		fail:     true,
		fakeFail: true,
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Params: httprouter.Params{
				{
					Key:   "name",
					Value: "trooper.js",
				},
			},
		},
		lm: map[string]func(*router.Context, string) (string, error){
			"trooper.js": f.troopers,
		},
	},
	}

	h := testSetup(t)

	for _, tst := range tests {
		f.fail = tst.fakeFail
		f.ret = tst.fakeReturn
		h.legacyMap = tst.lm

		h.HandleLegacy(tst.ctx)

		recorder := tst.ctx.Writer.(*httptest.ResponseRecorder)
		if got, want := (recorder.Code != http.StatusOK), tst.fail; got != want {
			t.Errorf("%s: HandleLegacy(ctx) = %t want: %t, code: %v", tst.name, got, want, recorder.Code)
			continue
		}
	}

}

func TestLegacySheriff(t *testing.T) {
	ctx := newTestContext()

	tests := []struct {
		name       string
		fail       bool
		calFail    bool
		time       time.Time
		calShifts  []rotang.ShiftEntry
		ctx        *router.Context
		file       string
		memberPool []rotang.Member
		cfgs       []*rotang.Configuration
		want       string
	}{{
		name: "Success JS",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		file: "sheriff.js",
		time: midnight,
		cfgs: []*rotang.Configuration{
			{
				Config: rotang.Config{
					Name: "Build Sheriff",
				},
			},
		},
		calShifts: []rotang.ShiftEntry{
			{
				StartTime: midnight,
				EndTime:   midnight.Add(5 * fullDay),
				OnCall: []rotang.ShiftMember{
					{
						Email: "test1@oncall.com",
					}, {
						Email: "test2@oncall.com",
					},
				},
			}, {
				StartTime: midnight.Add(5 * fullDay),
				EndTime:   midnight.Add(10 * fullDay),
				OnCall: []rotang.ShiftMember{
					{
						Email: "test3@oncall.com",
					}, {
						Email: "test4@oncall.com",
					},
				},
			},
		},
		want: "document.write('test1, test2');",
	}, {
		name: "Success JSON",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		file: "sheriff.json",
		time: midnight.Add(6 * fullDay),
		cfgs: []*rotang.Configuration{
			{
				Config: rotang.Config{
					Name: "Build Sheriff",
				},
			},
		},
		calShifts: []rotang.ShiftEntry{
			{
				StartTime: midnight,
				EndTime:   midnight.Add(5 * fullDay),
				OnCall: []rotang.ShiftMember{
					{
						Email: "test1@oncall.com",
					}, {
						Email: "test2@oncall.com",
					},
				},
			}, {
				StartTime: midnight.Add(5 * fullDay),
				EndTime:   midnight.Add(10 * fullDay),
				OnCall: []rotang.ShiftMember{
					{
						Email: "test3@oncall.com",
					}, {
						Email: "test4@oncall.com",
					},
				},
			},
		},
		want: `{"updated_unix_timestamp":1144454400,"emails":["test3@oncall.com","test4@oncall.com"]}
`,
	}, {
		name: "File not supported",
		fail: true,
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		file: "sheriff_not_supported.js",
		time: midnight,
	}, {
		name: "Config not found",
		fail: true,
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		file: "sheriff.js",
		time: midnight,
	}, {
		name:    "Calendar fail",
		fail:    true,
		calFail: true,
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		file: "sheriff.json",
		time: midnight.Add(6 * fullDay),
		cfgs: []*rotang.Configuration{
			{
				Config: rotang.Config{
					Name: "Build Sheriff",
				},
			},
		},
	},
	}

	h := testSetup(t)

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			for _, m := range tst.memberPool {
				if err := h.memberStore(ctx).CreateMember(ctx, &m); err != nil {
					t.Fatalf("%s: CreateMember(ctx, _) failed: %v", tst.name, err)
				}
				defer h.memberStore(ctx).DeleteMember(ctx, m.Email)
			}
			for _, cfg := range tst.cfgs {
				if err := h.configStore(ctx).CreateRotaConfig(ctx, cfg); err != nil {
					t.Fatalf("%s: CreateRotaConfig(ctx, _) failed: %v", tst.name, err)
				}
				defer h.configStore(ctx).DeleteRotaConfig(ctx, cfg.Config.Name)
			}

			h.legacyCalendar.(*fakeCal).Set(tst.calShifts, tst.calFail, false, 0)

			tst.ctx.Context = clock.Set(tst.ctx.Context, testclock.New(tst.time))

			res, err := h.legacySheriff(tst.ctx, tst.file)
			if got, want := (err != nil), tst.fail; got != want {
				t.Fatalf("%s: h.legacySheriff(ctx, %q) = %t want: %t, err: %v", tst.name, tst.file, got, want, err)
			}
			if err != nil {
				return
			}

			if diff := pretty.Compare(tst.want, res); diff != "" {
				t.Fatalf("%s: h.legacySheriff(ctx, %q) differ -want +got, \n%s", tst.name, tst.file, diff)
			}
		})
	}
}

func TestLegacyTroopers(t *testing.T) {
	ctx := newTestContext()

	tests := []struct {
		name       string
		fail       bool
		calFail    bool
		ctx        *router.Context
		file       string
		oncallers  []string
		updateTime time.Time
		want       string
	}{{
		name: "Success JS",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file:       "trooper.js",
		oncallers:  []string{"primary1", "secondary1", "secondary2"},
		updateTime: midnight,
		want:       "document.write('primary1, secondary: secondary1, secondary2');",
	}, {
		name: "Success JSON",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file:       "current_trooper.json",
		oncallers:  []string{"primary1", "secondary1", "secondary2"},
		updateTime: midnight,
		want:       `{"primary":"primary1","secondaries":["secondary1","secondary2"],"updated_unix_timestamp":1143936000}` + "\n",
	}, {
		name: "Success trooper.json",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file:       "current_trooper.json",
		oncallers:  []string{"primary1", "secondary1", "secondary2"},
		updateTime: midnight,
		want:       `{"primary":"primary1","secondaries":["secondary1","secondary2"],"updated_unix_timestamp":1143936000}` + "\n",
	}, {
		name: "Success txt",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file:       "current_trooper.txt",
		oncallers:  []string{"primary1", "secondary1", "secondary2"},
		updateTime: midnight,
		want:       "primary1,secondary1,secondary2",
	}, {
		name:    "Calendar fail",
		fail:    true,
		calFail: true,
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file:       "current_trooper.txt",
		oncallers:  []string{"primary1", "secondary1", "secondary2"},
		updateTime: midnight,
	}, {
		name: "Unknown file",
		fail: true,
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file:       "unknown_trooper.txt",
		oncallers:  []string{"primary1", "secondary1", "secondary2"},
		updateTime: midnight,
	},
	}

	h := testSetup(t)

	for _, tst := range tests {
		h.legacyCalendar.(*fakeCal).SetTroopers(tst.oncallers, tst.calFail)

		tst.ctx.Context = clock.Set(tst.ctx.Context, testclock.New(tst.updateTime))

		resStr, err := h.legacyTrooper(tst.ctx, tst.file)
		if got, want := (err != nil), tst.fail; got != want {
			t.Errorf("%s: legacyTrooper(ctx) = %t want: %t, err: %v", tst.name, got, want, err)
			continue
		}
		if err != nil {
			continue
		}

		if diff := pretty.Compare(tst.want, resStr); diff != "" {
			t.Errorf("%s: legacyTrooper(ctx) differ -want +got,\n%s", tst.name, diff)
		}
	}

}

func TestLegacyTroopersByRotation(t *testing.T) {
	ctx := newTestContext()

	tests := []struct {
		name      string
		fail      bool
		calFail   bool
		ctx       *router.Context
		file      string
		calShifts []rotang.ShiftEntry
		time      time.Time
		want      string
	}{{
		name: "Success chrome-ops-devx.json",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file: "chrome-ops-devx.json",
		calShifts: []rotang.ShiftEntry{
			{
				StartTime: midnight,
				EndTime:   midnight.Add(5 * fullDay),
				OnCall: []rotang.ShiftMember{
					{Email: "primary@oncall.com"},
					{Email: "secondary1@oncall.com"},
					{Email: "secondary2@oncall.com"},
				},
			},
		},
		want: `{"primary":"primary@oncall.com","secondaries":["secondary1@oncall.com","secondary2@oncall.com"],"updated_unix_timestamp":1143936000}` + "\n",
	}, {
		name: "Success chrome-ops-foundation.json",
		calShifts: []rotang.ShiftEntry{
			{
				StartTime: midnight,
				EndTime:   midnight.Add(5 * fullDay),
				OnCall: []rotang.ShiftMember{
					{Email: "primary@oncall.com"},
					{Email: "secondary1@oncall.com"},
					{Email: "secondary2@oncall.com"},
				},
			},
		},
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file: "chrome-ops-foundation.json",
		want: `{"primary":"primary@oncall.com","secondaries":["secondary1@oncall.com","secondary2@oncall.com"],"updated_unix_timestamp":1143936000}` + "\n",
	}, {
		name: "Success chrome-ops-client-infra.json",
		calShifts: []rotang.ShiftEntry{
			{
				StartTime: midnight,
				EndTime:   midnight.Add(5 * fullDay),
				OnCall: []rotang.ShiftMember{
					{Email: "primary@oncall.com"},
					{Email: "secondary1@oncall.com"},
					{Email: "secondary2@oncall.com"},
				},
			},
		},
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file: "chrome-ops-client-infra.json",
		want: `{"primary":"primary@oncall.com","secondaries":["secondary1@oncall.com","secondary2@oncall.com"],"updated_unix_timestamp":1143936000}` + "\n",
	}, {
		name: "Success chrome-ops-sre.json",
		calShifts: []rotang.ShiftEntry{
			{
				StartTime: midnight,
				EndTime:   midnight.Add(5 * fullDay),
				OnCall: []rotang.ShiftMember{
					{Email: "primary@oncall.com"},
					{Email: "secondary1@oncall.com"},
					{Email: "secondary2@oncall.com"},
				},
			},
		},
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file: "chrome-ops-sre.json",
		want: `{"primary":"primary@oncall.com","secondaries":["secondary1@oncall.com","secondary2@oncall.com"],"updated_unix_timestamp":1143936000}` + "\n",
	}, {
		name:    "Calendar fail",
		fail:    true,
		calFail: true,
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file: "chrome-ops-sre.txt",
	}, {
		name: "Unknown file",
		fail: true,
		time: midnight,
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
			Request: httptest.NewRequest("GET", "/legacy", nil),
		},
		file: "unknown_trooper.txt",
	},
	}

	h := testSetup(t)

	for _, tst := range tests {
		h.legacyCalendar.(*fakeCal).Set(tst.calShifts, tst.calFail, false, 0)

		tst.ctx.Context = clock.Set(tst.ctx.Context, testclock.New(tst.time))

		resStr, err := h.legacyTrooperByRotation(tst.ctx, tst.file)
		if got, want := (err != nil), tst.fail; got != want {
			t.Errorf("%s: legacyTrooper(ctx) = %t want: %t, err: %v", tst.name, got, want, err)
			continue
		}
		if err != nil {
			continue
		}

		if diff := pretty.Compare(tst.want, resStr); diff != "" {
			t.Errorf("%s: legacyTrooper(ctx) differ -want +got,\n%s", tst.name, diff)
		}
	}

}

func TestLegacyAllRotations(t *testing.T) {
	ctx := newTestContext()

	tests := []struct {
		name      string
		fail      bool
		calFail   bool
		rotaMap   map[string][2]string
		calShifts []rotang.ShiftEntry
		cfgs      []*rotang.Configuration
		ctx       *router.Context
		time      time.Time
		want      string
	}{{
		name: "Success",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		time: midnight,
		cfgs: []*rotang.Configuration{
			{
				Config: rotang.Config{
					Name: "Test rota",
				},
			},
		},
		rotaMap: map[string][2]string{
			"testrota": {"Test rota", ""},
		},
		calShifts: []rotang.ShiftEntry{
			{
				StartTime: midnight,
				EndTime:   midnight.Add(2 * fullDay),
				OnCall: []rotang.ShiftMember{
					{
						Email: "test1@test.com",
					}, {
						Email: "test2@test.com",
					},
				},
			}, {
				StartTime: midnight.Add(2 * fullDay),
				EndTime:   midnight.Add(4 * fullDay),
				OnCall: []rotang.ShiftMember{
					{
						Email: "test3@test.com",
					}, {
						Email: "test4@test.com",
					},
				},
			},
		},
		want: `{"rotations":["testrota","troopers"],` +
			`"calendar":` +
			`[{"date":"2006-04-02","participants":[["test1","test2"],["test1","test2"]]},` +
			`{"date":"2006-04-03","participants":[["test1","test2"],["test1","test2"]]},` +
			`{"date":"2006-04-04","participants":[["test3","test4"],["test3","test4"]]},` +
			`{"date":"2006-04-05","participants":[["test3","test4"],["test3","test4"]]}]}` + "\n",
	}, {
		name: "No rota config",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		time: midnight,
		rotaMap: map[string][2]string{
			"testrota": {"Test rota", ""},
		},
		want: `{"rotations":["troopers"],"calendar":null}` + "\n",
	}, {
		name:    "Cal fail",
		calFail: true,
		fail:    true,
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		time: midnight,
		rotaMap: map[string][2]string{
			"testrota": {"Test rota", ""},
		},
		want: `{"rotations":["troopers"],"calendar":null}` + "\n",
	},
	}

	h := testSetup(t)

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			for _, cfg := range tst.cfgs {
				if err := h.configStore(ctx).CreateRotaConfig(ctx, cfg); err != nil {
					t.Fatalf("%s: CreateRotaConfig(ctx, _) failed: %v", tst.name, err)
				}
				defer h.configStore(ctx).DeleteRotaConfig(ctx, cfg.Config.Name)
			}

			rotaToName = tst.rotaMap
			h.legacyCalendar.(*fakeCal).Set(tst.calShifts, tst.calFail, false, 0)
			tst.ctx.Context = clock.Set(tst.ctx.Context, testclock.New(tst.time))

			res, err := h.legacyAllRotations(tst.ctx, "")
			if got, want := (err != nil), tst.fail; got != want {
				t.Fatalf("%s: legacyAllRotations(ctx, %q) = %t want: %t, err: %v", tst.name, "", got, want, err)
			}
			if err != nil {
				return
			}
			if diff := pretty.Compare(tst.want, res); diff != "" {
				t.Fatalf("%s: legacyAllRotations(ctx, %q) differ -want +got, \n%s", tst.name, "", diff)
			}

		})
	}
}

func TestBuildLegacyRotation(t *testing.T) {
	tests := []struct {
		name    string
		start   time.Time
		rota    string
		shifts  []rotang.ShiftEntry
		dateMap map[string]map[string][]string
		want    map[string]map[string][]string
	}{{
		name:  "Simple",
		rota:  "test",
		start: midnight,
		shifts: []rotang.ShiftEntry{
			{
				StartTime: midnight,
				EndTime:   midnight.Add(5 * fullDay),
				OnCall: []rotang.ShiftMember{
					{
						Email: "test1@test.com",
					}, {
						Email: "test2@test.com",
					},
				},
			}, {
				StartTime: midnight.Add(5 * fullDay),
				EndTime:   midnight.Add(10 * fullDay),
				OnCall: []rotang.ShiftMember{
					{
						Email: "test3@test.com",
					}, {
						Email: "test4@test.com",
					},
				},
			},
		},
		dateMap: make(map[string]map[string][]string),
		want: map[string]map[string][]string{
			"2006-04-02": {
				"test": {"test1", "test2"},
			},
			"2006-04-03": {
				"test": {"test1", "test2"},
			},
			"2006-04-04": {
				"test": {"test1", "test2"},
			},
			"2006-04-05": {
				"test": {"test1", "test2"},
			},
			"2006-04-06": {
				"test": {"test1", "test2"},
			},
			"2006-04-07": {
				"test": {"test3", "test4"},
			},
			"2006-04-08": {
				"test": {"test3", "test4"},
			},
			"2006-04-09": {
				"test": {"test3", "test4"},
			},
			"2006-04-10": {
				"test": {"test3", "test4"},
			},
			"2006-04-11": {
				"test": {"test3", "test4"},
			},
		},
	},
	}

	for _, tst := range tests {
		buildLegacyRotation(tst.dateMap, tst.rota, tst.shifts)

		if diff := pretty.Compare(tst.want, tst.dateMap); diff != "" {
			t.Errorf("%s: buildLegacyRotation(_, %v, _) differ -want +got , %s", tst.name, tst.start, diff)
		}
	}
}
