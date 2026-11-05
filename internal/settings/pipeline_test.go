package settings

import (
	"errors"
	"reflect"
	"testing"
)

// stagesOf builds stages named by their type's initial and index, with positions 1..n.
func stagesOf(names ...string) []Stage {
	out := make([]Stage, len(names))
	for i, n := range names {
		out[i] = Stage{Name: n, Type: stageOpen, Position: i + 1}
	}
	return out
}

func namesOf(stages []Stage) []string {
	out := make([]string, len(stages))
	for i, s := range stages {
		out[i] = s.Name
	}
	return out
}

func assertDense(t *testing.T, stages []Stage) {
	t.Helper()
	for i, s := range stages {
		if s.Position != i+1 {
			t.Fatalf("stage %q has position %d, want %d (list %v)", s.Name, s.Position, i+1, namesOf(stages))
		}
	}
}

func typed(types ...StageType) []Stage {
	out := make([]Stage, len(types))
	for i, ty := range types {
		out[i] = Stage{Type: ty, Position: i + 1}
	}
	return out
}

// T-002-B301 (002/DD-5, INV-2, P-6): the first stage is open and at least one is won.
func TestValidatePipeline(t *testing.T) {
	o, w, l := stageOpen, stageWon, stageLost
	tests := []struct {
		name   string
		stages []StageType
		ok     bool
	}{
		{"empty", nil, false},
		{"only open", []StageType{o}, false},
		{"open and lost without won", []StageType{o, l}, false},
		{"won first", []StageType{w, o}, false},
		{"lost first", []StageType{l, o, w}, false},
		{"open won", []StageType{o, w}, true},
		{"open won lost", []StageType{o, w, l}, true},
		{"open lost won", []StageType{o, l, w}, true},
		{"two open two won", []StageType{o, o, w, w}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePipeline(typed(tc.stages...))
			if tc.ok && err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if !tc.ok && !errors.Is(err, ErrPipelineRule) {
				t.Fatalf("err = %v, want ErrPipelineRule", err)
			}
		})
	}
}

// T-002-B301 (002/DD-5): a request error is a field error and a rule violation is not.
func TestValidatePipelineIsNotAFieldError(t *testing.T) {
	var fe FieldErrors
	if errors.As(validatePipeline(nil), &fe) {
		t.Fatal("ErrPipelineRule must not be FieldErrors")
	}
}

func positionIsInvalid(t *testing.T, err error) {
	t.Helper()
	var fe FieldErrors
	if !errors.As(err, &fe) || fe["position"] != codeInvalidValue || len(fe) != 1 {
		t.Fatalf("err = %v, want position -> invalid_value", err)
	}
}

// T-002-B301 (002/DD-5, DD-18): inserting at 1..n+1 gives dense positions; the input is untouched.
func TestInsertStage(t *testing.T) {
	base := stagesOf("a", "b", "c")
	tests := []struct {
		position int
		want     []string
	}{
		{1, []string{"x", "a", "b", "c"}},
		{2, []string{"a", "x", "b", "c"}},
		{3, []string{"a", "b", "x", "c"}},
		{4, []string{"a", "b", "c", "x"}},
	}
	for _, tc := range tests {
		got, err := insertStage(base, Stage{Name: "x", Type: stageOpen}, tc.position)
		if err != nil {
			t.Fatalf("position %d: %v", tc.position, err)
		}
		if !reflect.DeepEqual(namesOf(got), tc.want) {
			t.Errorf("position %d: got %v, want %v", tc.position, namesOf(got), tc.want)
		}
		assertDense(t, got)
	}
	if !reflect.DeepEqual(base, stagesOf("a", "b", "c")) {
		t.Errorf("the input was modified: %v", base)
	}
	for _, position := range []int{0, 5, -1} {
		_, err := insertStage(base, Stage{Name: "x"}, position)
		positionIsInvalid(t, err)
	}
	got, err := insertStage(nil, Stage{Name: "x"}, 1)
	if err != nil || len(got) != 1 || got[0].Position != 1 {
		t.Errorf("insert into an empty list: %v, %v", got, err)
	}
}

// T-002-B301 (002/DD-5, DD-18): moving up, down, to the first and to the last gives dense positions.
func TestMoveStage(t *testing.T) {
	base := stagesOf("a", "b", "c", "d")
	tests := []struct {
		name     string
		from, to int
		want     []string
		changed  bool
	}{
		{"same position is a no-op", 2, 2, []string{"a", "b", "c", "d"}, false},
		{"up one", 3, 2, []string{"a", "c", "b", "d"}, true},
		{"down one", 2, 3, []string{"a", "c", "b", "d"}, true},
		{"to the first", 4, 1, []string{"d", "a", "b", "c"}, true},
		{"to the last", 1, 4, []string{"b", "c", "d", "a"}, true},
		{"down several", 1, 3, []string{"b", "c", "a", "d"}, true},
		{"up several", 4, 2, []string{"a", "d", "b", "c"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := moveStage(base, tc.from, tc.to)
			if err != nil {
				t.Fatal(err)
			}
			if changed != tc.changed || !reflect.DeepEqual(namesOf(got), tc.want) {
				t.Fatalf("got %v changed=%v, want %v changed=%v", namesOf(got), changed, tc.want, tc.changed)
			}
			assertDense(t, got)
		})
	}
	if !reflect.DeepEqual(base, stagesOf("a", "b", "c", "d")) {
		t.Errorf("the input was modified: %v", base)
	}
	for _, to := range []int{0, 5, -1} {
		_, _, err := moveStage(base, 2, to)
		positionIsInvalid(t, err)
	}
	if _, _, err := moveStage(base, 9, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("from out of range: err = %v, want ErrNotFound", err)
	}
}

// T-002-B301 (002/DD-5, INV-2): removing the first, a middle and the last stage leaves positions 1..n-1.
func TestRemoveStage(t *testing.T) {
	base := stagesOf("a", "b", "c", "d")
	tests := []struct {
		name     string
		position int
		want     []string
	}{
		{"first", 1, []string{"b", "c", "d"}},
		{"middle", 2, []string{"a", "c", "d"}},
		{"last", 4, []string{"a", "b", "c"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := removeStage(base, tc.position)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(namesOf(got), tc.want) {
				t.Fatalf("got %v, want %v", namesOf(got), tc.want)
			}
			assertDense(t, got)
		})
	}
	if !reflect.DeepEqual(base, stagesOf("a", "b", "c", "d")) {
		t.Errorf("the input was modified: %v", base)
	}
	for _, position := range []int{0, 5} {
		if _, err := removeStage(base, position); !errors.Is(err, ErrNotFound) {
			t.Errorf("position %d: err = %v, want ErrNotFound", position, err)
		}
	}
}
