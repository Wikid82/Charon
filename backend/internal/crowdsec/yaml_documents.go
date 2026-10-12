package crowdsec

import (
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// ParseYAMLDocuments decodes the documents of a YAML stream, each first into a yaml.Node and then
// into a generic value so duplicate mapping keys and alias abuse are rejected. It returns the document
// nodes in order (none for an empty stream). With firstOnly set, later documents are never read;
// otherwise every document must parse.
func ParseYAMLDocuments(r io.Reader, firstOnly bool) ([]*yaml.Node, error) {
	dec := yaml.NewDecoder(r)
	var docs []*yaml.Node
	for {
		node := new(yaml.Node)
		err := dec.Decode(node)
		if errors.Is(err, io.EOF) {
			return docs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode yaml document: %w", err)
		}
		var doc any
		if err := node.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode yaml value: %w", err)
		}
		docs = append(docs, node)
		if firstOnly {
			return docs, nil
		}
	}
}
