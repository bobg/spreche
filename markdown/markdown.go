package markdown

import (
	"io"

	"github.com/pkg/errors"
	"github.com/russross/blackfriday/v2"
)

type Parsed struct {
	blackfridayTree *blackfriday.Node
}

func ParseGitHub(r io.Reader) (*Parsed, error) {
	p := blackfriday.New(blackfriday.WithExtensions(
		blackfriday.NoIntraEmphasis |
			blackfriday.FencedCode |
			blackfriday.Autolink |
			blackfriday.Strikethrough,
	))
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "reading GitHub markdown")
	}
	tree := p.Parse(content)
	return &Parsed{blackfridayTree: tree}, errors.Wrap(err, "parsing GitHub markdown")
}

func ParseSlack(r io.Reader) (*Parsed, error) {
	return nil, nil
}

func (p Parsed) ToGitHub(w io.Writer) error {
	return nil
}

func (p Parsed) ToSlack(w io.Writer) error {
	return nil
}
