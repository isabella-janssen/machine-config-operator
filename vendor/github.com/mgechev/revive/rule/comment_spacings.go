package rule

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mgechev/revive/lint"
)

// CommentSpacingsRule check the whether there is a space between
// the comment symbol( // ) and the start of the comment text
type CommentSpacingsRule struct {
	allowList []string

	configureOnce sync.Once
}

func (r *CommentSpacingsRule) configure(arguments lint.Arguments) {
	r.allowList = []string{}
	for _, arg := range arguments {
		allow, ok := arg.(string) // Alt. non panicking version
		if !ok {
			panic(fmt.Sprintf("invalid argument %v for %s; expected string but got %T", arg, r.Name(), arg))
		}
		r.allowList = append(r.allowList, `//`+allow)
	}
}

// Apply the rule.
func (r *CommentSpacingsRule) Apply(file *lint.File, args lint.Arguments) []lint.Failure {
	r.configureOnce.Do(func() { r.configure(args) })

	var failures []lint.Failure

	for _, cg := range file.AST.Comments {
		for _, comment := range cg.List {
			commentLine := comment.Text
			if len(commentLine) < 3 {
				continue // nothing to do
			}

			isMultiLineComment := commentLine[1] == '*'
			isOK := commentLine[2] == '\n'
			if isMultiLineComment && isOK {
				continue
			}

			isOK = (commentLine[2] == ' ') || (commentLine[2] == '\t')
			if isOK {
				continue
			}

			if r.isAllowed(commentLine) {
				continue
			}

			failures = append(failures, lint.Failure{
				Node:       comment,
				Confidence: 1,
				Category:   "style",
				Failure:    "no space between comment delimiter and comment text",
			})
		}
	}
	return failures
}

// Name yields this rule name.
func (*CommentSpacingsRule) Name() string {
	return "comment-spacings"
}

func (r *CommentSpacingsRule) isAllowed(line string) bool {
	for _, allow := range r.allowList {
		if strings.HasPrefix(line, allow) {
			return true
		}
	}

	return isDirectiveComment(line)
}
