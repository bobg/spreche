package spreche

import (
	"github.com/golang-commonmark/markdown"
	"github.com/slack-go/slack"
)

func ghMarkdownToSlack(inp []byte) []slack.Block {
	md := markdown.New() // xxx options?
	tokens := md.Parse(inp)
	return ghTokensToSlackBlocks(tokens)
}

// Token types and properties in github.com/golang-commonmark/markdown v0.0.0-20180910011815-a8f139058164
//
//                     Opening  Closing  Block
//                     -------  -------  -----
// BlockquoteOpen      true     false    true
// BlockquoteClose     false    true     true
// BulletListOpen      true     false    true
// BulletListClose     false    true     true
// OrderedListOpen     true     false    true
// OrderedListClose    false    true     true
// ListItemOpen        true     false    true
// ListItemClose       false    true     true
// CodeBlock           false    false    true
// CodeInline          false    false    false
// EmphasisOpen        true     false    false
// EmphasisClose       false    true     false
// StrongOpen          true     false    false
// StrongClose         false    true     false
// StrikethroughOpen   true     false    false
// StrikethroughClose  false    true     false
// Fence               false    false    true
// Softbreak           false    false    false
// Hardbreak           false    false    false
// HeadingOpen         true     false    true
// HeadingClose        false    true     true
// HTMLBlock           false    false    true
// HTMLInline          false    false    false
// Hr                  false    false    true
// Image               false    false    false
// Inline              false    false    false
// LinkOpen            true     false    false
// LinkClose           false    true     false
// ParagraphOpen       true     false    true
// ParagraphClose      false    true     true
// TableOpen           true     false    true
// TableClose          false    true     true
// TheadOpen           true     false    true
// TheadClose          false    true     true
// TrOpen              true     false    true
// TrClose             false    true     true
// ThOpen              true     false    true
// ThClose             false    true     true
// TbodyOpen           true     false    true
// TbodyClose          false    true     true
// TdOpen              true     false    true
// TdClose             false    true     true
// Text                false    false    false

func ghTokensToSlackBlocks(tokens []markdown.Token) []slack.Block {
	if len(tokens) == 0 {
		return nil
	}

	var (
		result []slack.Block
		tok    = tokens[0]
	)

	if tok.Opening() {
		for i := 1; i < len(tokens); i++ {
			if !tokens[i].Closing() {
				continue
			}
			if tokens[i].Level() != tok.Level() {
				continue
			}

			subTokens := tokens[1:i]

			if tok.Block() {
				subBlocks := ghTokensToSlackBlocks(subTokens)
				if len(subBlocks) > 0 {
					switch tok := tok.(type) {
					case *markdown.BlockquoteOpen:
					case *markdown.BulletListOpen:
					case *markdown.OrderedListOpen:
					case *markdown.ListItemOpen:
					case *markdown.HeadingOpen:
					case *markdown.ParagraphOpen:

					case *markdown.TableOpen:
					case *markdown.TheadOpen:
					case *markdown.TrOpen:
					case *markdown.ThOpen:
					case *markdown.TbodyOpen:
					case *markdown.TdOpen:
					}
				}
			} else {
				// tok.Opening() && !tok.Block()

			}

			return append(result, ghTokensToSlackBlocks(tokens[i+1:])...)
		}

		// Reached the end without finding a matching close-token.
		return nil
	}

	if tok.Closing() {
		// Error case? (Should have encountered a matching open-token first.)
		return ghTokensToSlackBlocks(tokens[1:])
	}

	rest := ghTokensToSlackBlocks(tokens[1:])
	if blk := ghTokenToSlackBlock(tok); blk != nil {
		return append([]slack.Blocks{blk}, rest...)
	}
	return rest
}

// Precondition: !tok.Opening() && !tok.Closing()
func ghTokenToSlackBlock(tok markdown.Token) slack.Block {
	if tok.Block() {
		switch tok := tok.(type) {
		case *markdown.CodeBlock:
			return &slack.RichTextUnknown{Type: slack.RTEPreformatted, Raw: tok.Content}

		case *markdown.Fence:
			return &slack.RichTextUnknown{Type: slack.RTEPreformatted, Raw: tok.Content}

		case *markdown.HTMLBlock:
			// xxx

		case *markdown.Hr:
			return slack.NewDividerBlock()
		}

		return nil
	}

	// !tok.Opening() && !tok.Closing() && !tok.Block()

	switch tok := tok.(type) {
	case *markdown.CodeInline:
	case *markdown.Softbreak:
	case *markdown.Hardbreak:
	case *markdown.HTMLInline:
		// xxx

	case *markdown.Image:
		return slack.NewImageBlock(tok.Src, altText, "", ghTokensToTextBlockObject(tok.Tokens))

	case *markdown.Inline:
		elems := ghTokensToRichTextSectionElements(tok.Children)
		return slack.NewRichTextBlock("", slack.NewRichTextSection(elems...))

	case *markdown.Text:
		elem := ghTextTokenToRichTextSectionElement(tok)
		return slack.NewRichTextBlock("", slack.NewRichTextSection(elem))
	}
}

func ghTokensToRichTextSectionElements(tokens []markdown.Token) []slack.RichTextSectionElement {
	if len(tokens) == 0 {
		return nil
	}

	tok := tokens[0]
	if tok.Block() {
		// Error case?
		return ghTokensToRichTextSectionElements(tokens[1:])
	}

	if tok.Opening() {
		for i := 1; i < len(tokens); i++ {
			if !tokens[i].Closing() {
				continue
			}
			if !tokens[i].Level() == tok.Level() {
				continue
			}

			subTokens := tokens[1:i]
		}

		// Reached the end without finding a matching close-token.
		return nil
	}

	if tok.Closing() {
		// Error case? (Should have encountered a matching open-token first.)
		return ghTokensToRichTextSectionElements(tokens[1:])
	}

	// !tok.Opening() && !tok.Closing() && !tok.Block()

	rest := ghTokensToRichTextSectionElements(tokens[1:])
	if elem := ghTokenToRichTextSectionElement(tok); elem != nil {
		return append([]slack.RichTextSectionElement{elem}, rest...)
	}
	return rest
}

// Precondition: !tok.Opening() && !tok.Closing() && !tok.Block()
func ghTokenToRichTextSectionElement(tok markdown.Token) slack.RichTextSectionElement {
	switch tok := tok.(type) {
	case *markdown.CodeInline:
	case *markdown.Softbreak:
	case *markdown.Hardbreak:
	case *markdown.HTMLInline:
	case *markdown.Image:
	case *markdown.Inline:
	case *markdown.Text:
	}
}
