package crowdsec

import (
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

const (
	// maxYAMLStreamBytes caps how much of a stream ParseYAMLDocuments reads; it sits above every
	// per-caller input bound (1 MiB inspect entries and the CrowdSec file limit).
	maxYAMLStreamBytes = 2 << 20
	// maxYAMLNodeWeight caps the size one document reaches once its aliases are expanded. yaml.v3's own
	// alias check allows far more, and the decode cost grows with the expanded size.
	maxYAMLNodeWeight = 1_000_000
)

var (
	// ErrYAMLTooLarge is returned when a YAML stream is longer than maxYAMLStreamBytes.
	ErrYAMLTooLarge = errors.New("yaml stream too large")
	// ErrYAMLTooComplex is returned when a document expands past maxYAMLNodeWeight or its aliases loop.
	ErrYAMLTooComplex = errors.New("yaml document too complex")
)

// ParseYAMLDocuments decodes the documents of a YAML stream, each first into a yaml.Node and then
// into a generic value so duplicate mapping keys and alias abuse are rejected. The cost is bounded:
// at most maxYAMLStreamBytes are read (ErrYAMLTooLarge) and a document whose expansion exceeds
// maxYAMLNodeWeight, or whose aliases loop, is refused before it is decoded (ErrYAMLTooComplex). It
// returns the document nodes in order (none for an empty stream). With firstOnly set, later documents
// are never read; otherwise every document must parse.
func ParseYAMLDocuments(r io.Reader, firstOnly bool) ([]*yaml.Node, error) {
	capped := &cappedReader{r: r, left: maxYAMLStreamBytes}
	dec := yaml.NewDecoder(capped)
	var docs []*yaml.Node
	for {
		node := new(yaml.Node)
		err := dec.Decode(node)
		if capped.over {
			return nil, ErrYAMLTooLarge
		}
		if errors.Is(err, io.EOF) {
			return docs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode yaml document: %w", err)
		}
		if _, err = newYAMLWeigher().weight(node); err != nil {
			return nil, err
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

// cappedReader fails once more than left bytes have been read from r.
type cappedReader struct {
	r    io.Reader
	left int64
	over bool
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.over {
		return 0, ErrYAMLTooLarge
	}
	if int64(len(p)) > c.left+1 {
		p = p[:c.left+1]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	if c.left < 0 {
		c.over = true
		return n + int(c.left), ErrYAMLTooLarge
	}
	return n, err
}

// yamlWeigher computes the size of a node tree with every alias expanded, once per node.
type yamlWeigher struct {
	memo   map[*yaml.Node]uint64
	active map[*yaml.Node]bool
}

func newYAMLWeigher() *yamlWeigher {
	return &yamlWeigher{memo: map[*yaml.Node]uint64{}, active: map[*yaml.Node]bool{}}
}

// weight returns 1 plus the weight of the children of n (an alias counts the node it names). It stops
// with ErrYAMLTooComplex as soon as the total passes maxYAMLNodeWeight or a node is reached again while
// it is still being measured (an alias that loops back into its own anchor).
func (w *yamlWeigher) weight(n *yaml.Node) (uint64, error) {
	if v, ok := w.memo[n]; ok {
		return v, nil
	}
	if w.active[n] {
		return 0, ErrYAMLTooComplex
	}
	w.active[n] = true
	children := n.Content
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		children = []*yaml.Node{n.Alias}
	}
	total := uint64(1)
	for _, c := range children {
		cw, err := w.weight(c)
		if err != nil {
			return 0, err
		}
		if total += cw; total > maxYAMLNodeWeight {
			return 0, ErrYAMLTooComplex
		}
	}
	delete(w.active, n)
	w.memo[n] = total
	return total, nil
}
