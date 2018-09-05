package datastore

import (
	"infra/appengine/rotang"
	"sort"
	"time"

	"go.chromium.org/gae/service/datastore"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	shiftEntryKind = "DsShiftEntry"
	shiftKind      = "DsShifts"
)

// DsShifts is the parent entry used for DsShiftEntry.
type DsShifts struct {
	Key  *datastore.Key `gae:"$parent"`
	Name string         `gae:"$id"`
}

// DsShiftEntry represents a single shift entry.
type DsShiftEntry struct {
	Key       *datastore.Key `gae:"$parent"`
	Name      string
	ID        int64 `gae:"$id"`
	StartTime time.Time
	EndTime   time.Time
	OnCall    []rotang.ShiftMember
	Comment   string
}

var (
	_ rotang.ShiftStorer = &Store{}
)

// Oncall returns the shift entry for the specific time.
func (s *Store) Oncall(ctx context.Context, at time.Time, rota string) (*rotang.ShiftEntry, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// AddShifts adds shift entries.
func (s *Store) AddShifts(ctx context.Context, rota string, entries []rotang.ShiftEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dsShifts := &DsShifts{
		Key:  rootKey(ctx),
		Name: rota,
	}
	dsRota := &DsRotaConfig{
		Key: rootKey(ctx),
		ID:  rota,
	}

	return datastore.RunInTransaction(ctx, func(ctx context.Context) error {
		if err := datastore.Get(ctx, dsRota); err != nil {
			return err
		}
		memberSet := make(map[string]struct{})
		for _, m := range dsRota.Members {
			memberSet[m.Email] = struct{}{}
		}

		if err := datastore.Get(ctx, dsShifts); err != nil {
			if err != datastore.ErrNoSuchEntity {
				return err
			}
			if err := datastore.Put(ctx, dsShifts); err != nil {
				return err
			}
		}

		for _, e := range entries {
			if err := datastore.Get(ctx, &DsShiftEntry{
				Key: datastore.NewKey(ctx, shiftKind, rota, 0, datastore.KeyForObj(ctx, dsShifts)),
				ID:  e.StartTime.Unix(),
			}); err == nil {
				return status.Errorf(codes.AlreadyExists, "shift already exists at time: %v", e.StartTime)
			}
			for _, o := range e.OnCall {
				if _, ok := memberSet[o.Email]; !ok {
					return status.Errorf(codes.NotFound, "shift member: %q not a member of rota: %q", o.Email, rota)
				}
			}
			// TODO(olakar): Consider handling overlapping shifts.
			shiftEntry := &DsShiftEntry{
				Key:       datastore.NewKey(ctx, shiftKind, rota, 0, datastore.KeyForObj(ctx, dsShifts)),
				Name:      e.Name,
				ID:        e.StartTime.Unix(),
				StartTime: e.StartTime.UTC(),
				EndTime:   e.EndTime.UTC(),
				Comment:   e.Comment,
				OnCall:    e.OnCall,
			}
			if err := datastore.Put(ctx, shiftEntry); err != nil {
				return err
			}
		}
		return nil
	}, nil)
}

// DeleteShift deletes the identified shift.
func (s *Store) DeleteShift(ctx context.Context, rota string, start time.Time) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// UpdateShift updates the information in the identified shift.
func (s *Store) UpdateShift(ctx context.Context, rota string, shift *rotang.ShiftEntry) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

type byStartTime []rotang.ShiftEntry

func (s byStartTime) Less(i, j int) bool {
	return s[i].StartTime.Before(s[j].StartTime)
}

func (s byStartTime) Len() int {
	return len(s)
}

func (s byStartTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// AllShifts fetches all shifts from a rotation rota ordered by StartTime.
func (s *Store) AllShifts(ctx context.Context, rota string) ([]rotang.ShiftEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dsShifts := DsShifts{
		Key:  rootKey(ctx),
		Name: rota,
	}
	if err := datastore.Get(ctx, &dsShifts); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, status.Errorf(codes.NotFound, "shifts not found")
		}
		return nil, err
	}
	queryShifts := datastore.NewQuery(shiftEntryKind).Ancestor(datastore.KeyForObj(ctx, &DsShifts{
		Key:  rootKey(ctx),
		Name: rota,
	}))
	var dsEntries []DsShiftEntry
	if err := datastore.GetAll(ctx, queryShifts, &dsEntries); err != nil {
		return nil, err
	}

	var shifts []rotang.ShiftEntry
	for _, shift := range dsEntries {
		shifts = append(shifts, rotang.ShiftEntry{
			Name:      shift.Name,
			StartTime: shift.StartTime,
			EndTime:   shift.EndTime,
			Comment:   shift.Comment,
			OnCall:    shift.OnCall,
		})
	}

	// TODO(olakar): Look into why the Store emulator doesn't generated indexes automatically when
	// specifing .Order for the queryShifts query.
	sort.Sort(byStartTime(shifts))

	return shifts, nil
}

// Shift returns the requested shift.
func (s *Store) Shift(ctx context.Context, rota string, start time.Time) (*rotang.ShiftEntry, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// DeleteAllShifts deletes all shifts from the specified rota.
func (s *Store) DeleteAllShifts(ctx context.Context, rota string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	shiftKey := datastore.KeyForObj(ctx, &DsShifts{
		Key:  rootKey(ctx),
		Name: rota,
	})
	var shifts []DsShiftEntry
	if err := datastore.GetAll(ctx, datastore.NewQuery(shiftEntryKind).Ancestor(shiftKey), &shifts); err != nil {
		return err
	}

	return datastore.Delete(ctx, shifts, &DsShifts{
		Key:  rootKey(ctx),
		Name: rota,
	})
}