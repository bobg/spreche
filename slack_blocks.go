package spreche

import (
	"bytes"
	"fmt"
	"io"

	"github.com/slack-go/slack"
)

func textOrBlocksToGH(commentURL, username, text string, blocks []slack.Block) string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "_[[comment](%s) from %s]_", commentURL, username)
	if len(blocks) == 0 {
		fmt.Fprint(buf, "\n\n"+text) // xxx escaping of text
	} else {
		blocksToGH(buf, blocks)
	}
	return buf.String()
}

func blocksToGH(w io.Writer, blocks []slack.Block) {
	for _, block := range blocks {
		fmt.Fprint(w, "\n\n")
		blockToGH(w, block)
	}
}

func blockToGH(w io.Writer, block slack.Block) {
	switch block := block.(type) {
	case slack.ActionBlock:
		fmt.Fprint(w, "[unrendered action block]")

	case *slack.ContextBlock:
		for _, elem := range block.ContextElements.Elements {
			mixedElementToGH(w, elem)
		}

	case *slack.DividerBlock:
		fmt.Fprint(w, "--")

	case *slack.FileBlock:
		fmt.Fprint(w, "[unrendered file block]")

	case *slack.HeaderBlock:
		if block.Text != nil {
			fmt.Fprint(w, "## ")
			textBlockObjectToGH(w, block.Text)
		}

	case *slack.ImageBlock:
		imageToGH(w, block.ImageURL, block.AltText, block.Title)

	case *slack.InputBlock:
		fmt.Fprint(w, "[unrendered input block]")

	case *slack.RichTextBlock:
		for _, elem := range block.Elements {
			richTextElementToGH(w, elem)
		}

	case *slack.SectionBlock:
		if len(block.Fields) > 0 {
			sectionFieldsToGH(w, block.Fields)
		} else {
			textBlockObjectToGH(w, block.Text)
		}
		// TODO: block.Accessory

	case *slack.TextBlockObject:
		textBlockObjectToGH(w, block)

	default:
		fmt.Fprintf(w, "[unknown Slack block type %T (%s)]", block, block.BlockType())
	}
}

func imageToGH(w io.Writer, imageURL, altText string, title *slack.TextBlockObject) {
	fmt.Fprintf(w, "![%s](%s)", altText, imageURL) // xxx escaping (also honor title)
}

func mixedElementToGH(w io.Writer, elem slack.MixedElement) {
	switch elem := elem.(type) {
	case *slack.ImageBlockElement:
		imageToGH(w, elem.ImageURL, elem.AltText, nil)

	case *slack.TextBlockObject:
		textBlockObjectToGH(w, elem)

	default:
		fmt.Fprintf(w, "[unknown Slack mixed-element type %T (%s)]", elem, elem.MixedElementType())
	}
}

func richTextElementToGH(w io.Writer, elem slack.RichTextElement) {
	switch elem := elem.(type) {
	case *slack.RichTextSection:
		for _, ee := range elem.Elements {
			richTextSectionElementToGH(w, ee)
		}

	default:
		fmt.Fprintf(w, "[unknown Slack rich-text element type %T (%s)]", elem, elem.RichTextElementType())
	}
}

func richTextSectionElementToGH(w io.Writer, elem slack.RichTextSectionElement) {
	switch elem := elem.(type) {
	case *slack.RichTextSectionBroadcastElement:
	case *slack.RichTextSectionChannelElement:
	case *slack.RichTextSectionColorElement:
	case *slack.RichTextSectionDateElement:
	case *slack.RichTextSectionEmojiElement:
	case *slack.RichTextSectionLinkElement:
	case *slack.RichTextSectionTeamElement:
	case *slack.RichTextSectionTextElement:
		styledContentToGH(w, elem.Style, func() {
			fmt.Fprint(w, elem.Text) // xxx escaping
		})

	case *slack.RichTextSectionUserElement:
	case *slack.RichTextSectionUserGroupElement:
	default:
		fmt.Fprintf(w, "[unknown Slack rich-text-section element type %T (%s)]", elem, elem.RichTextSectionElementType())
	}
}

func textBlockObjectToGH(w io.Writer, obj *slack.TextBlockObject) {
	fmt.Fprint(w, obj.Text) // xxx escaping, obj.Verbatim
}

func sectionFieldsToGH(w io.Writer, objs []*slack.TextBlockObject) {
}

func styledContentToGH(w io.Writer, style *slack.RichTextSectionTextStyle, f func()) {
	if style != nil {
		if style.Strike {
			fmt.Fprint(w, "~~")
		}
		if style.Bold && style.Italic {
			fmt.Fprint(w, "***")
		} else if style.Bold {
			fmt.Fprint(w, "**")
		} else if style.Italic {
			fmt.Fprint(w, "_")
		}
		if style.Code {
			fmt.Fprint(w, "`")
		}
	}
	f()
	if style != nil {
		if style.Code {
			fmt.Fprint(w, "`")
		}
		if style.Bold && style.Italic {
			fmt.Fprint(w, "***")
		} else if style.Bold {
			fmt.Fprint(w, "**")
		} else if style.Italic {
			fmt.Fprint(w, "_")
		}
		if style.Strike {
			fmt.Fprint(w, "~~")
		}
	}
}
