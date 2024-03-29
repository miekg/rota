package handlers

import (
	"bytes"
	"chromium.googlesource.com/infra/rotang"
	"net/http"
	"text/template"
	"time"

	"go.chromium.org/gae/service/mail"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server/router"
	"google.golang.org/appengine"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// JobEmail sends emails to upcoming oncallers.
// The rotang.Email `DaysBeforeNotify` number is used to
// set when the mail is going to be sent.
// Setting DaysBeforeNotify == 0 disables sending Emails for that
// rotation.
func (h *State) JobEmail(ctx *router.Context) {
	if err := ctx.Context.Err(); err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}

	now := clock.Now(ctx.Context)
	configs, err := h.configStore(ctx.Context).RotaConfig(ctx.Context, "")
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, cfg := range configs {
		if err := h.notifyEmail(ctx, cfg, now); err != nil {
			logging.Warningf(ctx.Context, "notifyEmail(ctx, _,%v) for rota: %q failed: %v", now, cfg.Config.Name, err)
		}
	}
}

// notifyEmail figures out if a notification should be sent for the specified shift.
func (h *State) notifyEmail(ctx *router.Context, cfg *rotang.Configuration, t time.Time) error {
	if !cfg.Config.Email.Enabled || !cfg.Config.Enabled {
		msg := "config not Enabled"
		if cfg.Config.Enabled {
			msg = "e-mail notifications disabled"
		}
		logging.Infof(ctx.Context, "notifyEmail: %q not considered due to %s", cfg.Config.Name, msg)
		return nil
	}
	expTime := t.Add(time.Duration(cfg.Config.Email.DaysBeforeNotify) * fullDay).UTC()
	shifts, err := h.shiftStore(ctx.Context).ShiftsFromTo(ctx.Context, cfg.Config.Name, expTime, expTime.Add(fullDay))
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil
		}
		return err
	}
	for _, s := range shifts {
		logging.Debugf(ctx.Context, "notifyEmail: %q considering shift: %v with expTime: %v", cfg.Config.Name, s, expTime)
		// startAfterExpiry checks that the shift StartTime is Equal or After the expiry time.
		startAfterExpiry := s.StartTime.After(expTime) || s.StartTime.Equal(expTime)
		// startInsideDay handles sending only one mail per shift.
		startInsideDay := s.StartTime.Before(expTime.Add(fullDay))
		// notifyZero DatesBeforeNotify 0, then just check we're in the same day as ShiftStart.
		notifyZero := t.Equal(expTime)
		if (notifyZero || startAfterExpiry) && startInsideDay {
			logging.Debugf(ctx.Context, "notifyEmail: %q matched shift: %v with expTime: %v", cfg.Config.Name, s, expTime)
			for _, m := range s.OnCall {
				if err := h.sendMail(ctx, cfg, &s, m.Email); err != nil {
					return err
				}
				logging.Infof(ctx.Context, "notifyEmail: mail sent out to: %q, rota: %q", m.Email, cfg.Config.Name)
			}
		}
	}
	return nil
}

const (
	emailSender = "oncall_notify"
	// emailDomain is used when the h.mailAddress was not specifically set.
	// See https://cloud.google.com/appengine/docs/standard/go/mail/.
	emailDomain = ".appspotmail.com"
)

// stagingEmail is used as the To address for all emails in the staging environment.
const stagingEmail = "rotang-staging@google.com"

// sendMail executes the subject/body templates and sends the mail out.
func (h *State) sendMail(ctx *router.Context, cfg *rotang.Configuration, shift *rotang.ShiftEntry, email string) error {
	m, err := h.memberStore(ctx.Context).Member(ctx.Context, email)
	if err != nil {
		return err
	}

	subject, body, err := emailFromTemplate(cfg, &rotang.Info{
		RotaName:    cfg.Config.Name,
		ShiftConfig: cfg.Config.Shifts,
		ShiftEntry:  *shift,
		Member:      *m,
	})
	if err != nil {
		return err
	}

	to, sender := h.setSender(ctx, email)

	return h.mailSender.Send(ctx.Context, &mail.Message{
		Sender:  sender,
		To:      []string{to},
		Subject: subject,
		Body:    body,
	})
}

func (h *State) setSender(ctx *router.Context, email string) (string, string) {
	sender := h.mailAddress
	if h.IsStaging() {
		email = stagingEmail
		if sender == "" {
			sender = stagingEmail
		}
	}

	if sender == "" {
		// https://cloud.google.com/appengine/docs/standard/go/mail/
		sender = emailSender + "@" + h.projectID(appengine.NewContext(ctx.Request)) + emailDomain
	}
	return email, sender
}

func emailFromTemplate(cfg *rotang.Configuration, info *rotang.Info) (string, string, error) {
	if info == nil || cfg == nil {
		return "", "", status.Errorf(codes.InvalidArgument, "info and cfg must be set")
	}
	subjectTemplate, err := template.New("Subject").Parse(cfg.Config.Email.Subject)
	if err != nil {
		return "", "", err
	}
	bodyTemplate, err := template.New("Body").Parse(cfg.Config.Email.Body)
	if err != nil {
		return "", "", err
	}

	var subjectBuf, bodyBuf bytes.Buffer
	if err := subjectTemplate.Execute(&subjectBuf, info); err != nil {
		return "", "", err
	}
	if err := bodyTemplate.Execute(&bodyBuf, info); err != nil {
		return "", "", err
	}
	return subjectBuf.String(), bodyBuf.String(), nil
}
