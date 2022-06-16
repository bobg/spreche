package markdown

import (
	"io"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

type Parsed struct {
	goldmarkTree ast.Node
}

func ParseGitHub(r io.Reader) (*Parsed, error) {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	p := md.Parser()
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "reading GitHub markdown")
	}
	tree := p.Parse(text.NewReader(content))
	return &Parsed{goldmarkTree: tree}, errors.Wrap(err, "parsing GitHub markdown")
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
