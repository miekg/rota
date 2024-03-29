package handlers

import (
	"chromium.googlesource.com/infra/rotang"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"

	"context"

	"github.com/kylelemons/godebug/pretty"
	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/router"
	"go.chromium.org/luci/server/templates"
)

func TestHandleDeleteRota(t *testing.T) {
	ctx := newTestContext()
	ctxCancel, cancel := context.WithCancel(ctx)
	cancel()

	tests := []struct {
		name     string
		fail     bool
		rotaName string
		user     string
		ctx      *router.Context
		cfg      []rotang.Configuration
		want     []string
	}{{
		name: "Context canceled",
		fail: true,
		ctx: &router.Context{
			Context: ctxCancel,
			Writer:  httptest.NewRecorder(),
		},
	}, {
		name: "Delete success",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		rotaName: "Test Rota",
		user:     "test@user.com",
		cfg: []rotang.Configuration{
			{
				Config: rotang.Config{
					Name:   "Test Rota",
					Owners: []string{"test@user.com", "another@user.com"},
				},
			}, {
				Config: rotang.Config{
					Name:   "Another Rota",
					Owners: []string{"test@user.com", "another@user.com"},
				},
			},
		},
		want: []string{"Another Rota"},
	}, {
		name: "Not owner",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		rotaName: "Test Rota",
		fail:     true,
		user:     "test@user.com",
		cfg: []rotang.Configuration{
			{
				Config: rotang.Config{
					Name:   "Test Rota",
					Owners: []string{"another@user.com"},
				},
			}, {
				Config: rotang.Config{
					Name:   "Another Rota",
					Owners: []string{"test@user.com", "another@user.com"},
				},
			},
		},
	}, {
		name: "Not logged in",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		rotaName: "Test Rota",
		fail:     true,
		cfg: []rotang.Configuration{
			{
				Config: rotang.Config{
					Name:   "Test Rota",
					Owners: []string{"test@user.com", "another@user.com"},
				},
			}, {
				Config: rotang.Config{
					Name:   "Another Rota",
					Owners: []string{"test@user.com", "another@user.com"},
				},
			},
		},
	}, {
		name: "No rota name set",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		fail: true,
		cfg: []rotang.Configuration{
			{
				Config: rotang.Config{
					Name:   "Test Rota",
					Owners: []string{"test@user.com", "another@user.com"},
				},
			}, {
				Config: rotang.Config{
					Name:   "Another Rota",
					Owners: []string{"test@user.com", "another@user.com"},
				},
			},
		},
	}, {
		name: "Rota does not exist",
		ctx: &router.Context{
			Context: ctx,
			Writer:  httptest.NewRecorder(),
		},
		rotaName: "Non Existing",
		fail:     true,
		cfg: []rotang.Configuration{
			{
				Config: rotang.Config{
					Name:   "Test Rota",
					Owners: []string{"test@user.com", "another@user.com"},
				},
			}, {
				Config: rotang.Config{
					Name:   "Another Rota",
					Owners: []string{"test@user.com", "another@user.com"},
				},
			},
		},
	},
	}

	h := testSetup(t)

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			for _, c := range tst.cfg {
				if err := h.configStore(ctx).CreateRotaConfig(ctx, &c); err != nil {
					t.Fatalf("%s: CreateRotaConfig(ctx, _) failed: %v", tst.name, err)
				}
				defer h.configStore(ctx).DeleteRotaConfig(ctx, c.Config.Name)
			}
			tst.ctx.Context = templates.Use(tst.ctx.Context, &templates.Bundle{
				Loader: templates.FileSystemLoader(templatesLocation),
			}, nil)
			if tst.user != "" {
				tst.ctx.Context = auth.WithState(tst.ctx.Context, &authtest.FakeState{
					Identity: identity.Identity("user:" + tst.user),
				})
			}
			tst.ctx.Request = httptest.NewRequest("POST", "/deleterota", nil)
			tst.ctx.Request.Form = url.Values{
				"name": {tst.rotaName},
			}
			h.HandleDeleteRota(tst.ctx)

			recorder := tst.ctx.Writer.(*httptest.ResponseRecorder)
			if got, want := (recorder.Code != http.StatusFound), tst.fail; got != want {
				t.Fatalf("%s: HandleDeleteRota(ctx) = %t want: %t , res: %v", tst.name, got, want, recorder.Body)
			}
			if recorder.Code != http.StatusFound {
				return
			}

			rs, err := h.configStore(ctx).RotaConfig(ctx, "")
			if err != nil {
				t.Fatalf("%s: RotaConfig(ctx, %q) failed: %v", tst.name, "", err)
			}
			var got []string
			for _, c := range rs {
				got = append(got, c.Config.Name)
			}
			sort.Strings(got)
			sort.Strings(tst.want)
			if diff := pretty.Compare(tst.want, got); diff != "" {
				t.Errorf("%s: HandleDeleteRota(ctx) differ -want +got, %s", tst.name, diff)
			}

		})
	}
}
