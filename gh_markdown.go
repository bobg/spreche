package spreche

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/bobg/go-generics/slices"
	"github.com/bobg/htree"
	"github.com/golang-commonmark/markdown"
	"github.com/slack-go/slack"
	"golang.org/x/net/html"
)

func ghMarkdownToSlack(inp []byte) []slack.Block {
	md := markdown.New() // xxx options?
	tokens := md.Parse(inp)
	blocks := ghTokensToSlackBlocks(tokens)
	for i, block := range blocks {
		rblock, ok := block.(*slack.RichTextBlock)
		if !ok {
			continue
		}

		// Cannot use rich-text blocks on input to the Slack API (yet?).
		// See: https://github.com/slackapi/java-slack-sdk/issues/876#issuecomment-956557504
		// Downconvert them to TextBlockObjects with mrkdwn.

		buf := new(bytes.Buffer)
		richTextBlockToMrkdwn(&lineWriter{w: buf}, rblock)
		blocks[i] = slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, buf.String(), false, false), nil, nil)
	}

	return blocks
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
			if tokens[i].Tag() != tok.Tag() {
				continue
			}
			if tokens[i].Level() > tok.Level() {
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
					sectionElements := ghTokensToRichTextSectionElements(subTokens, false, false, false, false)
					elems, _ := slices.Map(sectionElements, func(_ int, secElem slack.RichTextSectionElement) (slack.RichTextElement, error) {
						return slack.NewRichTextSection(secElem), nil
					})
					result = append(result, slack.NewRichTextBlock("", &rtList{Elements: elems, Style: "ordered"}))

				case *markdown.HeadingOpen:
					text := ghTokensToTextBlockObject(subTokens)
					result = append(result, slack.NewHeaderBlock(text))

				case *markdown.ParagraphOpen:
					sectionElements := ghTokensToRichTextSectionElements(subTokens, false, false, false, false)
					result = append(result, slack.NewRichTextBlock("", slack.NewRichTextSection(sectionElements...)))

				// case *markdown.ListItemOpen:
				// case *markdown.TableOpen:
				// case *markdown.TheadOpen:
				// case *markdown.TrOpen:
				// case *markdown.ThOpen:
				// case *markdown.TbodyOpen:
				// case *markdown.TdOpen:

				default:
					text := fmt.Sprintf("[unconverted token of type %T]", tok)
					textObj := slack.NewTextBlockObject(slack.PlainTextType, text, false, true)
					result = append(result, slack.NewContextBlock("", textObj))
				}
			} else {
				// tok.Opening() && !tok.Block()

				var elems []slack.RichTextSectionElement

				if link, ok := tok.(*markdown.LinkOpen); ok {
					text := ghTokensToPlainText(subTokens)
					elems = []slack.RichTextSectionElement{slack.NewRichTextSectionLinkElement(link.Href, text, nil)}
				} else {
					var bold, italic, strike bool

					switch tok.(type) {
					case *markdown.EmphasisOpen:
						italic = true
					case *markdown.StrongOpen:
						bold = true
					case *markdown.StrikethroughOpen:
						strike = true
					}

					elems = ghTokensToRichTextSectionElements(subTokens, bold, italic, strike, false)
				}

				result = append(result, slack.NewRichTextBlock("", slack.NewRichTextSection(elems...)))
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
			node, err := html.Parse(strings.NewReader(tok.Content))
			if err != nil {
				return slack.NewTextBlockObject(slack.PlainTextType, "[unconverted HTMLBlock node]", true, true)
			}
			elems := htmlToRichTextSectionElements(node)
			return slack.NewRichTextBlock("", slack.NewRichTextSection(elems...))

		case *markdown.Hr:
			return slack.NewDividerBlock()
		}

		return nil
	}

	// !tok.Opening() && !tok.Closing() && !tok.Block()

	switch tok := tok.(type) {
	case *markdown.Image:
		var altText string // xxx
		return slack.NewImageBlock(tok.Src, altText, "", ghTokensToTextBlockObject(tok.Tokens))

	case *markdown.Inline:
		elems := ghTokensToRichTextSectionElements(tok.Children, false, false, false, false)
		return slack.NewRichTextBlock("", slack.NewRichTextSection(elems...))

	case *markdown.Text:
		elem := ghTokenToRichTextSectionElement(tok, false, false, false, false)
		return slack.NewRichTextBlock("", slack.NewRichTextSection(elem))

		// case *markdown.CodeInline:
		// case *markdown.Softbreak:
		// case *markdown.Hardbreak:
		// case *markdown.HTMLInline:

	default:
		elems := ghTokenToRichTextSectionElements(tok, false, false, false, false)
		return slack.NewRichTextBlock("", slack.NewRichTextSection(elems...))
	}
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
			if tokens[i].Tag() != tok.Tag() {
				continue
			}
			if tokens[i].Level() > tok.Level() {
				continue
			}

			var (
				subTokens = tokens[1:i]
				elems     []slack.RichTextSectionElement
			)

			if link, ok := tok.(*markdown.LinkOpen); ok {
				text := ghTokensToPlainText(subTokens)
				elems = []slack.RichTextSectionElement{slack.NewRichTextSectionLinkElement(link.Href, text, rtStyle(bold, italic, strike, code))}
			} else {
				var (
					newbold   = bold
					newitalic = italic
					newstrike = strike
				)

				switch tok.(type) {
				case *markdown.EmphasisOpen:
					newitalic = true

				case *markdown.StrongOpen:
					newbold = true

				case *markdown.StrikethroughOpen:
					newstrike = true
				}

				elems = ghTokensToRichTextSectionElements(subTokens, newbold, newitalic, newstrike, code)
			}
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
		node, err := html.Parse(strings.NewReader(tok.Content))
		if err != nil {
			return []slack.RichTextSectionElement{slack.NewRichTextSectionTextElement("[unconverted HTMLInline node]", nil)}
		}
		return htmlToRichTextSectionElements(node)

	case *markdown.Image:
		return []slack.RichTextSectionElement{slack.NewRichTextSectionTextElement(fmt.Sprintf("[image %s]", ghTokensToPlainText(tok.Tokens)), nil)}

	case *markdown.Inline:
		return ghTokensToRichTextSectionElements(tok.Children, bold, italic, strike, code)

	default:
		return []slack.RichTextSectionElement{ghTokenToRichTextSectionElement(tok, bold, italic, strike, code)}
	}
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

	return slack.NewRichTextSectionTextElement(content, rtStyle(bold, italic, strike, code))
}

func rtStyle(bold, italic, strike, code bool) *slack.RichTextSectionTextStyle {
	if !bold && !italic && !strike && !code {
		return nil
	}
	return &slack.RichTextSectionTextStyle{
		Bold:   bold,
		Italic: italic,
		Strike: strike,
		Code:   code,
	}
}

func ghTokensToTextBlockObject(tokens []markdown.Token) *slack.TextBlockObject {
	text := ghTokensToPlainText(tokens)
	return slack.NewTextBlockObject(slack.PlainTextType, text, true, true)
}

func ghTokensToPlainText(tokens []markdown.Token) string {
	buf := new(bytes.Buffer)
	ghTokensToPlainTextHelper(buf, tokens)
	return buf.String()
}

func ghTokensToPlainTextHelper(w io.Writer, tokens []markdown.Token) {
	for _, tok := range tokens {
		ghTokenToPlainText(w, tok)
	}
}

func ghTokenToPlainText(w io.Writer, tok markdown.Token) {
	switch tok := tok.(type) {
	case *markdown.CodeInline:
		fmt.Fprint(w, tok.Content)

	case *markdown.Softbreak:
		fmt.Fprint(w, " ")

	case *markdown.Hardbreak:
		fmt.Fprint(w, "\n")

	case *markdown.HTMLInline:
		node, err := html.Parse(strings.NewReader(tok.Content))
		if err != nil {
			fmt.Fprint(w, "[unconverted HTMLInline node]")
			return
		}
		htree.WriteText(w, node)

	case *markdown.Image:
		fmt.Fprintf(w, "[image %s]", ghTokensToPlainText(tok.Tokens))

	case *markdown.Inline:
		ghTokensToPlainTextHelper(w, tok.Children)

	case *markdown.Text:
		fmt.Fprint(w, tok.Content)
	}
}

func htmlToRichTextSectionElements(node *html.Node) []slack.RichTextSectionElement {
	// TODO: actual rich text
	buf := new(bytes.Buffer)
	htree.WriteText(buf, node)
	return []slack.RichTextSectionElement{slack.NewRichTextSectionTextElement(buf.String(), nil)}
}

func richTextBlockToMrkdwn(w *lineWriter, block *slack.RichTextBlock) {
	for _, elem := range block.Elements {
		w.ensurePar()
		richTextElementToMrkdwn(w, elem)
	}
}

func richTextElementToMrkdwn(w *lineWriter, elem slack.RichTextElement) {
	switch elem := elem.(type) {
	case *slack.RichTextSection:
		for _, secElem := range elem.Elements {
			richTextSectionElementToMrkdwn(w, secElem)
		}

	case *rtList:
		for i, sub := range elem.Elements {
			w.ensureLine()
			if elem.Style == "bullet" {
				fmt.Fprint(w, "- ")
			} else {
				fmt.Fprintf(w, "%d. ", i+1)
			}
			richTextElementToMrkdwn(w, sub)
		}

	case *rtQuote:
		buf := new(bytes.Buffer)
		for _, sub := range elem.Elements {
			richTextElementToMrkdwn(&lineWriter{w: buf}, sub)
		}
		w.ensureLine()
		lines := strings.Split(buf.String(), "\n")
		for _, line := range lines {
			fmt.Fprintf(w, "> %s\n", line) // TODO: blockquoting levels
		}

	case *slack.RichTextUnknown:
		switch elem.RichTextElementType() {
		case slack.RTEList:
			// xxx handled above

		case slack.RTEPreformatted:
			w.ensureLine()
			fmt.Fprintf(w, "```\n%s", elem.Raw)
			w.ensureLine()
			fmt.Fprintln(w, "```")

		case slack.RTEQuote:
			// xxx handled above

		case slack.RTESection:
			// xxx ?
			fmt.Fprint(w, elem.Raw)

		default:
			// xxx ?
			fmt.Fprint(w, elem.Raw)
		}
	}
}

func richTextSectionElementToMrkdwn(w io.Writer, elem slack.RichTextSectionElement) {
	switch elem := elem.(type) {
	case *slack.RichTextSectionTextElement:
		richTextStrToMrkdwn(w, elem.Text, elem.Style)

	case *slack.RichTextSectionChannelElement:
		richTextStrToMrkdwn(w, elem.ChannelID, elem.Style)

	case *slack.RichTextSectionUserElement:
		richTextStrToMrkdwn(w, elem.UserID, elem.Style)

	case *slack.RichTextSectionEmojiElement:
		richTextStrToMrkdwn(w, elem.Name, elem.Style)

	case *slack.RichTextSectionLinkElement:
		richTextStrToMrkdwn(w, fmt.Sprintf("<%s|%s>", elem.URL, elem.Text), elem.Style)

	case *slack.RichTextSectionTeamElement:
		richTextStrToMrkdwn(w, elem.TeamID, elem.Style)

	case *slack.RichTextSectionUserGroupElement:
		richTextStrToMrkdwn(w, elem.UsergroupID, nil)

	case *slack.RichTextSectionDateElement:
		richTextStrToMrkdwn(w, elem.Timestamp, nil)

	case *slack.RichTextSectionBroadcastElement:
		richTextStrToMrkdwn(w, elem.Range, nil)

	case *slack.RichTextSectionColorElement:
		richTextStrToMrkdwn(w, elem.Value, nil)

	case *slack.RichTextSectionUnknownElement:
		richTextStrToMrkdwn(w, elem.Raw, nil)
	}
}

func richTextStrToMrkdwn(w io.Writer, str string, style *slack.RichTextSectionTextStyle) {
	if style != nil {
		if style.Code {
			fmt.Fprint(w, "`")
			defer fmt.Fprint(w, "`")
		}
		if style.Strike {
			fmt.Fprint(w, "~")
			defer fmt.Fprint(w, "~")
		}
		if style.Italic {
			fmt.Fprint(w, "_")
			defer fmt.Fprint(w, "_")
		}
		if style.Bold {
			fmt.Fprint(w, "*")
			defer fmt.Fprint(w, "*")
		}
	}
	fmt.Fprint(w, str)
}

type lineWriter struct {
	w        io.Writer
	anyBytes bool
	newlines int
}

var _ io.Writer = &lineWriter{}

func (w *lineWriter) Write(inp []byte) (int, error) {
	res, err := w.w.Write(inp)
	if err == nil { // sic
		if res > 0 {
			w.anyBytes = true
		}
		for _, b := range inp {
			if b == '\n' {
				w.newlines++
			} else {
				w.newlines = 0
			}
		}
	}
	return res, err
}

func (w *lineWriter) ensureLine() error {
	if !w.anyBytes {
		return nil
	}
	if w.newlines > 0 {
		return nil
	}
	_, err := w.Write([]byte{'\n'})
	return err
}

func (w *lineWriter) ensurePar() error {
	if !w.anyBytes {
		return nil
	}
	switch w.newlines {
	case 0:
		_, err := w.Write([]byte{'\n', '\n'})
		return err
	case 1:
		_, err := w.Write([]byte{'\n'})
		return err
	default:
		return nil
	}
}
