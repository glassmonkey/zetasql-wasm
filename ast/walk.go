package ast

// Walk traverses the AST rooted at n, calling fn for each node.
// If fn returns a non-nil error, traversal stops and the error is returned.
func Walk(n Node, fn func(Node) error) error {
	if n == nil {
		return nil
	}
	if err := fn(n); err != nil {
		return err
	}
	for i := range n.NumChildren() {
		if err := Walk(n.Child(i), fn); err != nil {
			return err
		}
	}
	return nil
}
