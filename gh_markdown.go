package spreche

import (
	"github.com/golang-commonmark/markdown"
	"github.com/slack-go/slack"
)

func ghMarkdownToSlack(inp []byte) []slack.Block {
	md := markdown.New() // xxx options?
	tokens := md.Parse(src)
	return ghTokensToSlackBlocks(tokens)
}

func ghTokensToSlackBlocks(tokens []markdown.Token) []slack.Block {
	if len(tokens) == 0 {
		return nil
	}

	var result []slack.Block

}
