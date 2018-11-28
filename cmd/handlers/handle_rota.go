package handlers

import (
	"bytes"
	"encoding/json"
	"infra/appengine/rotang"
	"net/http"
	"time"

	"go.chromium.org/luci/server/router"
	"go.chromium.org/luci/server/templates"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const jsonHeader = "application/json"

type jsonMember struct {
	Name  string
	Email string
	TZ    string
}

type jsonRota struct {
	Cfg     rotang.Configuration
	Members []jsonMember
}

// HandleRotaCreate handler creation of new rotations.
func (h *State) HandleRotaCreate(ctx *router.Context) {
	if err := ctx.Context.Err(); err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if ctx.Request.Method == "POST" {
		var res jsonRota
		if err := json.NewDecoder(ctx.Request.Body).Decode(&res); err != nil {
			http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err := h.configStore(ctx.Context).RotaConfig(ctx.Context, res.Cfg.Config.Name)
		if status.Code(err) != codes.NotFound {
			if err == nil {
				http.Error(ctx.Writer, "rotation exists", http.StatusInternalServerError)
				return
			}
			http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := h.createRota(ctx, &res); err != nil {
			http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	var genBuf bytes.Buffer
	if err := json.NewEncoder(&genBuf).Encode(h.generators.List()); err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
	templates.MustRender(ctx.Context, ctx.Writer, "pages/rotacreate.html", templates.Args{"Generators": genBuf.String()})
}

func (h *State) validateConfig(ctx *router.Context, jr *jsonRota) error {
	members, err := convertMembers(jr.Members)
	if err != nil {
		return err
	}

	if err := h.updateMembers(ctx.Context, members); err != nil {
		return err
	}

	if !adminOrOwner(ctx, &jr.Cfg) {
		return status.Errorf(codes.PermissionDenied, "not admin or owner")
	}

	if dur, ok := checkShiftDuration(&jr.Cfg); !ok {
		return status.Errorf(codes.InvalidArgument, "shift durations does not add up to 24h,got %v", dur)
	}
	return nil
}

func (h *State) createRota(ctx *router.Context, jr *jsonRota) error {
	if err := h.validateConfig(ctx, jr); err != nil {
		return err
	}

	return h.configStore(ctx.Context).CreateRotaConfig(ctx.Context, &jr.Cfg)
}

func (h *State) modifyRota(ctx *router.Context, jr *jsonRota) error {
	if err := h.validateConfig(ctx, jr); err != nil {
		return err
	}
	return h.configStore(ctx.Context).UpdateRotaConfig(ctx.Context, &jr.Cfg)
}

// checkShiftDuration checks if the shift durations add up to 24 hours.
func checkShiftDuration(cfg *rotang.Configuration) (time.Duration, bool) {
	var totalDuration time.Duration
	for _, s := range cfg.Config.Shifts.Shifts {
		totalDuration += s.Duration
	}
	if totalDuration == fullDay {
		return totalDuration, true
	}
	return totalDuration, false
}

// convertMembers converts between the jsonMember format to rotang.Member.
// In practice this is just changing the TZ field from string to time.Location.
func convertMembers(jm []jsonMember) ([]rotang.Member, error) {
	var res []rotang.Member
	for _, m := range jm {
		tz, err := time.LoadLocation(m.TZ)
		if err != nil {
			return nil, err
		}
		res = append(res, rotang.Member{
			Name:  m.Name,
			Email: m.Email,
			TZ:    *tz,
		})
	}
	return res, nil
}

// updateMembers adds in members in the member list not already in the pool.
// Members already represented in the pool are updated.
func (h *State) updateMembers(ctx context.Context, members []rotang.Member) error {
	ms := h.memberStore(ctx)
	for _, m := range members {
		_, err := ms.Member(ctx, m.Email)
		switch {
		case err == nil:
			if err := ms.UpdateMember(ctx, &m); err != nil {
				return err
			}
		case status.Code(err) == codes.NotFound:
			if err := ms.CreateMember(ctx, &m); err != nil {
				return err
			}
		default:
			return err
		}
	}
	return nil
}

// HandleRotaModify is used to modify or copy rotation configurations.
func (h *State) HandleRotaModify(ctx *router.Context) {
	if err := ctx.Context.Err(); err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}

	switch ctx.Request.Method {
	case "GET":
		rotaName := ctx.Request.FormValue("name")
		if rotaName == "" {
			http.Error(ctx.Writer, "`name` not set", http.StatusBadRequest)
			return
		}
		rotas, err := h.configStore(ctx.Context).RotaConfig(ctx.Context, rotaName)
		if err != nil {
			http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
			return
		}

		if len(rotas) != 1 {
			http.Error(ctx.Writer, "Unexpected number of rotations returned", http.StatusInternalServerError)
			return
		}
		rota := rotas[0]

		if !adminOrOwner(ctx, rota) {
			http.Error(ctx.Writer, "not in the rotation owners", http.StatusForbidden)
			return
		}

		var members []jsonMember
		ms := h.memberStore(ctx.Context)
		for _, rm := range rota.Members {
			m, err := ms.Member(ctx.Context, rm.Email)
			if err != nil {
				http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
				return
			}
			members = append(members, jsonMember{
				Name:  m.Name,
				Email: m.Email,
				TZ:    m.TZ.String(),
			})
		}

		var resBuf bytes.Buffer
		if err := json.NewEncoder(&resBuf).Encode(&jsonRota{
			Cfg:     *rota,
			Members: members,
		}); err != nil {
			http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
			return
		}
		var genBuf bytes.Buffer
		if err := json.NewEncoder(&genBuf).Encode(h.generators.List()); err != nil {
			http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
			return
		}
		templates.MustRender(ctx.Context, ctx.Writer, "pages/rotamodify.html", templates.Args{"Config": resBuf.String(), "Generators": genBuf.String()})
		return
	case "POST":
		var res jsonRota
		if err := json.NewDecoder(ctx.Request.Body).Decode(&res); err != nil {
			http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err := h.configStore(ctx.Context).RotaConfig(ctx.Context, res.Cfg.Config.Name)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				if err := h.createRota(ctx, &res); err != nil {
					http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := h.modifyRota(ctx, &res); err != nil {
			http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(ctx.Writer, "HandleModifyRota handles only GET and POST requests", http.StatusBadRequest)
		return
	}
}
