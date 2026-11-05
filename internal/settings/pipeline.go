package settings

import (
	"fmt"

	"github.com/google/uuid"
)

// StageType is the kind of a pipeline stage: "open", "won" or "lost".
type StageType string

// Color is a token of the closed palette of 002/DD-7; the UI decides the real color.
type Color string

const (
	stageOpen StageType = "open"
	stageWon  StageType = "won"
	stageLost StageType = "lost"
)

var (
	stageTypes = []StageType{stageOpen, stageWon, stageLost}
	colors     = []Color{"gray", "blue", "teal", "green", "yellow", "orange", "red", "purple"}
)

// Stage is an active pipeline stage; Position goes from 1 to n among the active ones.
type Stage struct {
	ID       uuid.UUID
	Name     string
	Type     StageType
	Color    Color
	Position int // 1..n among the active stages
}

// NewStage is the input of CreateStage; a nil Position appends at the end.
type NewStage struct {
	Name     string
	Type     StageType
	Color    Color
	Position *int // nil = at the end; 1..n+1
}

// StageChanges is the PATCH of a stage: nil means absent; null is not accepted in any field.
type StageChanges struct {
	Name  *string
	Type  *StageType
	Color *Color
}

func validateStageType(t StageType) string { return validateEnum(t, stageTypes) }
func validateColor(c Color) string         { return validateEnum(c, colors) }

func normalizeStageName(name string) (string, string) {
	return normalizeRequiredText(name, maxStageNameLength)
}

// validatePipeline checks the resulting list of every stage operation, in the order received: the
// first stage is open and at least one is won; lost is optional.
// T-002-B302 (002/DD-5, 002/INV-2, P-6): a violation is ErrPipelineRule, never a field error.
func validatePipeline(stages []Stage) error {
	if len(stages) == 0 {
		return fmt.Errorf("%w: no stages", ErrPipelineRule)
	}
	if stages[0].Type != stageOpen {
		return fmt.Errorf("%w: the first stage must be open", ErrPipelineRule)
	}
	for _, s := range stages {
		if s.Type == stageWon {
			return nil
		}
	}
	return fmt.Errorf("%w: at least one stage must be won", ErrPipelineRule)
}

// insertStage returns a new list with st at position (1..n+1) and positions 1..n+1. The input is not
// modified. An out-of-range position is FieldErrors{"position": "invalid_value"}.
// T-002-B302 (002/DD-5, DD-18): the caller validates the result with validatePipeline.
func insertStage(stages []Stage, st Stage, position int) ([]Stage, error) {
	if position < 1 || position > len(stages)+1 {
		return nil, FieldErrors{"position": codeInvalidValue}
	}
	out := make([]Stage, 0, len(stages)+1)
	out = append(out, stages[:position-1]...)
	out = append(out, st)
	out = append(out, stages[position-1:]...)
	return renumber(out), nil
}

// moveStage returns a new list with the stage at position from moved to position to, with dense
// positions, and whether anything changed (false when from == to: a no-op). An out-of-range to is
// FieldErrors{"position": "invalid_value"}; an out-of-range from is ErrNotFound.
// T-002-B302 (002/DD-5, DD-18): the caller validates the result with validatePipeline.
func moveStage(stages []Stage, from, to int) ([]Stage, bool, error) {
	n := len(stages)
	if from < 1 || from > n {
		return nil, false, ErrNotFound
	}
	if to < 1 || to > n {
		return nil, false, FieldErrors{"position": codeInvalidValue}
	}
	if from == to {
		return renumber(append([]Stage(nil), stages...)), false, nil
	}
	moving := stages[from-1]
	rest := make([]Stage, 0, n-1)
	rest = append(rest, stages[:from-1]...)
	rest = append(rest, stages[from:]...)
	out := make([]Stage, 0, n)
	out = append(out, rest[:to-1]...)
	out = append(out, moving)
	out = append(out, rest[to-1:]...)
	return renumber(out), true, nil
}

// removeStage returns a new list without the stage at position, with positions 1..n-1. A position
// out of range is ErrNotFound.
// T-002-B302 (002/DD-5, INV-2): the caller validates the result with validatePipeline.
func removeStage(stages []Stage, position int) ([]Stage, error) {
	if position < 1 || position > len(stages) {
		return nil, ErrNotFound
	}
	out := make([]Stage, 0, len(stages)-1)
	out = append(out, stages[:position-1]...)
	out = append(out, stages[position:]...)
	return renumber(out), nil
}

func renumber(stages []Stage) []Stage {
	for i := range stages {
		stages[i].Position = i + 1
	}
	return stages
}
