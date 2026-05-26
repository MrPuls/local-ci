package config

import (
	"sort"
	"strings"
	"testing"
)

func TestExpandMatrixNoMatrix(t *testing.T) {
	job := JobConfig{Name: "Test", Image: "x"}
	got, err := ExpandMatrix(job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Test" {
		t.Fatalf("expected single unchanged job, got %v", got)
	}
	if got[0].MatrixGroup != "" {
		t.Errorf("expected empty MatrixGroup, got %q", got[0].MatrixGroup)
	}
}

func TestExpandMatrixSingleEntryCartesian(t *testing.T) {
	job := JobConfig{
		Name: "Test",
		Matrix: []MatrixEntry{
			{"GO": {"1.21", "1.22"}, "OS": {"linux", "alpine"}},
		},
	}
	got, err := ExpandMatrix(job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 variants, got %d", len(got))
	}

	names := variantNames(got)
	sort.Strings(names)
	want := []string{
		"Test_GO.1.21_OS.alpine",
		"Test_GO.1.21_OS.linux",
		"Test_GO.1.22_OS.alpine",
		"Test_GO.1.22_OS.linux",
	}
	if !equalSorted(names, want) {
		t.Errorf("variant names mismatch:\n got: %v\nwant: %v", names, want)
	}

	for _, v := range got {
		if v.MatrixGroup != "Test" {
			t.Errorf("variant %s: expected MatrixGroup=Test, got %q", v.Name, v.MatrixGroup)
		}
		if v.Variables["GO"] == "" || v.Variables["OS"] == "" {
			t.Errorf("variant %s: matrix vars not set: %v", v.Name, v.Variables)
		}
		if v.Matrix != nil {
			t.Errorf("variant %s: Matrix field should be cleared", v.Name)
		}
	}
}

func TestExpandMatrixMultipleEntriesAsymmetric(t *testing.T) {
	job := JobConfig{
		Name: "Deploy",
		Matrix: []MatrixEntry{
			{"PROVIDER": {"aws"}, "REGION": {"us-east", "us-west"}},
			{"PROVIDER": {"ovh"}, "REGION": {"eu-west"}},
		},
	}
	got, err := ExpandMatrix(job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(got))
	}
	names := variantNames(got)
	sort.Strings(names)
	want := []string{
		"Deploy_PROVIDER.aws_REGION.us-east",
		"Deploy_PROVIDER.aws_REGION.us-west",
		"Deploy_PROVIDER.ovh_REGION.eu-west",
	}
	if !equalSorted(names, want) {
		t.Errorf("variant names mismatch:\n got: %v\nwant: %v", names, want)
	}
}

func TestExpandMatrixVariableIsolation(t *testing.T) {
	job := JobConfig{
		Name:      "Test",
		Variables: map[string]string{"BASE": "shared"},
		Matrix: []MatrixEntry{
			{"GO": {"1.21", "1.22"}},
		},
	}
	got, _ := ExpandMatrix(job)
	if got[0].Variables["GO"] == got[1].Variables["GO"] {
		t.Errorf("variants share matrix var: %v vs %v", got[0].Variables, got[1].Variables)
	}
	got[0].Variables["GO"] = "tampered"
	if got[1].Variables["GO"] == "tampered" {
		t.Errorf("variants share variables map (mutation leaked)")
	}
	if got[0].Variables["BASE"] != "shared" || got[1].Variables["BASE"] != "shared" {
		t.Errorf("variants did not inherit parent variables")
	}
}

func TestExpandMatrixPropagatesParallel(t *testing.T) {
	job := JobConfig{
		Name:     "Test",
		Parallel: true,
		Matrix:   []MatrixEntry{{"X": {"a", "b"}}},
	}
	got, _ := ExpandMatrix(job)
	for _, v := range got {
		if !v.Parallel {
			t.Errorf("variant %s: expected Parallel=true", v.Name)
		}
	}
}

func TestExpandMatrixRejectsEmptyEntry(t *testing.T) {
	job := JobConfig{Name: "Test", Matrix: []MatrixEntry{{}}}
	if _, err := ExpandMatrix(job); err == nil {
		t.Error("expected error for empty matrix entry")
	}
}

func TestExpandMatrixRejectsEmptyValues(t *testing.T) {
	job := JobConfig{Name: "Test", Matrix: []MatrixEntry{{"X": {}}}}
	if _, err := ExpandMatrix(job); err == nil {
		t.Error("expected error for empty value list")
	}
}

func TestExpandMatrixRejectsUnsafeChars(t *testing.T) {
	cases := []MatrixEntry{
		{"BAD KEY": {"a"}},
		{"OK": {"bad value"}},
		{"OK": {"bad=value"}},
		{"OK": {"bad:value"}},
	}
	for i, entry := range cases {
		job := JobConfig{Name: "Test", Matrix: []MatrixEntry{entry}}
		if _, err := ExpandMatrix(job); err == nil {
			t.Errorf("case %d: expected error for unsafe chars in %v", i, entry)
		}
	}
}

func TestExpandMatrixRejectsDuplicates(t *testing.T) {
	job := JobConfig{
		Name: "Test",
		Matrix: []MatrixEntry{
			{"X": {"a"}},
			{"X": {"a"}},
		},
	}
	if _, err := ExpandMatrix(job); err == nil {
		t.Error("expected duplicate-variant error")
	}
}

func TestExpandMatrixScalarValueViaUnmarshal(t *testing.T) {
	// Sanity: scalar normalization happens during YAML unmarshal, not in
	// ExpandMatrix. ExpandMatrix expects already-normalized []string.
	entry := MatrixEntry{"X": {"only"}}
	job := JobConfig{Name: "T", Matrix: []MatrixEntry{entry}}
	got, _ := ExpandMatrix(job)
	if len(got) != 1 || got[0].Variables["X"] != "only" {
		t.Errorf("unexpected result: %v", got)
	}
}

func variantNames(jobs []JobConfig) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.Name
	}
	return out
}

func equalSorted(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Ensures variant names contain a stable order regardless of the iteration
// order of the underlying map. Run a single expansion many times and verify
// names are identical each time.
func TestExpandMatrixDeterministicOrdering(t *testing.T) {
	job := JobConfig{
		Name: "T",
		Matrix: []MatrixEntry{
			{"A": {"1"}, "B": {"2"}, "C": {"3"}},
		},
	}
	expected, _ := ExpandMatrix(job)
	for i := 0; i < 50; i++ {
		got, _ := ExpandMatrix(job)
		if len(got) != 1 {
			t.Fatalf("expected 1 variant, got %d", len(got))
		}
		if got[0].Name != expected[0].Name {
			t.Fatalf("non-deterministic name: %q vs %q", got[0].Name, expected[0].Name)
		}
		if !strings.Contains(got[0].Name, "A.1_B.2_C.3") {
			t.Errorf("name not in sorted-key order: %s", got[0].Name)
		}
	}
}
