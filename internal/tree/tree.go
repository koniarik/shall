// Package tree reads YAML into a small tree that keeps the line of every node.
package tree

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// Kind is the shape of a node.
type Kind int

// The kinds of node.
const (
	Null Kind = iota
	Scalar
	Map
	Seq
)

// Node is one YAML value. Value holds a scalar as a string, int64, float64 or
// bool; Pairs hold a map in written order; Items hold a sequence.
type Node struct {
	Kind  Kind
	Line  int
	Value any
	Pairs []Pair
	Items []*Node
}

// Pair is one entry of a map.
type Pair struct {
	Key     string
	KeyLine int
	Value   *Node
}

// Error is a document that is not valid YAML, or uses YAML the format does
// not.
type Error struct {
	Line    int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Message) }

// Get returns the value under key, or nil when n is not a map or lacks it.
func (n *Node) Get(key string) *Node {
	if p := n.Pair(key); p != nil {
		return p.Value
	}
	return nil
}

// Pair returns the entry under key, or nil.
func (n *Node) Pair(key string) *Pair {
	if n == nil || n.Kind != Map {
		return nil
	}
	for i := range n.Pairs {
		if n.Pairs[i].Key == key {
			return &n.Pairs[i]
		}
	}
	return nil
}

// Str returns the node's value when it is a string scalar.
func (n *Node) Str() (string, bool) {
	if n == nil || n.Kind != Scalar {
		return "", false
	}
	s, ok := n.Value.(string)
	return s, ok
}

// Number returns the node's value when it is a numeric scalar.
func (n *Node) Number() (float64, bool) {
	if n == nil || n.Kind != Scalar {
		return 0, false
	}
	switch v := n.Value.(type) {
	case int64:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

// Text renders a node for a message.
func (n *Node) Text() string {
	switch {
	case n == nil || n.Kind == Null:
		return "null"
	case n.Kind == Scalar:
		if s, ok := n.Value.(string); ok {
			return strconv.Quote(s)
		}
		return fmt.Sprint(n.Value)
	case n.Kind == Map:
		return "a mapping"
	default:
		return "a list"
	}
}

var (
	position = regexp.MustCompile(`^\[(\d+):\d+\]\s*`)
	bom      = []byte("\xEF\xBB\xBF")
)

// Parse reads data, which holds at most one document. An empty document is a
// Null node. Every error it returns is an *Error.
func Parse(data []byte) (*Node, error) {
	file, err := parser.ParseBytes(bytes.TrimPrefix(data, bom), 0)
	if err != nil {
		return nil, parseError(err)
	}
	if len(file.Docs) > 1 {
		return nil, &Error{Line: documentLine(file.Docs[1]), Message: "a file holds one YAML document"}
	}
	if len(file.Docs) == 0 || file.Docs[0].Body == nil {
		return &Node{Kind: Null, Line: 1}, nil
	}
	return convert(file.Docs[0].Body)
}

func documentLine(d *ast.DocumentNode) int {
	if d.Start != nil && d.Start.Position != nil {
		return d.Start.Position.Line
	}
	return lineOf(d.Body)
}

func parseError(err error) *Error {
	first, _, _ := strings.Cut(err.Error(), "\n")
	m := position.FindStringSubmatch(first)
	if m == nil {
		return &Error{Line: 1, Message: first}
	}
	line, _ := strconv.Atoi(m[1])
	return &Error{Line: line, Message: strings.TrimSpace(first[len(m[0]):])}
}

func lineOf(n ast.Node) int {
	if n == nil {
		return 0
	}
	if t := n.GetToken(); t != nil && t.Position != nil {
		return t.Position.Line
	}
	return 0
}

func scalar(n ast.Node, v any) *Node { return &Node{Kind: Scalar, Line: lineOf(n), Value: v} }

func convert(n ast.Node) (*Node, error) {
	switch v := n.(type) {
	case *ast.MappingNode:
		out := &Node{Kind: Map, Line: lineOf(v)}
		if len(v.Values) > 0 {
			out.Line = lineOf(v.Values[0].Key)
		}
		for _, mv := range v.Values {
			p, err := convertPair(mv)
			if err != nil {
				return nil, err
			}
			out.Pairs = append(out.Pairs, p)
		}
		return out, nil
	case *ast.MappingValueNode:
		p, err := convertPair(v)
		if err != nil {
			return nil, err
		}
		return &Node{Kind: Map, Line: p.KeyLine, Pairs: []Pair{p}}, nil
	case *ast.SequenceNode:
		out := &Node{Kind: Seq, Line: lineOf(v)}
		for _, item := range v.Values {
			c, err := convert(item)
			if err != nil {
				return nil, err
			}
			out.Items = append(out.Items, c)
		}
		return out, nil
	case *ast.StringNode:
		return scalar(v, v.Value), nil
	case *ast.LiteralNode:
		return scalar(v, v.Value.Value), nil
	case *ast.IntegerNode:
		switch i := v.Value.(type) {
		case int64:
			return scalar(v, i), nil
		case uint64:
			if i <= math.MaxInt64 {
				return scalar(v, int64(i)), nil
			}
			return scalar(v, float64(i)), nil
		}
		return scalar(v, v.String()), nil
	case *ast.FloatNode:
		return scalar(v, v.Value), nil
	case *ast.BoolNode:
		return scalar(v, v.Value), nil
	case *ast.NullNode:
		return &Node{Kind: Null, Line: lineOf(v)}, nil
	case *ast.TagNode:
		return nil, &Error{Line: lineOf(v), Message: "tags are not part of the format"}
	case *ast.AnchorNode, *ast.AliasNode:
		return nil, &Error{Line: lineOf(v), Message: "anchors and aliases are not part of the format"}
	case *ast.CommentGroupNode:
		return &Node{Kind: Null, Line: lineOf(v)}, nil
	}
	return nil, &Error{Line: lineOf(n), Message: fmt.Sprintf("unexpected YAML node %s", n.Type())}
}

func convertPair(mv *ast.MappingValueNode) (Pair, error) {
	value, err := convert(mv.Value)
	if err != nil {
		return Pair{}, err
	}
	key := mv.Key.String()
	if s, ok := mv.Key.(*ast.StringNode); ok {
		key = s.Value
	}
	return Pair{Key: key, KeyLine: lineOf(mv.Key), Value: value}, nil
}
