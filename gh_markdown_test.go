package spreche

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGHMarkdown(t *testing.T) {
	inputs, err := filepath.Glob("testdata/gh_markdown/*.input")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range inputs {
		basename := filepath.Base(input)
		basename = strings.TrimSuffix(basename, ".input")
		t.Run(basename, func(t *testing.T) {
			data, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}

			got := ghMarkdownToSlack(data)
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			var gotMap []map[string]any
			err = json.Unmarshal(gotJSON, &gotMap)
			if err != nil {
				t.Fatal(err)
			}

			t.Log(string(gotJSON))

			output := strings.TrimSuffix(input, ".input")
			output += ".output"

			f, err := os.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			dec := json.NewDecoder(f)

			var want []map[string]any
			err = dec.Decode(&want)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(gotMap, want) {
				gotstr, err := json.MarshalIndent(gotMap, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				wantstr, err := json.MarshalIndent(want, "", "  ")
				if err != nil {
					t.Fatal(err)
				}

				t.Errorf("got:\n%s\nwant:\n%s", string(gotstr), string(wantstr))
			}
		})
	}
}
