/*
Package traverse implements tree traversals for the x/net/html parsed HTML trees.
*/
package traverse

import "golang.org/x/net/html"

// Depth performs a depth-first traversal over a parsed HTML document.
// If the visitor function returns false, traversal is stopped.
func Depth(root *html.Node, visitor func(*html.Node) bool) {
	stack := []*html.Node{root}

	for l := len(stack); l > 0; l = len(stack) {
		node := stack[l-1]
		stack = stack[:l-1]

		if !visitor(node) {
			break
		}

		for next := node.LastChild; next != nil; next = next.PrevSibling {
			stack = append(stack, next)
		}
	}
}
