package algo

import (
	"infra/appengine/rotang"
	"math/rand"
	"testing"
	"time"

	"github.com/kylelemons/godebug/pretty"
)

func TestGenerateRandom(t *testing.T) {

	tests := []struct {
		name      string
		fail      bool
		cfg       *rotang.Configuration
		start     time.Time
		members   string
		numShifts int
		previous  string
		want      []rotang.ShiftEntry
	}{{
		name:  "Random when not providing any previous shifts",
		start: midnight,
		cfg: &rotang.Configuration{
			Config: rotang.Config{
				Name: "Test Rota",
				Shifts: rotang.ShiftConfig{
					Length: 5,
					Skip:   2,
					Shifts: []rotang.Shift{
						{
							Name: "Test Shift",
						},
					},
					ShiftMembers: 1,
					Generator:    "Fair",
				},
			},
		},
		members:   "ABCDEFGHIJ",
		numShifts: 4,
		want: []rotang.ShiftEntry{
			{
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "I@I.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight,
				EndTime:   midnight.Add(5 * fullDay),
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "C@C.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(7 * fullDay),
				EndTime:   midnight.Add(7*fullDay + 5*fullDay),
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "F@F.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(14 * fullDay),
				EndTime:   midnight.Add(14*fullDay + 5*fullDay),
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "A@A.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(21 * fullDay),
				EndTime:   midnight.Add(21*fullDay + 5*fullDay),
				Comment:   "",
			},
		}}, {
		name: "Simple reverse",
		cfg: &rotang.Configuration{
			Config: rotang.Config{
				Name: "Test Rota",
				Shifts: rotang.ShiftConfig{
					Length: 5,
					Skip:   2,
					Shifts: []rotang.Shift{
						{
							Name:     "Test Shift",
							Duration: time.Hour * 8,
						},
					},
					ShiftMembers: 1,
					Generator:    "Fair",
				},
			},
		},
		numShifts: 10,
		members:   "ABCDEF",
		previous:  "ABCDEF",
		want: []rotang.ShiftEntry{
			{
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "D@D.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(2 * fullDay),           // Shift skips two days.
				EndTime:   midnight.Add(2*fullDay + 5*fullDay), // Length of the shift is 5 days.
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "E@E.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(9 * fullDay),
				EndTime:   midnight.Add(9*fullDay + 5*fullDay),
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "F@F.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(16 * fullDay),
				EndTime:   midnight.Add(16*fullDay + 5*fullDay),
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "B@B.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(23 * fullDay),
				EndTime:   midnight.Add(23*fullDay + 5*fullDay),
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "A@A.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(30 * fullDay),
				EndTime:   midnight.Add(30*fullDay + 5*fullDay),
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "C@C.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(37 * fullDay),
				EndTime:   midnight.Add(37*fullDay + 5*fullDay),
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "D@D.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(44 * fullDay),
				EndTime:   midnight.Add(44*fullDay + 5*fullDay),
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "E@E.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(51 * fullDay),
				EndTime:   midnight.Add(51*fullDay + 5*fullDay),
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "F@F.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(58 * fullDay),
				EndTime:   midnight.Add(58*fullDay + 5*fullDay),
				Comment:   "",
			}, {
				Name: "Test Shift",
				OnCall: []rotang.ShiftMember{
					{
						Email:     "B@B.com",
						ShiftName: "Test Shift",
					},
				},
				StartTime: midnight.Add(65 * fullDay),
				EndTime:   midnight.Add(65*fullDay + 5*fullDay),
				Comment:   "",
			},
		},
	},
	}

	rand.Seed(7357)
	as := New()
	as.Register(NewRandomGen())
	generator, err := as.Fetch("Random")
	if err != nil {
		t.Fatalf("as.Fetch(%q) failed: %v", "Random", err)
	}

	for _, tst := range tests {
		shifts, err := generator.Generate(tst.cfg, tst.start, stringToShifts(tst.previous), stringToMembers(tst.members), tst.numShifts)
		if got, want := (err != nil), tst.fail; got != want {
			t.Errorf("%s: Generate(_) = %t want: %t, err: %v", tst.name, got, want, err)
		}
		if err != nil {
			continue
		}
		if diff := pretty.Compare(tst.want, shifts); diff != "" {
			t.Errorf("%s: Genrate(_) differs -want +got: %s", tst.name, diff)
		}
	}

}