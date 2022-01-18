package solrouter

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
)

type SolRouter struct {
	tree *Tree
}

func New() *SolRouter {
	t := &Tree{
		Root: &TreeNode{
			Children: map[string]*TreeNode{
				http.MethodGet: {},
				http.MethodPost: {},
				http.MethodPut: {},
				http.MethodDelete: {},
			},
		}}
	return &SolRouter{tree: t}
}

func (sol *SolRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := sol.MatchPath(r)
	if handler == nil {
		fmt.Fprint(w, "Invalid route")
		return
	}
	handler(w, r, sol)
}

func (sol *SolRouter) SetGET(endpoint string, handler func(w http.ResponseWriter, r *http.Request, s *SolRouter)) {
	err := sol.setPath(endpoint, "GET", handler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

func (sol *SolRouter) GetPOST(endpoint string, handler func(w http.ResponseWriter, r *http.Request, s *SolRouter)) {
	err := sol.setPath(endpoint, "POST", handler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

func (sol *SolRouter) GetPUT(endpoint string, handler func(w http.ResponseWriter, r *http.Request, s *SolRouter)) {
	err := sol.setPath(endpoint, "PUT", handler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

func (sol *SolRouter) GetDELETE(endpoint string, handler func(w http.ResponseWriter, r *http.Request, s *SolRouter)) {
	err := sol.setPath(endpoint, "DELETE", handler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

func (sol *SolRouter) Param(name string, r *http.Request) string {
	if sol.tree == nil || sol.tree.Root == nil || sol.tree.Root.Children == nil || sol.tree.Root.Children[r.Method] == nil {
		return ""
	}
	currentNode := sol.tree.Root.Children[r.Method]
	sections := strings.Split(r.URL.Path, "/")
	return paramHelper(sections[1:], name, currentNode)
}

func paramHelper(sections []string, name string, node *TreeNode) string {
	if node == nil {
		return ""
	} else if node.Params != nil && node.Params.names != nil && len(node.Params.names) > 0 {
		for i, p := range node.Params.names {
			if p != name {
				continue
			} else if len(node.Params.values) > i {
				return node.Params.values[i]
			}
		}
	}
	if len(sections) == 0 {
		return ""
	}
	next, _ := nextNode(node, sections[0])
	if next == nil {
		return ""
	}
	if len(sections) == 1 {
		sections = []string{}
	} else {
		sections = sections[1:]
	}
	return paramHelper(sections, name, next)
}

func (sol *SolRouter) setPath(endpoint, httpMethod string, handler func(w http.ResponseWriter, r *http.Request, s *SolRouter)) error {
	if string(endpoint[0]) != "/" {
		return fmt.Errorf("Endpoint: %s must start with //", endpoint)
	}
	if sol.tree == nil || sol.tree.Root == nil || sol.tree.Root.Children == nil || sol.tree.Root.Children[httpMethod] == nil {
		return fmt.Errorf("We've encountered an error setting endpoint: %s", endpoint)
	}
	currentNode := sol.tree.Root.Children[httpMethod]
	sections := strings.Split(endpoint, "/")
	for _, section := range sections[1:] {
		if currentNode.Children == nil {
			currentNode.Children = map[string]*TreeNode{}
		}

		// for paths with params, replaces {param} with [^/]+ to match anything not /
		// paths without params will not change
		key := constructRegex(section)

		if currentNode.Children[key] == nil {
			newNode := &TreeNode{Children: make(map[string]*TreeNode), Params: &Param{[]string{}, []string{}}}
			setParamNames(section, newNode.Params)
			currentNode.Children[key] = newNode
			currentNode = newNode
			continue
		}
		currentNode = currentNode.Children[key]
		continue
	}
	currentNode.Handler = handler
	return nil
}

func constructRegex(s string) string {
	var buffer bytes.Buffer
	prev := 0
	start := -1
	end := -1
	for j, r := range s {
		if r == 123 {
			start = j
		} else if r == 125 {
			end = j
		}

		if start >= 0 && end > 0 && start < end {
			buffer.WriteString(s[prev:start])
			buffer.WriteString("[^/]+")
			prev = end + 1
			start = -1
			end = -1
		}
	}
	if prev < len(s) {
		buffer.WriteString(s[prev:])
	}
	return buffer.String()
}

func setParamNames(s string, params *Param) {
	start := -1
	end := -1
	for j, r := range s {
		if r == 123 {
			start = j
		} else if r == 125 {
			end = j
		}

		if start >= 0 && end > 0 && start < end {
			paramName := s[start+1 : end]
			params.names = append(params.names, paramName)
			params.values = append(params.values, "")
			start = -1
			end = -1
		}
	}
}

func (sol *SolRouter) MatchPath(r *http.Request) func(w http.ResponseWriter, r *http.Request, s *SolRouter) {
	if string(r.URL.Path[0]) != "/" {
		fmt.Println("Endpoint must start with //")
		return nil
	}
	if sol.tree == nil || sol.tree.Root == nil || sol.tree.Root.Children == nil {
		return nil
	}
	currentNode := sol.tree.Root.Children[r.Method]
	sections := strings.Split(r.URL.Path, "/")
	return sol.matchPathHelper(sections[1:], currentNode)
}

func (sol *SolRouter) matchPathHelper(pathSections []string, currentNode *TreeNode) func(w http.ResponseWriter, r *http.Request, s *SolRouter) {
	if currentNode == nil || len(pathSections) == 0 {
		return nil
	}

	nextNode, key := nextNode(currentNode, pathSections[0])
	if nextNode == nil {
		return nil
	}

	sol.setParamValues(pathSections[0], key, nextNode)
	
	if len(pathSections) == 1 {
		return nextNode.Handler
	}
	return sol.matchPathHelper(pathSections[1:], nextNode)
}

func nextNode(node *TreeNode, s string) (child *TreeNode, key string) {
	if node == nil || node.Children == nil {
		return nil, ""
	}
	c, ok := node.Children[s]
	if ok {
		return c, s
	}
	for k, cur := range node.Children {
		if cur == nil || cur.Params == nil || len(cur.Params.names) == 0{
			continue
		}
		//match first path selection with node value
		r := regexp.MustCompile(k)
		match := r.FindStringSubmatch(s)
		if match == nil {
			continue
		}
		child = cur
		key = k
		break
	}
	return
}

func (router *SolRouter) setParamValues(inputPathSection, savedPathSection string, node *TreeNode) {
	numParamsParsed := 0
	nonParamsInKey := strings.Split(savedPathSection, "[^/]+")
	i := 0
	j := 0
	start := -1

	for i < len(nonParamsInKey) && j <= len(inputPathSection) {
		if len(nonParamsInKey[i]) == 0 && start >= 0 {
			parsedParam := inputPathSection[start:]
			node.Params.values[numParamsParsed] = parsedParam
			numParamsParsed++
			break
		} else if start == -1 {
			start = len(nonParamsInKey[i])
			i++
			j = start + 1
		} else if inputPathSection[j:j+len(nonParamsInKey[i])] == nonParamsInKey[i] {
			parsedParam := inputPathSection[start:j]
			node.Params.values[numParamsParsed] = parsedParam
			numParamsParsed++
			j += len(nonParamsInKey[i])
			start = j
			i++
			continue
		} else {
			j++
		}
	}
}