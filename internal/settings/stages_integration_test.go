//go:build integration

package settings_test

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/settings"
	"github.com/google/uuid"
)

// stagesEnv is a company seeded with the "generic" template: Presupuesto open, Aprobado open, En curso open,
// Finalizado won, Perdido lost.
type stagesEnv struct {
	t    *testing.T
	clk  *testClock
	svc  *settings.Service
	co   company
	meta settings.RequestMeta
}

func newStagesEnv(t *testing.T) *stagesEnv {
	t.Helper()
	clk := newTestClock()
	return &stagesEnv{t: t, clk: clk, svc: newService(t, clk), co: newCompany(t, clk, "generic"),
		meta: settings.RequestMeta{IP: netip.MustParseAddr("192.0.2.10"), UserAgent: "stages-test/1.0"}}
}

func (e *stagesEnv) list() []settings.Stage {
	e.t.Helper()
	got, err := e.svc.ListStages(context.Background(), e.co.Admin)
	if err != nil {
		e.t.Fatalf("ListStages: %v", err)
	}
	return got
}

// summary is the pipeline as "name:type" in order, to compare it in one assertion.
func (e *stagesEnv) summary() []string {
	e.t.Helper()
	return summarize(e.list())
}

func summarize(stages []settings.Stage) []string {
	out := make([]string, len(stages))
	for i, s := range stages {
		out[i] = fmt.Sprintf("%s:%s", s.Name, s.Type)
	}
	return out
}

func (e *stagesEnv) id(name string) uuid.UUID {
	e.t.Helper()
	for _, s := range e.list() {
		if s.Name == name {
			return s.ID
		}
	}
	e.t.Fatalf("no active stage %q in %v", name, e.summary())
	return uuid.Nil
}

func (e *stagesEnv) create(in settings.NewStage) settings.Stage {
	e.t.Helper()
	got, err := e.svc.CreateStage(context.Background(), e.co.Admin, in, e.meta)
	if err != nil {
		e.t.Fatalf("CreateStage(%q): %v", in.Name, err)
	}
	return got
}

func (e *stagesEnv) assertDense() {
	e.t.Helper()
	for i, s := range e.list() {
		if s.Position != i+1 {
			e.t.Fatalf("stage %q has position %d, want %d", s.Name, s.Position, i+1)
		}
	}
}

// assertNothingChanged runs op and checks that no stage row and no audit row changed, even after the
// clock moved: a rejected or empty operation writes nothing (002/INV-13).
func (e *stagesEnv) assertNothingChanged(op func()) {
	e.t.Helper()
	before, auditBefore := storedStages(e.t, e.co.ID), readAudit(e.t, e.co.ID, "stage.")
	e.clk.Advance(time.Hour)
	op()
	if after := storedStages(e.t, e.co.ID); after != before {
		e.t.Fatalf("stages changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if auditAfter := readAudit(e.t, e.co.ID, "stage."); len(auditAfter) != len(auditBefore) {
		e.t.Fatalf("audit rows: %d before, %d after", len(auditBefore), len(auditAfter))
	}
}

func ptr[T any](v T) *T { return &v }

var genericPipeline = []string{"Presupuesto:open", "Aprobado:open", "En curso:open", "Finalizado:won", "Perdido:lost"}

func equal(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
}

func wantFieldError(t *testing.T, err error, field, code string) {
	t.Helper()
	var fields settings.FieldErrors
	if !errors.As(err, &fields) || fields[field] != code || len(fields) != 1 {
		t.Fatalf("error = %v, want FieldErrors{%s: %s}", err, field, code)
	}
}

// T-002-B401 (Historia 1.5, 002/DD-11): an admin and an operator read the same five stages.
func TestListStagesForAnyRole(t *testing.T) {
	e := newStagesEnv(t)
	other := newCompany(t, e.clk, "aluminum_carpentry")
	for name, p := range map[string]authz.Principal{"admin": e.co.Admin, "operator": e.co.Operator} {
		t.Run(name, func(t *testing.T) {
			got, err := e.svc.ListStages(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			equal(t, "pipeline", summarize(got), genericPipeline)
			for i, s := range got {
				if s.Position != i+1 || s.ID == uuid.Nil || s.Color == "" {
					t.Fatalf("stage %+v at index %d", s, i)
				}
			}
		})
	}
	// The other company has its own pipeline; nothing of it shows up here.
	others, err := e.svc.ListStages(context.Background(), other.Admin)
	if err != nil || len(others) != 6 {
		t.Fatalf("other company: %d stages, %v", len(others), err)
	}
}

// T-002-B401 (Historia 1.1, P-6, 002/INV-6, INV-13): creating a stage.
func TestCreateStage(t *testing.T) {
	ctx := context.Background()

	t.Run("at the end by default", func(t *testing.T) {
		e := newStagesEnv(t)
		got := e.create(settings.NewStage{Name: "  Instalación ", Type: "open", Color: "orange"})
		if got.Position != 6 || got.Name != "Instalación" || got.Type != "open" || got.Color != "orange" || got.ID == uuid.Nil {
			t.Fatalf("created = %+v", got)
		}
		equal(t, "pipeline", e.summary(), append(append([]string{}, genericPipeline...), "Instalación:open"))
		e.assertDense()
	})
	t.Run("at a position pushes the following ones", func(t *testing.T) {
		e := newStagesEnv(t)
		got := e.create(settings.NewStage{Name: "Instalación", Type: "open", Color: "orange", Position: ptr(3)})
		if got.Position != 3 {
			t.Fatalf("created at %d", got.Position)
		}
		equal(t, "pipeline", e.summary(), []string{"Presupuesto:open", "Aprobado:open", "Instalación:open", "En curso:open", "Finalizado:won", "Perdido:lost"})
		e.assertDense()
	})
	t.Run("before the closing stages", func(t *testing.T) {
		e := newStagesEnv(t)
		e.create(settings.NewStage{Name: "Instalación", Type: "open", Color: "orange", Position: ptr(4)})
		equal(t, "pipeline", e.summary(), []string{"Presupuesto:open", "Aprobado:open", "En curso:open", "Instalación:open", "Finalizado:won", "Perdido:lost"})
	})
	t.Run("position out of range", func(t *testing.T) {
		for _, position := range []int{0, 7, -1} {
			e := newStagesEnv(t)
			e.assertNothingChanged(func() {
				_, err := e.svc.CreateStage(ctx, e.co.Admin, settings.NewStage{Name: "Instalación", Type: "open", Color: "blue", Position: ptr(position)}, e.meta)
				wantFieldError(t, err, "position", "invalid_value")
			})
		}
	})
	t.Run("invalid fields", func(t *testing.T) {
		e := newStagesEnv(t)
		e.assertNothingChanged(func() {
			_, err := e.svc.CreateStage(ctx, e.co.Admin, settings.NewStage{Name: "  ", Type: "closed", Color: "pink"}, e.meta)
			var fields settings.FieldErrors
			if !errors.As(err, &fields) {
				t.Fatalf("error = %v", err)
			}
			equal(t, "fields", map[string]string(fields), map[string]string{"name": "required", "type": "invalid_value", "color": "invalid_value"})
		})
	})
	t.Run("name taken ignoring case", func(t *testing.T) {
		e := newStagesEnv(t)
		e.assertNothingChanged(func() {
			_, err := e.svc.CreateStage(ctx, e.co.Admin, settings.NewStage{Name: "presupuesto", Type: "open", Color: "blue"}, e.meta)
			if !errors.Is(err, settings.ErrNameTaken) {
				t.Fatalf("error = %v, want ErrNameTaken", err)
			}
		})
	})
	t.Run("name of a deleted stage is free", func(t *testing.T) {
		e := newStagesEnv(t)
		if err := e.svc.DeleteStage(ctx, e.co.Admin, e.id("Aprobado"), e.meta); err != nil {
			t.Fatal(err)
		}
		e.create(settings.NewStage{Name: "Aprobado", Type: "open", Color: "blue", Position: ptr(2)})
		e.assertDense()
	})
	t.Run("a won or lost stage cannot be first", func(t *testing.T) {
		for _, typ := range []settings.StageType{"won", "lost"} {
			e := newStagesEnv(t)
			e.assertNothingChanged(func() {
				_, err := e.svc.CreateStage(ctx, e.co.Admin, settings.NewStage{Name: "Cierre", Type: typ, Color: "green", Position: ptr(1)}, e.meta)
				if !errors.Is(err, settings.ErrPipelineRule) {
					t.Fatalf("error = %v, want ErrPipelineRule", err)
				}
			})
		}
	})
}

// T-002-B401 (Historia 1.2, 002/INV-2, INV-6, INV-13): changing the name, type or color of a stage.
func TestUpdateStage(t *testing.T) {
	ctx := context.Background()

	t.Run("name and color", func(t *testing.T) {
		e := newStagesEnv(t)
		e.clk.Advance(time.Hour)
		got, err := e.svc.UpdateStage(ctx, e.co.Admin, e.id("En curso"), settings.StageChanges{Name: ptr("En obra"), Color: ptr(settings.Color("red"))}, e.meta)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "En obra" || got.Color != "red" || got.Type != "open" || got.Position != 3 {
			t.Fatalf("updated = %+v", got)
		}
		equal(t, "pipeline", e.summary(), []string{"Presupuesto:open", "Aprobado:open", "En obra:open", "Finalizado:won", "Perdido:lost"})
	})
	t.Run("same values write nothing", func(t *testing.T) {
		e := newStagesEnv(t)
		current := e.list()[2]
		e.assertNothingChanged(func() {
			got, err := e.svc.UpdateStage(ctx, e.co.Admin, current.ID,
				settings.StageChanges{Name: ptr("  " + current.Name + " "), Type: ptr(current.Type), Color: ptr(current.Color)}, e.meta)
			if err != nil || got != current {
				t.Fatalf("got %+v, %v; want %+v", got, err, current)
			}
		})
	})
	t.Run("a different case is a change", func(t *testing.T) {
		e := newStagesEnv(t)
		got, err := e.svc.UpdateStage(ctx, e.co.Admin, e.id("En curso"), settings.StageChanges{Name: ptr("EN CURSO")}, e.meta)
		if err != nil || got.Name != "EN CURSO" {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
	t.Run("the only won stage cannot become open", func(t *testing.T) {
		e := newStagesEnv(t)
		e.assertNothingChanged(func() {
			_, err := e.svc.UpdateStage(ctx, e.co.Admin, e.id("Finalizado"), settings.StageChanges{Type: ptr(settings.StageType("open"))}, e.meta)
			if !errors.Is(err, settings.ErrPipelineRule) {
				t.Fatalf("error = %v, want ErrPipelineRule", err)
			}
		})
	})
	t.Run("the first stage cannot become won", func(t *testing.T) {
		e := newStagesEnv(t)
		e.assertNothingChanged(func() {
			_, err := e.svc.UpdateStage(ctx, e.co.Admin, e.id("Presupuesto"), settings.StageChanges{Type: ptr(settings.StageType("won"))}, e.meta)
			if !errors.Is(err, settings.ErrPipelineRule) {
				t.Fatalf("error = %v, want ErrPipelineRule", err)
			}
		})
	})
	t.Run("the won type can move to another stage", func(t *testing.T) {
		e := newStagesEnv(t)
		if _, err := e.svc.UpdateStage(ctx, e.co.Admin, e.id("En curso"), settings.StageChanges{Type: ptr(settings.StageType("won"))}, e.meta); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.UpdateStage(ctx, e.co.Admin, e.id("Finalizado"), settings.StageChanges{Type: ptr(settings.StageType("open"))}, e.meta); err != nil {
			t.Fatal(err)
		}
		equal(t, "pipeline", e.summary(), []string{"Presupuesto:open", "Aprobado:open", "En curso:won", "Finalizado:open", "Perdido:lost"})
	})
	t.Run("name of another active stage", func(t *testing.T) {
		e := newStagesEnv(t)
		e.assertNothingChanged(func() {
			_, err := e.svc.UpdateStage(ctx, e.co.Admin, e.id("En curso"), settings.StageChanges{Name: ptr("aprobado")}, e.meta)
			if !errors.Is(err, settings.ErrNameTaken) {
				t.Fatalf("error = %v, want ErrNameTaken", err)
			}
		})
	})
	t.Run("invalid fields", func(t *testing.T) {
		e := newStagesEnv(t)
		e.assertNothingChanged(func() {
			_, err := e.svc.UpdateStage(ctx, e.co.Admin, e.id("En curso"), settings.StageChanges{Name: ptr(""), Color: ptr(settings.Color("pink"))}, e.meta)
			var fields settings.FieldErrors
			if !errors.As(err, &fields) {
				t.Fatalf("error = %v", err)
			}
			equal(t, "fields", map[string]string(fields), map[string]string{"name": "required", "color": "invalid_value"})
		})
	})
	t.Run("no changes at all is malformed", func(t *testing.T) {
		e := newStagesEnv(t)
		_, err := e.svc.UpdateStage(ctx, e.co.Admin, e.id("En curso"), settings.StageChanges{}, e.meta)
		if !errors.Is(err, settings.ErrMalformedChanges) {
			t.Fatalf("error = %v, want ErrMalformedChanges", err)
		}
	})
	t.Run("deleted, other company's and unknown stages are not found", func(t *testing.T) {
		e := newStagesEnv(t)
		other := newStagesEnv(t)
		deleted := e.id("Aprobado")
		if err := e.svc.DeleteStage(ctx, e.co.Admin, deleted, e.meta); err != nil {
			t.Fatal(err)
		}
		e.assertNothingChanged(func() {
			for name, id := range map[string]uuid.UUID{"deleted": deleted, "other company": other.id("En curso"), "unknown": uuid.New()} {
				_, err := e.svc.UpdateStage(ctx, e.co.Admin, id, settings.StageChanges{Name: ptr("Otro")}, e.meta)
				if !errors.Is(err, settings.ErrNotFound) {
					t.Fatalf("%s: error = %v, want ErrNotFound", name, err)
				}
			}
		})
		equal(t, "other company pipeline", other.summary(), genericPipeline)
	})
}

// T-002-B401 (Historia 1.3, P-2, DD-6, 002/INV-2): deleting is logical and keeps the pipeline valid.
func TestDeleteStage(t *testing.T) {
	ctx := context.Background()

	t.Run("a middle stage", func(t *testing.T) {
		e := newStagesEnv(t)
		id := e.id("Aprobado")
		if err := e.svc.DeleteStage(ctx, e.co.Admin, id, e.meta); err != nil {
			t.Fatal(err)
		}
		equal(t, "pipeline", e.summary(), []string{"Presupuesto:open", "En curso:open", "Finalizado:won", "Perdido:lost"})
		e.assertDense()
		var deletedRows int
		for _, line := range strings.Split(storedStages(t, e.co.ID), "\n") {
			if strings.HasPrefix(line, id.String()) && !strings.Contains(line, "|<nil>|") {
				deletedRows++
			}
		}
		if deletedRows != 1 {
			t.Fatalf("the row of the deleted stage must stay with deleted_at set:\n%s", storedStages(t, e.co.ID))
		}
	})
	t.Run("the only won stage", func(t *testing.T) {
		e := newStagesEnv(t)
		e.assertNothingChanged(func() {
			if err := e.svc.DeleteStage(ctx, e.co.Admin, e.id("Finalizado"), e.meta); !errors.Is(err, settings.ErrPipelineRule) {
				t.Fatalf("error = %v, want ErrPipelineRule", err)
			}
		})
	})
	t.Run("the first stage when the second is open", func(t *testing.T) {
		e := newStagesEnv(t)
		if err := e.svc.DeleteStage(ctx, e.co.Admin, e.id("Presupuesto"), e.meta); err != nil {
			t.Fatal(err)
		}
		equal(t, "pipeline", e.summary(), genericPipeline[1:])
		e.assertDense()
	})
	t.Run("the first stage when the second is a closing one", func(t *testing.T) {
		e := newStagesEnv(t)
		for _, name := range []string{"Aprobado", "En curso"} {
			if err := e.svc.DeleteStage(ctx, e.co.Admin, e.id(name), e.meta); err != nil {
				t.Fatal(err)
			}
		}
		equal(t, "pipeline", e.summary(), []string{"Presupuesto:open", "Finalizado:won", "Perdido:lost"})
		// [open, won, lost] without its first stage would start with a won one.
		e.assertNothingChanged(func() {
			if err := e.svc.DeleteStage(ctx, e.co.Admin, e.id("Presupuesto"), e.meta); !errors.Is(err, settings.ErrPipelineRule) {
				t.Fatalf("error = %v, want ErrPipelineRule", err)
			}
		})
	})
	t.Run("twice", func(t *testing.T) {
		e := newStagesEnv(t)
		id := e.id("Aprobado")
		if err := e.svc.DeleteStage(ctx, e.co.Admin, id, e.meta); err != nil {
			t.Fatal(err)
		}
		e.assertNothingChanged(func() {
			if err := e.svc.DeleteStage(ctx, e.co.Admin, id, e.meta); !errors.Is(err, settings.ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
		})
	})
	t.Run("other company's and unknown stages are not found", func(t *testing.T) {
		e := newStagesEnv(t)
		other := newStagesEnv(t)
		e.assertNothingChanged(func() {
			for name, id := range map[string]uuid.UUID{"other company": other.id("Aprobado"), "unknown": uuid.New()} {
				if err := e.svc.DeleteStage(ctx, e.co.Admin, id, e.meta); !errors.Is(err, settings.ErrNotFound) {
					t.Fatalf("%s: error = %v, want ErrNotFound", name, err)
				}
			}
		})
		equal(t, "other company pipeline", other.summary(), genericPipeline)
	})
}

// T-002-B401 (Historia 1.4, DD-18, 002/INV-2, INV-13): moving one stage to a position.
func TestMoveStage(t *testing.T) {
	ctx := context.Background()

	t.Run("a closing stage cannot go first", func(t *testing.T) {
		e := newStagesEnv(t)
		e.assertNothingChanged(func() {
			if _, err := e.svc.MoveStage(ctx, e.co.Admin, e.id("Perdido"), 1, e.meta); !errors.Is(err, settings.ErrPipelineRule) {
				t.Fatalf("error = %v, want ErrPipelineRule", err)
			}
		})
	})
	t.Run("up to the first position returns the list", func(t *testing.T) {
		e := newStagesEnv(t)
		got, err := e.svc.MoveStage(ctx, e.co.Admin, e.id("En curso"), 1, e.meta)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"En curso:open", "Presupuesto:open", "Aprobado:open", "Finalizado:won", "Perdido:lost"}
		equal(t, "returned pipeline", summarize(got), want)
		equal(t, "stored pipeline", e.summary(), want)
		e.assertDense()
		for i, s := range got {
			if s.Position != i+1 {
				t.Fatalf("returned %q at position %d", s.Name, s.Position)
			}
		}
	})
	t.Run("down", func(t *testing.T) {
		e := newStagesEnv(t)
		if _, err := e.svc.MoveStage(ctx, e.co.Admin, e.id("Presupuesto"), 3, e.meta); err != nil {
			t.Fatal(err)
		}
		equal(t, "pipeline", e.summary(), []string{"Aprobado:open", "En curso:open", "Presupuesto:open", "Finalizado:won", "Perdido:lost"})
		e.assertDense()
	})
	t.Run("to the current position", func(t *testing.T) {
		e := newStagesEnv(t)
		e.assertNothingChanged(func() {
			got, err := e.svc.MoveStage(ctx, e.co.Admin, e.id("En curso"), 3, e.meta)
			if err != nil {
				t.Fatal(err)
			}
			equal(t, "returned pipeline", summarize(got), genericPipeline)
		})
	})
	t.Run("position out of range", func(t *testing.T) {
		for _, position := range []int{0, 6} {
			e := newStagesEnv(t)
			e.assertNothingChanged(func() {
				_, err := e.svc.MoveStage(ctx, e.co.Admin, e.id("En curso"), position, e.meta)
				wantFieldError(t, err, "position", "invalid_value")
			})
		}
	})
	t.Run("deleted, other company's and unknown stages are not found", func(t *testing.T) {
		e := newStagesEnv(t)
		other := newStagesEnv(t)
		deleted := e.id("Aprobado")
		if err := e.svc.DeleteStage(ctx, e.co.Admin, deleted, e.meta); err != nil {
			t.Fatal(err)
		}
		e.assertNothingChanged(func() {
			for name, id := range map[string]uuid.UUID{"deleted": deleted, "other company": other.id("En curso"), "unknown": uuid.New()} {
				if _, err := e.svc.MoveStage(ctx, e.co.Admin, id, 1, e.meta); !errors.Is(err, settings.ErrNotFound) {
					t.Fatalf("%s: error = %v, want ErrNotFound", name, err)
				}
			}
		})
		equal(t, "other company pipeline", other.summary(), genericPipeline)
	})
}

// T-002-B401 (002/INV-11, DD-11): every write needs settings.manage, and it is checked before anything
// else, so an operator learns nothing about a stage (not even its absence).
func TestStageWritesAsOperatorAreForbidden(t *testing.T) {
	ctx := context.Background()
	e := newStagesEnv(t)
	id := e.id("En curso")
	e.assertNothingChanged(func() {
		for name, call := range map[string]func() error{
			"create": func() error {
				_, err := e.svc.CreateStage(ctx, e.co.Operator, settings.NewStage{Name: "Instalación", Type: "open", Color: "blue"}, e.meta)
				return err
			},
			"create with invalid fields": func() error {
				_, err := e.svc.CreateStage(ctx, e.co.Operator, settings.NewStage{}, e.meta)
				return err
			},
			"update": func() error {
				_, err := e.svc.UpdateStage(ctx, e.co.Operator, id, settings.StageChanges{Name: ptr("Otro")}, e.meta)
				return err
			},
			"update of an unknown stage": func() error {
				_, err := e.svc.UpdateStage(ctx, e.co.Operator, uuid.New(), settings.StageChanges{Name: ptr("Otro")}, e.meta)
				return err
			},
			"delete": func() error { return e.svc.DeleteStage(ctx, e.co.Operator, id, e.meta) },
			"move": func() error {
				_, err := e.svc.MoveStage(ctx, e.co.Operator, id, 1, e.meta)
				return err
			},
		} {
			if err := call(); !errors.Is(err, authz.ErrForbidden) {
				t.Fatalf("%s: error = %v, want authz.ErrForbidden", name, err)
			}
		}
	})
}

// T-002-B401 (002/INV-12, DD-14, data-model section 6): each write leaves one audit row with the exact keys
// of the catalog, the actor, the target and the request's IP and user agent. Reads and no-ops leave none.
func TestStageWritesAreAudited(t *testing.T) {
	ctx := context.Background()
	e := newStagesEnv(t)

	created, err := e.svc.CreateStage(ctx, e.co.Admin, settings.NewStage{Name: "Instalación", Type: "open", Color: "blue", Position: ptr(4)}, e.meta)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UpdateStage(ctx, e.co.Admin, created.ID, settings.StageChanges{Color: ptr(settings.Color("red")), Name: ptr("Montaje")}, e.meta); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UpdateStage(ctx, e.co.Admin, created.ID, settings.StageChanges{Type: ptr(settings.StageType("open")), Name: ptr("Montaje")}, e.meta); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.MoveStage(ctx, e.co.Admin, created.ID, 2, e.meta); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.DeleteStage(ctx, e.co.Admin, created.ID, e.meta); err != nil {
		t.Fatal(err)
	}
	e.list()

	ip := netip.MustParseAddr("192.0.2.10")
	want := []auditRow{
		{Action: "stage.created", Data: map[string]any{"type": "open", "position": float64(4)}},
		{Action: "stage.updated", Data: map[string]any{"fields": []any{"name", "color"}}},
		{Action: "stage.moved", Data: map[string]any{"from": float64(4), "to": float64(2)}},
		{Action: "stage.deleted", Data: map[string]any{}},
	}
	got := readAudit(t, e.co.ID, "stage.")
	if len(got) != len(want) {
		t.Fatalf("audit rows = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Action != w.Action || !reflect.DeepEqual(g.Data, w.Data) {
			t.Fatalf("row %d = %s %v, want %s %v", i, g.Action, g.Data, w.Action, w.Data)
		}
		if g.ActorID == nil || *g.ActorID != e.co.Admin.UserID || g.TargetType != "stage" || g.TargetID == nil || *g.TargetID != created.ID ||
			g.IP == nil || *g.IP != ip || g.UserAgent != "stages-test/1.0" {
			t.Fatalf("row %d = %+v: actor, target, ip or user agent do not match", i, g)
		}
	}
}
