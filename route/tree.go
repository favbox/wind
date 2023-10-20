package route

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/route/param"
)

type (
	kind uint8
	node struct {
		kind       kind     // 路由类型
		label      byte     // 路由标识符
		prefix     string   // 前缀
		parent     *node    // 父节点
		children   children // 子节点切片
		ppath      string   // 原始路径
		pnames     []string // 参数名称切片
		handlers   app.HandlersChain
		paramChild *node
		anyChild   *node
		// 表示该节点没有子路由
		isLeaf bool
	}
	children []*node
	// 保存 (*Node).getValue() 的返回值
	nodeValue struct {
		handlers app.HandlersChain
		tsr      bool
		fullPath string
	}

	// 路由树，一个方法一棵树。
	// 以路径前缀为节点进行枝叶蔓延。
	router struct {
		method        string
		root          *node
		hasTsrHandler map[string]bool
	}

	// MethodTrees 是路由器的方法树切片
	MethodTrees []*router
)

const (
	skind kind = iota // static 静态路由
	pkind             // param 命名参数路由
	akind             // all 通配参数路由

	paramLabel = byte(':') // 命名参数路由标识符
	anyLabel   = byte('*') // 通配参数路由标识符

	slash     = "/"
	nilString = ""
)

func (trees MethodTrees) get(method string) *router {
	for _, tree := range trees {
		if tree.method == method {
			return tree
		}
	}
	return nil
}

// 不区分大小写地查找指定 path 是否存在对应的路由。
func (n *node) findCaseInsensitivePath(path string, fixTrailingSlash bool) (ciPath []byte, found bool) {
	ciPath = make([]byte, 0, len(path)+1) // 预先分配足够的内存

	// 匹配命名参数路由
	if n.label == paramLabel {
		end := 0
		for end < len(path) && path[end] != '/' {
			end++
		}
		ciPath = append(ciPath, path[:end]...)
		if end < len(path) {
			if len(n.children) > 0 {
				path = path[end:]

				goto loop
			}

			if fixTrailingSlash && len(path) == end+1 {
				return ciPath, true
			}
			return
		}

		if n.handlers != nil {
			return ciPath, true
		}
		if fixTrailingSlash && len(n.children) == 1 {
			// 未找到处理器。检查该路径是否有或没有尾随斜杠
			n = n.children[0]
			if n.prefix == "/" && n.handlers != nil {
				return append(ciPath, '/'), true
			}
		}
		return
	}

	// 匹配通配参数路由
	if n.label == anyLabel {
		return append(ciPath, path...), true
	}

	// 匹配静态路由
	if len(path) >= len(n.prefix) && strings.EqualFold(path[:len(n.prefix)], n.prefix) {
		path = path[len(n.prefix):]
		ciPath = append(ciPath, n.prefix...)

		if len(path) == 0 {
			if n.handlers != nil {
				return ciPath, true
			}
			// 没找到处理器。
			// 尝试通过增加尾随斜杠来修正路径。
			if fixTrailingSlash {
				for i := 0; i < len(n.children); i++ {
					if n.children[i].label == '/' {
						n = n.children[i]
						if (len(n.prefix) == 1 && n.handlers != nil) ||
							(n.prefix == "*" && n.children[0].handlers != nil) {
							return append(ciPath, '/'), true
						}
						return
					}
				}
			}
			return
		}
	} else if fixTrailingSlash {
		// 啥也没找到。
		// 尝试通过移除尾随斜杠来修正路径。
		if path == "/" {
			return ciPath, true
		}
		if len(path)+1 == len(n.prefix) && n.prefix[len(path)] == '/' &&
			strings.EqualFold(path, n.prefix[:len(path)]) &&
			n.handlers != nil {
			return append(ciPath, n.prefix...), true
		}
	}

loop:
	// 首先，匹配静态路由
	for _, node := range n.children {
		if unicode.ToLower(rune(path[0])) == unicode.ToLower(rune(node.label)) {
			out, found := node.findCaseInsensitivePath(path, fixTrailingSlash)
			if found {
				return append(ciPath, out...), true
			}
		}
	}

	// 然后，匹配命名参数路由
	if n.paramChild != nil {
		out, found := n.paramChild.findCaseInsensitivePath(path, fixTrailingSlash)
		if found {
			return append(ciPath, out...), true
		}
	}

	// 再然后，匹配通配参数路由
	if n.anyChild != nil {
		out, found := n.anyChild.findCaseInsensitivePath(path, fixTrailingSlash)
		if found {
			return append(ciPath, out...), true
		}
	}

	// 啥也没找到。若该路径为叶子结点，我们推荐重定向到没有尾随斜杠的相同 URL
	found = fixTrailingSlash && path == "/" && n.handlers != nil
	return
}

// 在子节点中查找相同 label 的第一个子节点。
func (n *node) findChild(label byte) *node {
	for _, child := range n.children {
		if child.label == label {
			return child
		}
	}
	return nil
}

// 返回指定 label 的子节点。
// 查找优先级：子节点 > 命名参数节点 > 通配参数节点
func (n *node) findChildWithLabel(label byte) *node {
	for _, child := range n.children {
		if child.label == label {
			return child
		}
	}
	if label == paramLabel {
		return n.paramChild
	}
	if label == anyLabel {
		return n.anyChild
	}
	return nil
}

// 添加给定路径——处理链的路由到当前路由器。
func (r *router) addRoute(path string, h app.HandlersChain) {
	checkPathValid(path)

	var (
		pnames []string // 参数名称
		ppath  = path   // 路由定义的原始路径
	)

	if h == nil {
		panic(fmt.Sprintf("添加的路由必须有对应的处理器: %v", path))
	}

	// 添加非静态路由前面的静态路由部分
	for i, lcpIndex := 0, len(path); i < lcpIndex; i++ {
		// 命名参数路由
		if path[i] == paramLabel {
			j := i + 1

			r.insert(path[:i], nil, skind, nilString, nil)
			for ; i < lcpIndex && path[i] != '/'; i++ {
			}

			pnames = append(pnames, path[j:i])
			path = path[:j] + path[i:]
			i, lcpIndex = j, len(path)

			if i == lcpIndex {
				// 路径节点是路由路径的最后一个片段，如 `/users/:id`
				r.insert(path[:i], h, pkind, ppath, pnames)
				return
			} else {
				r.insert(path[:i], nil, pkind, nilString, pnames)
			}
		} else if path[i] == anyLabel {
			// 通配参数路由
			r.insert(path[:i], nil, skind, nilString, nil)
			pnames = append(pnames, path[i+1:])
			r.insert(path[:i+1], h, akind, ppath, pnames)
			return
		}
	}

	r.insert(path, h, skind, ppath, pnames)
}

// find 通过方法和路径找到对应的处理器，解析网址参数并放入上下文。
func (r *router) find(path string, paramsPointer *param.Params, unescape bool) (res nodeValue) {
	var (
		cn          = r.root // 当前节点
		search      = path   // 当前路径
		searchIndex = 0
		buf         []byte
		paramIndex  int
	)

	backtrackToNextNodeKind := func(fromKind kind) (nextNodeKind kind, valid bool) {
		previous := cn
		cn = previous.parent
		valid = cn != nil

		// 按优先级排列的下一个节点类型
		if previous.kind == akind {
			nextNodeKind = skind
		} else {
			nextNodeKind = previous.kind + 1
		}

		if fromKind == skind {
			// 当从静态类型回溯完成时，我们未改变搜索，故无需恢复
			return
		}

		// 恢复搜索为之前的值，移到我们回溯的当前节点
		if previous.kind == skind {
			searchIndex -= len(previous.prefix)
		} else {
			paramIndex--
			// 因为命名路由和通配路由的节点前缀都是 `:`，所以不能从中推导 searchIndex。
			// 而且必须对该索引使用 pValue，因为它还包含我们进入回溯节点之前截断的路径的一部分。
			searchIndex -= len((*paramsPointer)[paramIndex].Value)
			*paramsPointer = (*paramsPointer)[:paramIndex]
		}
		search = path[searchIndex:]
		return
	}

	// 搜索顺序：静态路由 > 命名参数路由 > 通配参数路由
	for {
		if cn.kind == skind {
			if len(search) >= len(cn.prefix) && cn.prefix == search[:len(cn.prefix)] {
				// Continue search
				search = search[len(cn.prefix):]
				searchIndex = searchIndex + len(cn.prefix)
			} else {
				// not equal
				if (len(cn.prefix) == len(search)+1) &&
					(cn.prefix[len(search)]) == '/' && cn.prefix[:len(search)] == search && (cn.handlers != nil || cn.anyChild != nil) {
					res.tsr = true
				}
				// No matching prefix, let's backtrack to the first possible alternative node of the decision path
				nk, ok := backtrackToNextNodeKind(skind)
				if !ok {
					return // No other possibilities on the decision path
				} else if nk == pkind {
					goto Param
				} else {
					// Not found (this should never be possible for static node we are looking currently)
					break
				}
			}
		}
		if search == nilString && len(cn.handlers) != 0 {
			res.handlers = cn.handlers
			break
		}

		// 静态节点
		if search != nilString {
			// If it can execute that logic, there is handler registered on the current node and search is `/`.
			if search == "/" && cn.handlers != nil {
				res.tsr = true
			}
			if child := cn.findChild(search[0]); child != nil {
				cn = child
				continue
			}
		}

		if search == nilString {
			if cd := cn.findChild('/'); cd != nil && (cd.handlers != nil || cd.anyChild != nil) {
				res.tsr = true
			}
		}

	Param:
		// 命名节点
		if child := cn.paramChild; search != nilString && child != nil {
			cn = child
			i := strings.Index(search, slash)
			if i == -1 {
				i = len(search)
			}
			(*paramsPointer) = (*paramsPointer)[:(paramIndex + 1)]
			val := search[:i]
			if unescape {
				if v, err := url.QueryUnescape(search[:i]); err == nil {
					val = v
				}
			}
			(*paramsPointer)[paramIndex].Value = val
			paramIndex++
			search = search[i:]
			searchIndex = searchIndex + i
			if search == nilString {
				if cd := cn.findChild('/'); cd != nil && (cd.handlers != nil || cd.anyChild != nil) {
					res.tsr = true
				}
			}
			continue
		}
	Any:
		// 通配节点
		if child := cn.anyChild; child != nil {
			// If any node is found, use remaining path for paramValues
			cn = child
			(*paramsPointer) = (*paramsPointer)[:(paramIndex + 1)]
			index := len(cn.pnames) - 1
			val := search
			if unescape {
				if v, err := url.QueryUnescape(search); err == nil {
					val = v
				}
			}

			(*paramsPointer)[index].Value = bytesconv.B2s(append(buf, val...))
			// update indexes/search in case we need to backtrack when no handler match is found
			paramIndex++
			searchIndex += len(search)
			search = nilString
			res.handlers = cn.handlers
			break
		}

		// 回到决策路径的第一个可能的替代节点
		nk, ok := backtrackToNextNodeKind(akind)
		if !ok {
			break // 决策路径上没有其他可能性
		} else if nk == pkind {
			goto Param
		} else if nk == akind {
			goto Any
		} else {
			// 未找到
			break
		}
	}

	if cn != nil {
		res.fullPath = cn.ppath
		for i, name := range cn.pnames {
			(*paramsPointer)[i].Key = name
		}
	}

	return
}

func (r *router) insert(path string, h app.HandlersChain, t kind, ppath string, pnames []string) {
	currentNode := r.root
	if currentNode == nil {
		panic("wind: 无效的路由节点")
	}
	search := path

	for {
		searchLen := len(search)
		prefixLen := len(currentNode.prefix)
		lcpLen := 0

		max := prefixLen
		if searchLen < max {
			max = searchLen
		}
		for ; lcpLen < max && search[lcpLen] == currentNode.prefix[lcpLen]; lcpLen++ {
		}

		if lcpLen == 0 {
			// 位于根节点
			currentNode.label = search[0]
			currentNode.prefix = search
			if h != nil {
				currentNode.kind = t
				currentNode.handlers = h
				currentNode.ppath = ppath
				currentNode.pnames = pnames
			}
			currentNode.isLeaf = currentNode.children == nil && currentNode.paramChild == nil && currentNode.anyChild == nil
		} else if lcpLen < prefixLen {
			// Split node
			n := newNode(
				currentNode.kind,
				currentNode.prefix[lcpLen:],
				currentNode,
				currentNode.children,
				currentNode.handlers,
				currentNode.ppath,
				currentNode.pnames,
				currentNode.paramChild,
				currentNode.anyChild,
			)
			// 将所有子节点的父路径更新到新节点
			for _, child := range currentNode.children {
				child.parent = n
			}
			if currentNode.paramChild != nil {
				currentNode.paramChild.parent = n
			}
			if currentNode.anyChild != nil {
				currentNode.anyChild.parent = n
			}

			// 重置父节点
			currentNode.kind = skind
			currentNode.label = currentNode.prefix[0]
			currentNode.prefix = currentNode.prefix[:lcpLen]
			currentNode.children = nil
			currentNode.handlers = nil
			currentNode.ppath = nilString
			currentNode.pnames = nil
			currentNode.paramChild = nil
			currentNode.anyChild = nil
			currentNode.isLeaf = false

			// 仅静态子节点可到达此处
			currentNode.children = append(currentNode.children, n)

			if lcpLen == searchLen {
				// At parent node
				currentNode.kind = t
				currentNode.handlers = h
				currentNode.ppath = ppath
				currentNode.pnames = pnames
			} else {
				// 创建子节点
				n = newNode(t, search[lcpLen:], currentNode, nil, h, ppath, pnames, nil, nil)
				// 仅静态子节点可到达此处
				currentNode.children = append(currentNode.children, n)
			}
			currentNode.isLeaf = currentNode.children == nil && currentNode.paramChild == nil && currentNode.anyChild == nil
		} else if lcpLen < searchLen {
			search = search[lcpLen:]
			c := currentNode.findChildWithLabel(search[0])
			if c != nil {
				// Go deeper
				currentNode = c
				continue
			}
			// 创建子节点
			n := newNode(t, search, currentNode, nil, h, ppath, pnames, nil, nil)
			switch t {
			case skind:
				currentNode.children = append(currentNode.children, n)
			case pkind:
				currentNode.paramChild = n
			case akind:
				currentNode.anyChild = n
			}
			currentNode.isLeaf = currentNode.children == nil && currentNode.paramChild == nil && currentNode.anyChild == nil
		} else {
			// 节点已存在
			if currentNode.handlers != nil && h != nil {
				panic("路径的处理器不可重复注册 '" + ppath + "'")
			}

			if h != nil {
				currentNode.handlers = h
				currentNode.ppath = ppath
				if len(currentNode.pnames) == 0 {
					currentNode.pnames = pnames
				}
			}
		}
		return
	}
}

func newNode(t kind, pre string, p *node, child children, mh app.HandlersChain, ppath string, pnames []string, paramChildren, anyChildren *node) *node {
	return &node{
		kind:       t,
		label:      pre[0],
		prefix:     pre,
		parent:     p,
		children:   child,
		ppath:      ppath,
		pnames:     pnames,
		handlers:   mh,
		paramChild: paramChildren,
		anyChild:   anyChildren,
		isLeaf:     child == nil && paramChildren == nil && anyChildren == nil,
	}
}

// 获取路径中命名参数和通配参数的个数。
func countParams(path string) uint16 {
	var n uint16
	s := bytesconv.S2b(path)
	n += uint16(bytes.Count(s, bytestr.StrColon))
	n += uint16(bytes.Count(s, bytestr.StrStar))
	return n
}

func checkPathValid(path string) {
	if path == nilString {
		panic("路由路径不能为空字符串")
	}
	if path[0] != '/' {
		panic("路由路径必须以 '/' 开头")
	}
	for i, c := range []byte(path) {
		switch c {
		case ':':
			if (i < len(path)-1 && path[i+1] == '/') || i == len(path)-1 {
				panic("命名标识符必须使用非空名称进行命名 '" + path + "'")
			}
			i++
			for ; i < len(path) && path[i] != '/'; i++ {
				if path[i] == ':' || path[i] == '*' {
					panic("每个路径段中只允许一个标识符，发现多个：'" + path + "'")
				}
			}
		case '*':
			if i == len(path)-1 {
				panic("通配标识符必须使用非空名称进行命名 '" + path + "'")
			}
			if i > 0 && path[i-1] != '/' {
				panic("通配符 '*' 之前必须有 '/'，" + path)
			}
			for ; i < len(path); i++ {
				if path[i] == '/' {
					panic("通配符必须位于路径末尾 '" + path + "'")
				}
			}
		}
	}
}
