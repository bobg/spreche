package spreche

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"
)

func TestSlackBlocks(t *testing.T) {
	inputs, err := filepath.Glob("testdata/slack_blocks/*.input")
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
			var evBlocks struct {
				Event struct {
					Blocks json.RawMessage `json:"blocks"`
				} `json:"event"`
			}
			err = json.Unmarshal(data, &evBlocks)
			if err != nil {
				t.Fatal(err)
			}
			var b slack.Blocks
			err = json.Unmarshal(evBlocks.Event.Blocks, &b)
			if err != nil {
				t.Fatal(err)
			}
			buf := new(bytes.Buffer)
			blocksToGH(buf, b.BlockSet)

			output := strings.TrimSuffix(input, ".input")
			output += ".output"
			want, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(string(want), buf.String()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
