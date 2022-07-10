package spreche

import (
	"encoding/json"

	"github.com/bobg/go-generics/slices"
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

type rtList struct {
	Elements []slack.RichTextElement `json:"elements,omitempty"`
	Style    string                  `json:"style,omitempty"`
}

var _ slack.RichTextElement = &rtList{}

func (*rtList) RichTextElementType() slack.RichTextElementType { return slack.RTEList }

func (l *rtList) MarshalJSON() ([]byte, error) {
	s := struct {
		Type     slack.RichTextElementType `json:"type"`
		Elements []slack.RichTextElement   `json:"elements,omitempty"`
		Style    string                    `json:"style,omitempty"`
	}{
		Type:     slack.RTEList,
		Elements: l.Elements,
		Style:    l.Style,
	}
	return json.Marshal(s)
}

type rtQuote struct {
	Elements []slack.RichTextElement `json:"elements,omitempty"`
}

var _ slack.RichTextElement = &rtQuote{}

func (*rtQuote) RichTextElementType() slack.RichTextElementType { return slack.RTEQuote }

func (q *rtQuote) MarshalJSON() ([]byte, error) {
	s := struct {
		Type     slack.RichTextElementType `json:"type"`
		Elements []slack.RichTextElement   `json:"elements,omitempty"`
	}{
		Type:     slack.RTEQuote,
		Elements: q.Elements,
	}
	return json.Marshal(s)
}

func ghTokensToSlackBlocks(tokens []markdown.Token) []slack.Block {
	blocks := ghTokensToSlackBlocksHelper(tokens)

	// Combine adjacent rich-text blocks.

	for i := 0; i < len(blocks)-1; /* n.b. no i++ */ {
		r1, ok := blocks[i].(*slack.RichTextBlock)
		if !ok {
			i++
			continue
		}
		r2, ok := blocks[i+1].(*slack.RichTextBlock)
		if !ok {
			i++
			continue
		}
		r1.Elements = append(r1.Elements, r2.Elements...)
		blocks = slices.RemoveN(blocks, i+1, 1)
	}

	return blocks
}

func ghTokensToSlackBlocksHelper(tokens []markdown.Token) []slack.Block {
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
				switch tok.(type) {
				case *markdown.BlockquoteOpen:
					sectionElements := ghTokensToRichTextSectionElements(subTokens, false, false, false, false)
					elems, _ := slices.Map(sectionElements, func(_ int, secElem slack.RichTextSectionElement) (slack.RichTextElement, error) {
						return slack.NewRichTextSection(secElem), nil
					})
					result = append(result, slack.NewRichTextBlock("", &rtQuote{Elements: elems}))

				case *markdown.BulletListOpen:
					sectionElements := ghTokensToRichTextSectionElements(subTokens, false, false, false, false)
					elems, _ := slices.Map(sectionElements, func(_ int, secElem slack.RichTextSectionElement) (slack.RichTextElement, error) {
						return slack.NewRichTextSection(secElem), nil
					})
					result = append(result, slack.NewRichTextBlock("", &rtList{Elements: elems, Style: "bullet"}))

				case *markdown.OrderedListOpen:
				case *markdown.ListItemOpen:
				case *markdown.HeadingOpen:
				case *markdown.ParagraphOpen:
					sectionElements := ghTokensToRichTextSectionElements(subTokens, false, false, false, false)
					result = append(result, slack.NewRichTextBlock("", slack.NewRichTextSection(sectionElements...)))

				case *markdown.TableOpen:
				case *markdown.TheadOpen:
				case *markdown.TrOpen:
				case *markdown.ThOpen:
				case *markdown.TbodyOpen:
				case *markdown.TdOpen:
				}
			} else {
				// tok.Opening() && !tok.Block()

			}

			return append(result, ghTokensToSlackBlocksHelper(tokens[i+1:])...)
		}

		// Reached the end without finding a matching close-token.
		return nil
	}

	if tok.Closing() {
		// Error case? (Should have encountered a matching open-token first.)
		return ghTokensToSlackBlocksHelper(tokens[1:])
	}

	rest := ghTokensToSlackBlocksHelper(tokens[1:])
	if blk := ghTokenToSlackBlock(tok); blk != nil {
		return append([]slack.Block{blk}, rest...)
	}
	return rest
}

// Precondition: !tok.Opening() && !tok.Closing()
func ghTokenToSlackBlock(tok markdown.Token) slack.Block {
	if tok.Block() {
		switch tok := tok.(type) {
		case *markdown.CodeBlock:
			return slack.NewRichTextBlock("", &slack.RichTextUnknown{Type: slack.RTEPreformatted, Raw: tok.Content})

		case *markdown.Fence:
			return slack.NewRichTextBlock("", &slack.RichTextUnknown{Type: slack.RTEPreformatted, Raw: tok.Content})

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
		var altText string // xxx
		return slack.NewImageBlock(tok.Src, altText, "", ghTokensToTextBlockObject(tok.Tokens))

	case *markdown.Inline:
		elems := ghTokensToRichTextSectionElements(tok.Children, false, false, false, false)
		return slack.NewRichTextBlock("", slack.NewRichTextSection(elems...))

	case *markdown.Text:
		elem := ghTokenToRichTextSectionElement(tok, false, false, false, false)
		return slack.NewRichTextBlock("", slack.NewRichTextSection(elem))
	}

	return nil
}

func ghTokensToRichTextSectionElements(tokens []markdown.Token, bold, italic, strike, code bool) []slack.RichTextSectionElement {
	if len(tokens) == 0 {
		return nil
	}

	tok := tokens[0]
	if tok.Block() {
		// Error case?
		return ghTokensToRichTextSectionElements(tokens[1:], bold, italic, strike, code)
	}

	if tok.Opening() {
		for i := 1; i < len(tokens); i++ {
			if !tokens[i].Closing() {
				continue
			}
			if tokens[i].Level() != tok.Level() {
				continue
			}

			var (
				newbold   = bold
				newitalic = italic
				newstrike = strike
				newcode   = code
			)

			subTokens := tokens[1:i]
			switch tok.(type) {
			case *markdown.EmphasisOpen:
				newitalic = true

			case *markdown.StrongOpen:
				newbold = true

			case *markdown.StrikethroughOpen:
				newstrike = true

			case *markdown.LinkOpen:
				// xxx
			}

			elems := ghTokensToRichTextSectionElements(subTokens, newbold, newitalic, newstrike, newcode)
			return append(elems, ghTokensToRichTextSectionElements(tokens[i+1:], bold, italic, strike, code)...)
		}

		// Reached the end without finding a matching close-token.
		return nil
	}

	if tok.Closing() {
		// Error case? (Should have encountered a matching open-token first.)
		return ghTokensToRichTextSectionElements(tokens[1:], bold, italic, strike, code)
	}

	// !tok.Opening() && !tok.Closing() && !tok.Block()

	elems := ghTokenToRichTextSectionElements(tok, bold, italic, strike, code)
	return append(elems, ghTokensToRichTextSectionElements(tokens[1:], bold, italic, strike, code)...)
}

// Precondition: !tok.Opening() && !tok.Closing() && !tok.Block()
func ghTokenToRichTextSectionElements(tok markdown.Token, bold, italic, strike, code bool) []slack.RichTextSectionElement {
	switch tok := tok.(type) {
	case *markdown.HTMLInline:
		// xxx

	case *markdown.Image:
		// xxx

	case *markdown.Inline:
		return ghTokensToRichTextSectionElements(tok.Children, bold, italic, strike, code)

	default:
		return []slack.RichTextSectionElement{ghTokenToRichTextSectionElement(tok, bold, italic, strike, code)}
	}

	return nil
}

func ghTokenToRichTextSectionElement(tok markdown.Token, bold, italic, strike, code bool) slack.RichTextSectionElement {
	var content string

	switch tok := tok.(type) {
	case *markdown.Softbreak:
		content = " "

	case *markdown.Hardbreak:
		content = "\n"

	case *markdown.CodeInline:
		content = tok.Content
		code = true

	case *markdown.Text:
		content = tok.Content
	}

	var style *slack.RichTextSectionTextStyle
	if bold || italic || strike || code {
		style = &slack.RichTextSectionTextStyle{
			Bold:   bold,
			Italic: italic,
			Strike: strike,
			Code:   code,
		}
	}

	return slack.NewRichTextSectionTextElement(content, style)
}

func ghTokensToTextBlockObject(tokens []markdown.Token) *slack.TextBlockObject {
	return slack.NewTextBlockObject("mrkdwn", "xxx", false, false)
}
