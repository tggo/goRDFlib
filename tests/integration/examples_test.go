package integration_test

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestExamples(t *testing.T) {
	examples := []string{
		"simple_example",
		"sparql_query_example",
		"format_examples",
		"property_paths_example",
		"resource_example",
		"transitive_example",
		"shacl_example",
		"shacl_constraints_example",
		"sparql_update_example",
	}

	for _, name := range examples {
		t.Run(name, func(t *testing.T) {
			exampleDir := filepath.Join("..", "..", "examples", name)

			cmd := exec.Command("go", "run", ".")
			cmd.Dir = exampleDir
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stdout
			if err := cmd.Run(); err != nil {
				t.Fatalf("example %s failed: %v\nOutput:\n%s", name, err, stdout.String())
			}

			goldenPath := filepath.Join(exampleDir, "output.golden")

			if *update {
				if err := os.WriteFile(goldenPath, stdout.Bytes(), 0644); err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
				t.Logf("updated %s", goldenPath)
				return
			}

			expected, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("failed to read golden file %s: %v\n"+
					"Run with -update to create it:\n"+
					"  go test -run TestExamples -update", goldenPath, err)
			}

			if !bytes.Equal(stdout.Bytes(), expected) {
				t.Errorf("output mismatch for %s\n"+
					"Run with -update to regenerate:\n"+
					"  go test -run TestExamples -update\n\n"+
					"Got:\n%s\nWant:\n%s",
					name, stdout.String(), string(expected))
			}
		})
	}
}
