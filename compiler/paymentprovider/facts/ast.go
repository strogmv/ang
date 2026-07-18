package ppfacts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

type methodImplementation map[string]string

func classifyProviderMethods(projectPath, structName string) (methodImplementation, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, projectPath, func(info os.FileInfo) bool {
		name := info.Name()
		if info.IsDir() {
			return false
		}
		if !strings.HasSuffix(name, ".go") {
			return false
		}
		if strings.HasPrefix(name, ".") {
			return false
		}
		return true
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	result := methodImplementation{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			inspectProviderMethods(file, structName, result)
			inspectCryptoBehaviors(file, result)
		}
	}
	return result, nil
}

func inspectProviderMethods(file *ast.File, structName string, result methodImplementation) {
	ast.Inspect(file, func(node ast.Node) bool {
		fn, ok := node.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name == nil {
			return true
		}
		if !receiverMatches(fn.Recv, structName) {
			return true
		}
		op := goMethodToOperation(fn.Name.Name)
		if op == "" {
			return true
		}
		result[op] = classifyMethodBody(fn.Body)
		return true
	})
}

func receiverMatches(recv *ast.FieldList, structName string) bool {
	if recv == nil || len(recv.List) == 0 {
		return false
	}
	typ := recv.List[0].Type
	switch t := typ.(type) {
	case *ast.StarExpr:
		typ = t.X
	case *ast.Ident:
		// keep
	default:
		return false
	}
	ident, ok := typ.(*ast.Ident)
	return ok && ident.Name == structName
}

func classifyMethodBody(body *ast.BlockStmt) string {
	if body == nil || len(body.List) == 0 {
		return "unknown"
	}
	if len(body.List) == 1 {
		if ret, ok := body.List[0].(*ast.ReturnStmt); ok && isNotImplementedReturn(ret) {
			return "stub"
		}
	}
	if len(body.List) >= 1 {
		return "implemented"
	}
	return "unknown"
}

func isNotImplementedReturn(ret *ast.ReturnStmt) bool {
	if ret == nil || len(ret.Results) == 0 {
		return false
	}
	for _, result := range ret.Results {
		if isErrNotImplementedExpr(result) {
			return true
		}
	}
	return false
}

func isErrNotImplementedExpr(expr ast.Expr) bool {
	switch node := expr.(type) {
	case *ast.SelectorExpr:
		return node.Sel != nil && node.Sel.Name == "ErrNotImplemented"
	case *ast.Ident:
		return node.Name == "ErrNotImplemented"
	default:
		return false
	}
}

func inspectCryptoBehaviors(file *ast.File, result methodImplementation) {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		name := path
		if spec.Name != nil {
			name = spec.Name.Name
		} else {
			if idx := strings.LastIndex(path, "/"); idx >= 0 {
				name = path[idx+1:]
			}
		}
		imports[name] = path
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil {
			return true
		}
	 pkgName := exprName(sel.X)
	 importPath, ok := imports[pkgName]
	 if !ok || importPath != "crypto/rsa" {
		 return true
	 }
	 switch sel.Sel.Name {
	 case "EncryptOAEP":
		 result["behavior:rsa_oaep_card_encryption"] = "present"
	 case "VerifyPKCS1v15":
		 result["behavior:rsa_pkcs1v15_callback_verification"] = "present"
	 }
	 return true
	})
}

func exprName(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.SelectorExpr:
		if node.Sel != nil {
			return node.Sel.Name
		}
	}
	return ""
}

func goMethodToOperation(name string) string {
	switch name {
	case "InitPay":
		return "init_pay"
	case "InitPayout":
		return "init_payout"
	case "CheckStatus":
		return "check_status"
	case "InitRefund":
		return "init_refund"
	case "InitPayP2P":
		return "init_pay_p2p"
	case "CancelPay":
		return "cancel_pay"
	case "InitSubscription":
		return "init_subscription"
	case "SubscriptionPay":
		return "subscription_pay"
	case "ParseCallback":
		return "parse_callback"
	case "ValidateCallback":
		return "validate_callback"
	case "FinishCallback":
		return "finish_callback"
	default:
		return ""
	}
}
