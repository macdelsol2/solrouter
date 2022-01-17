package solrouter

import "net/http"

type Tree struct {
	Root *TreeNode
}

type TreeNode struct {
	Children map[string]*TreeNode
	Params *Param
	Handler func(w http.ResponseWriter, r *http.Request, s *SolRouter)
}

type Param struct {
	names []string
	values []string
}