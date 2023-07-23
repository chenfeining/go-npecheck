package go_npecheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "npecheck",
	Doc:  Doc,
	Run:  Run,
}

const Doc = "check potential nil pointer reference"

type CheckPointerPosition struct {
	Line      int
	Colum     int
	IsChecked bool
	Type      int // DefaultPtrType, SlicePtrType, ParentPtrCurNonType
}

const (
	DefaultPtrType      int = 0 // ptr
	SlicePtrType        int = 1 // []ptr
	ParentPtrCurNonType int = 2 //  A.B.GNode , A is ptr，B is not ptr
)

func IsPointer(typ types.Type) bool {
	_, ok := typ.(*types.Pointer)
	return ok
}

func IsSliceIncludePointerElem(typ types.Type) bool {
	var sliceElem, isSlice = typ.(*types.Slice)
	if isSlice {
		elem := sliceElem.Elem()
		if _, ok := elem.(*types.Pointer); ok {
			return true
		}
	}
	return false
}

func GetNodeType(ident *ast.Ident, typesInfo *types.Info) (int, bool) {
	var (
		nodeType           = NodeTypeDefaultSinglePtr
		isReturnSingleFunc = false
	)
	obj := typesInfo.ObjectOf(ident)

	sign, ok := obj.Type().(*types.Signature)
	if ok && sign != nil && sign.Results() != nil && sign.Results().Len() == 1 { // 函数、方法
		isReturnSingleFunc = true
		retType := sign.Results().At(0).Type()
		if !IsPointer(retType) {
			nodeType = NodeTypeNonSinglePtr
		}
	} else {
		if !IsPointer(obj.Type()) {
			nodeType = NodeTypeNonSinglePtr
		}
	}

	return nodeType, isReturnSingleFunc
}

func IsPointerArray(ident *ast.Ident, info *types.Info) bool {
	obj := info.ObjectOf(ident)
	if obj == nil {
		// ident 不是一个有效的标识符
		return false
	}

	typ := obj.Type()
	arr, ok := typ.(*types.Slice)
	if !ok {
		// ident 不是一个数组类型
		return false
	}

	elem := arr.Elem()
	_, ok = elem.(*types.Pointer)
	if ok {
		return true
	}

	return false
}

func GetIdentPosition(p *token.Position, ident *ast.Ident, fset *token.FileSet) {
	*p = fset.Position(ident.Pos())
}

func WalkSelector(expr *ast.SelectorExpr, fset *token.FileSet, walkFunc func(ast.Node)) {
	walkFunc(expr)

	ident, ok := expr.X.(*ast.Ident)
	if ok {
		walkFunc(ident)
		return
	}

	if se, ok := expr.X.(*ast.SelectorExpr); ok {
		WalkSelector(se, fset, walkFunc)
	}
}

func (f *FuncDelChecker) isRootComeFromOutside(varName string) bool {
	if varName == "" {
		return false
	}

	varNameNodes := strings.Split(varName, ".")
	if len(varNameNodes) >= 1 {
		if _, ok := f.needCheckPointerPositionMap[varNameNodes[0]]; ok {
			return true
		}
	}

	return false
}

func (f *FuncDelChecker) isExistFuncRetPtr(nodeList []*SelectNode, isNeedRemoveLeaf bool) (bool, int) {
	if len(nodeList) == 0 {
		return false, -1
	}

	if isNeedRemoveLeaf {
		nodeList = nodeList[0 : len(nodeList)-1]
	}

	for index, n := range nodeList {
		if n.Type == NodeTypeDefaultSinglePtr && n.IsReturnSingleFunc == true {
			return true, index
		}
	}

	return false, -2
}

func (f *FuncDelChecker) recordIfBinaryNilValidation(binaryExpr *ast.BinaryExpr, fset *token.FileSet, lintErrors *[]*LintError) {
	if binaryExpr == nil {
		return
	}
	y := binaryExpr.Y
	if ident, ok := y.(*ast.Ident); ok {
		if ident.Name != "nil" { // 后面可以看看这里有无其他项目种常用的alias, 比如RedisNil
			return
		}

		if binaryExpr.Op != token.NEQ && binaryExpr.Op != token.EQL { // 只要 != nil , == nil 判断
			return
		}
		x := binaryExpr.X
		if expr, ok := x.(*ast.Ident); ok {
			var pos token.Position
			GetIdentPosition(&pos, expr, fset)
			if f.isRootComeFromOutside(expr.Name) /*|| (isReturnSingleFunc == true && nodeType == NodeTypeDefaultSinglePtr) */ {
				checkedPos := &CheckPointerPosition{
					Line:      pos.Line,
					Colum:     pos.Column,
					IsChecked: true,
				}

				index := f.findFirstSuitablePosIndexFromStart(expr.Name, pos)
				if index >= 0 {
					if f.needCheckPointerPositionMap[expr.Name][index].IsChecked != true {
						f.needCheckPointerPositionMap[expr.Name][index] = checkedPos

						if originName, ok := f.originFieldMap[expr.Name]; ok {
							f.needCheckPointerPositionMap[originName] = f.needCheckPointerPositionMap[expr.Name]
						}
					}
				}
			}
		}
	}

	binaryExpr = f.recordBinaryExpr(binaryExpr, fset, lintErrors)
}

func (f *FuncDelChecker) recordBinaryExpr(
	expr *ast.BinaryExpr,
	fset *token.FileSet,
	lintErrorList *[]*LintError) *ast.BinaryExpr {
	left := expr.X
	right := expr.Y
	switch leftExpr := left.(type) {
	case *ast.BinaryExpr:
		x := leftExpr.X
		switch expr := x.(type) {
		case *ast.SelectorExpr:
			_ = f.travelSelectorNameAndFuncWithRecord(expr, fset, lintErrorList)
		case *ast.Ident:
			f.recordIdentCheckedPosition(expr, fset)
		}
		f.recordBinaryExpr(leftExpr, fset, lintErrorList)

	case *ast.SelectorExpr:
		_ = f.travelSelectorNameAndFuncWithRecord(leftExpr, fset, lintErrorList)

	case *ast.CallExpr:
		if selectExpr, ok := leftExpr.Fun.(*ast.SelectorExpr); ok {
			_ = f.travelSelectorNameAndFuncWithRecord(selectExpr, fset, lintErrorList)
		}
	}

	switch rightExpr := right.(type) {
	case *ast.BinaryExpr:
		x := rightExpr.X
		switch expr := x.(type) {
		case *ast.SelectorExpr:
			_ = f.travelSelectorNameAndFuncWithRecord(expr, fset, lintErrorList)

		case *ast.Ident:
			f.recordIdentCheckedPosition(expr, fset)
		}
		f.recordBinaryExpr(rightExpr, fset, lintErrorList)

	case *ast.SelectorExpr:
		_ = f.travelSelectorNameAndFuncWithRecord(rightExpr, fset, lintErrorList)

	case *ast.CallExpr:
		if selectExpr, ok := rightExpr.Fun.(*ast.SelectorExpr); ok {
			_ = f.travelSelectorNameAndFuncWithRecord(selectExpr, fset, lintErrorList)
		}
	}

	newExpr := &ast.BinaryExpr{
		Op: expr.Op,
		X:  left,
		Y:  right,
	}

	return newExpr
}

func (f *FuncDelChecker) isExistRecord(name string) ([]*CheckPointerPosition, bool) {
	recordList, ok := f.needCheckPointerPositionMap[name]
	return recordList, ok
}

func (f *FuncDelChecker) findFirstSuitablePosIndexFromStart(name string, pos token.Position) int {
	needCheckPointerPositionList, ok := f.needCheckPointerPositionMap[name]
	if !ok || len(needCheckPointerPositionList) == 0 {
		return -1
	}

	if len(needCheckPointerPositionList) == 1 {
		return 0
	}

	for i, v := range needCheckPointerPositionList {
		if pos.Line > v.Line || (pos.Line == v.Line && pos.Column > v.Colum) {
			nextIndex := i + 1
			if nextIndex == len(needCheckPointerPositionList) {
				return i
			}

			nextV := needCheckPointerPositionList[nextIndex]
			if pos.Line < nextV.Line || (pos.Line == v.Line && pos.Column < nextV.Colum) {
				return i
			}
		}
	}

	return -2
}

func (f *FuncDelChecker) findFirstSuitablePosIndexFromEnd(name string, pos token.Position) int {
	needCheckPointerPositionList, ok := f.needCheckPointerPositionMap[name]
	if !ok {
		return -1
	}

	index := len(needCheckPointerPositionList) - 1
	for index >= 0 {
		if needCheckPointerPositionList[index].Line < pos.Line ||
			(needCheckPointerPositionList[index].Line == pos.Line &&
				needCheckPointerPositionList[index].Colum < pos.Column) {
			return index
		}

		index--
	}

	return -2
}

func (f *FuncDelChecker) recordIdentCheckedPosition(expr *ast.Ident, fset *token.FileSet) {
	var pos token.Position
	GetIdentPosition(&pos, expr, fset)
	if f.isRootComeFromOutside(expr.Name) {
		checkedPosition := &CheckPointerPosition{
			Line:      pos.Line,
			Colum:     pos.Column,
			IsChecked: true,
		}

		index := f.findFirstSuitablePosIndexFromStart(expr.Name, pos)
		if index >= 0 {
			if f.needCheckPointerPositionMap[expr.Name][index].IsChecked != true {
				f.needCheckPointerPositionMap[expr.Name][index] = checkedPosition
				if originName, ok := f.originFieldMap[expr.Name]; ok {
					f.needCheckPointerPositionMap[originName] = f.needCheckPointerPositionMap[expr.Name]
				}
			}
		}
	}
}

func (f *FuncDelChecker) preRecordNilPointerFromOutside(fnDel *ast.FuncDecl, lintErrors *[]*LintError) {
	if fnDel == nil {
		return
	}

	var (
		typeInfo = f.pass.TypesInfo
		fset     = f.pass.Fset
	)

	if fnDel.Type != nil && fnDel.Type.Params != nil {
		for _, field := range fnDel.Type.Params.List {
			for _, name := range field.Names {
				typ := typeInfo.Types[field.Type].Type
				var pos token.Position
				GetIdentPosition(&pos, name, fset)
				if IsPointer(typ) {
					f.needCheckPointerPositionMap[name.Name] = []*CheckPointerPosition{
						{
							Line:      pos.Line,
							Colum:     pos.Column,
							IsChecked: false,
						},
					}
				} else if IsSliceIncludePointerElem(typ) {
					f.needCheckPointerPositionMap[name.Name] = []*CheckPointerPosition{
						{
							Line:      pos.Line,
							Colum:     pos.Column,
							IsChecked: true,
							Type:      SlicePtrType,
						},
					}
				}
			}
		}
	}

	// 获取函数外部赋值的指针变量
	for _, stmt := range fnDel.Body.List {
		f.recordStmtNilValidation(stmt, fset, lintErrors, typeInfo)
	}
}

func (f *FuncDelChecker) recordStmtNilValidation(stmt ast.Stmt, fset *token.FileSet, lintErrors *[]*LintError, typeInfo *types.Info) {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		f.recordIfStmtNilValidation(s, fset, lintErrors, typeInfo)

	case *ast.AssignStmt:
		f.recordAssignmentStmtNilValidation(s, typeInfo, fset)

	case *ast.RangeStmt:
		f.recordRangeStmtNilValidation(s, typeInfo, fset, lintErrors)

	case *ast.SwitchStmt:
		f.recordSwitchStmtNilValidation(s, typeInfo, fset, lintErrors)
	}
}

func (f *FuncDelChecker) recordIfStmtNilValidation(s *ast.IfStmt, fset *token.FileSet, lintErrors *[]*LintError, typeInfo *types.Info) {
	cond := s.Cond
	switch cond := cond.(type) {
	case *ast.BinaryExpr:
		f.recordIfBinaryNilValidation(cond, fset, lintErrors)
	}

	for _, stmt := range s.Body.List {
		switch expr := stmt.(type) {
		case *ast.IfStmt, *ast.AssignStmt, *ast.RangeStmt:
			f.recordStmtNilValidation(expr, fset, lintErrors, typeInfo)
		}
	}
}

func (f *FuncDelChecker) recordSwitchStmtNilValidation(s *ast.SwitchStmt, typeInfo *types.Info, fset *token.FileSet, lintErrors *[]*LintError) {
	if s == nil {
		return
	}

	if s.Body == nil {
		return
	}

	for _, c := range s.Body.List {
		if caseClause, ok := c.(*ast.CaseClause); ok {
			for _, b := range caseClause.Body {
				switch expr := b.(type) {
				case *ast.AssignStmt, *ast.IfStmt, *ast.SwitchStmt, *ast.RangeStmt:
					f.recordStmtNilValidation(expr, fset, lintErrors, typeInfo)
				}
			}
		}
	}
}

func (f *FuncDelChecker) recordRangeStmtNilValidation(s *ast.RangeStmt, typeInfo *types.Info, fset *token.FileSet, lintErrors *[]*LintError) {
	var (
		x               = s.X
		rangeParentName string
	)

	switch expr := x.(type) {
	case *ast.Ident:
		rangeParentName = expr.Name
		if _, ok := f.needCheckPointerPositionMap[rangeParentName]; ok {
			value := s.Value
			switch value.(type) {
			case *ast.Ident:
				f.recordRangeValue(s, fset)
			}
		}

	case *ast.SelectorExpr:
		innerX := expr.X
		switch innerX := innerX.(type) {
		case *ast.Ident:
			rangeParentName = innerX.Name
			if _, ok := f.needCheckPointerPositionMap[rangeParentName]; ok {
				if IsPointerArray(expr.Sel, f.pass.TypesInfo) {
					lintErrorList := f.detectSelectorReferenceWithFunc(expr, fset)
					if len(lintErrorList) > 0 {
						*lintErrors = append(*lintErrors, lintErrorList...)
					}

					f.recordRangeValue(s, fset)
				}
			}
		}

	case *ast.CallExpr:
		if exprFun, ok := expr.Fun.(*ast.SelectorExpr); ok {
			lintErrorList := f.detectSelectorReferenceWithFunc(exprFun, fset)
			if len(lintErrorList) > 0 {
				*lintErrors = append(*lintErrors, lintErrorList...)
			}
		}

		sign := GetFuncSignature(expr, typeInfo)
		if sign.Results().Len() != 1 {
			return
		}

		retType := sign.Results().At(0).Type()
		if IsSliceIncludePointerElem(retType) {
			f.recordRangeValue(s, fset)
		}
	}

	body := s.Body
	if body == nil {
		return
	}

	for _, b := range body.List {
		switch expr := b.(type) {
		case *ast.IfStmt, *ast.AssignStmt, *ast.RangeStmt, *ast.SwitchStmt:
			f.recordStmtNilValidation(expr, fset, lintErrors, typeInfo)
		}
	}
}

func (f *FuncDelChecker) recordRangeValue(s *ast.RangeStmt, fset *token.FileSet) {
	value := s.Value
	if v, ok := value.(*ast.Ident); ok {
		var pos token.Position
		GetIdentPosition(&pos, v, fset)
		needCheckPos := &CheckPointerPosition{
			Line:      pos.Line,
			Colum:     pos.Column,
			IsChecked: false,
		}
		recordList, ok := f.isExistRecord(v.Name)
		if ok {
			f.needCheckPointerPositionMap[v.Name] = append(recordList, needCheckPos)
		} else {
			f.needCheckPointerPositionMap[v.Name] = []*CheckPointerPosition{needCheckPos}
		}
	}
}

func (f *FuncDelChecker) recordAssignmentStmtNilValidation(s *ast.AssignStmt, typeInfo *types.Info, fset *token.FileSet) {
	var respVarNameList = make([]string, 0)
	for _, l := range s.Lhs {
		if i, ok := l.(*ast.Ident); ok {
			respVarNameList = append(respVarNameList, i.Name)
		}
	}

	for _, expr := range s.Rhs {
		switch EX := expr.(type) {
		case *ast.SelectorExpr:
			obj := typeInfo.ObjectOf(EX.Sel)
			if IsPointer(obj.Type()) {
				fieldNameList := TravelSelectorName(EX, fset)
				if len(fieldNameList) == 0 {
					continue
				}

				if _, ok := f.needCheckPointerPositionMap[fieldNameList[0]]; ok {
					key := strings.Join(fieldNameList, ".")
					if len(respVarNameList) == 1 {
						f.originFieldMap[respVarNameList[0]] = key
						var pos token.Position
						GetIdentPosition(&pos, EX.Sel, fset)
						needCheckedPos := &CheckPointerPosition{
							Line:      pos.Line,
							Colum:     pos.Column,
							IsChecked: false,
						}

						recordList, ok := f.isExistRecord(respVarNameList[0])
						if ok {
							f.needCheckPointerPositionMap[respVarNameList[0]] = append(recordList, needCheckedPos)
						} else {
							f.needCheckPointerPositionMap[respVarNameList[0]] = []*CheckPointerPosition{needCheckedPos}
						}
						f.needCheckPointerPositionMap[key] = f.needCheckPointerPositionMap[respVarNameList[0]]
					}
				}
			}

		case *ast.CallExpr:
			sign := GetFuncSignature(EX, typeInfo)
			if sign == nil {
				continue
			}

			respVarNameListLength := len(respVarNameList)
			if respVarNameListLength > 0 && sign != nil && sign.Results() != nil && respVarNameListLength == sign.Results().Len() {
				for index, varName := range respVarNameList {
					retType := sign.Results().At(index).Type()
					pos := fset.Position(EX.Rparen)
					var (
						needCheckedPos *CheckPointerPosition
						isNeedRecord   bool
					)
					if IsSliceIncludePointerElem(retType) {
						needCheckedPos = &CheckPointerPosition{
							Line:      pos.Line,
							Colum:     pos.Column,
							IsChecked: true,
						}
						isNeedRecord = true
					}

					if IsPointer(retType) {
						needCheckedPos = &CheckPointerPosition{
							Line:      pos.Line,
							Colum:     pos.Column,
							IsChecked: false,
						}
						isNeedRecord = true
					}

					if isNeedRecord {
						recordList, ok := f.isExistRecord(varName)
						if ok {
							f.needCheckPointerPositionMap[varName] = append(recordList, needCheckedPos)
						} else {
							f.needCheckPointerPositionMap[varName] = []*CheckPointerPosition{needCheckedPos}
						}
					}
				}
			}
		}
	}
}

type LintError struct {
	Message string
	File    string
	Line    int
	Colum   int
}

const NPEMessageTipInfo = "potential nil pointer reference"

// 实现 Error 方法
func (err *LintError) Error() string {
	return fmt.Sprintf("%s: %s:%d:%d", err.Message, err.File, err.Line, err.Colum)
}

func RemoveVarLeafNode(varName string, originFieldMap map[string]string) string {
	if varName == "" {
		return ""
	}

	parts := strings.Split(varName, ".")
	var substr string
	if len(parts) > 1 {
		substr = strings.Join(parts[:len(parts)-1], ".")
	}

	if originName, ok := originFieldMap[substr]; ok {
		substr = originName
	}

	return substr
}

type SelectNode struct {
	Name               string
	Type               int // NodeTypeDefaultSinglePtr, NodeTypeNonSinglePtr
	IsReturnSingleFunc bool
	CurIdent           *ast.Ident
}

const (
	NodeTypeDefaultSinglePtr int = 0
	NodeTypeNonSinglePtr     int = 1
)

func buildFieldNameFromNodes(nodes []*SelectNode) string {
	fieldNameList := make([]string, 0)
	for _, n := range nodes {
		fieldNameList = append(fieldNameList, n.Name)
	}

	return strings.Join(fieldNameList, ".")
}

func walkSelectorWithFunc(expr *ast.SelectorExpr, fset *token.FileSet, walkFunc func(ast.Node)) {
	walkFunc(expr)

	switch ex := expr.X.(type) {
	case *ast.Ident:
		walkFunc(ex)
		return

	case *ast.SelectorExpr:
		walkSelectorWithFunc(ex, fset, walkFunc)

	case *ast.CallExpr:
		if se, ok := ex.Fun.(*ast.SelectorExpr); ok {
			walkSelectorWithFunc(se, fset, walkFunc)
		}

		if se, ok := ex.Fun.(*ast.Ident); ok {
			walkFunc(se)
			return
		}
	}
}

func (f *FuncDelChecker) travelSelectorNameAndFuncWithRecord(
	expr *ast.SelectorExpr,
	fset *token.FileSet,
	lintErrorList *[]*LintError,
) []*SelectNode {
	var nodeList = make([]*SelectNode, 0)
	walkSelectorWithFunc(expr, fset, func(node ast.Node) {
		switch exprInner := node.(type) {
		case *ast.SelectorExpr:
			if exprInner != nil && exprInner.Sel != nil {
				nodeType, isReturnSingleFunc := GetNodeType(exprInner.Sel, f.pass.TypesInfo) // 可以再记录一个是否是函数
				nodeList = append(nodeList, &SelectNode{
					Name:               exprInner.Sel.Name,
					CurIdent:           exprInner.Sel,
					Type:               nodeType,
					IsReturnSingleFunc: isReturnSingleFunc,
				})
			}

		case *ast.CallExpr:
			switch exFunc := exprInner.Fun.(type) {
			case *ast.SelectorExpr:
				if exFunc != nil && exFunc.Sel != nil {
					nodeType, isReturnSingleFunc := GetNodeType(exFunc.Sel, f.pass.TypesInfo)
					nodeList = append(nodeList, &SelectNode{
						Name:               exFunc.Sel.Name,
						CurIdent:           exFunc.Sel,
						Type:               nodeType,
						IsReturnSingleFunc: isReturnSingleFunc})
				}

			case *ast.Ident:
				nodeType, isReturnSingleFunc := GetNodeType(exFunc, f.pass.TypesInfo)
				nodeList = append(nodeList, &SelectNode{Name: exFunc.Name,
					CurIdent:           exFunc,
					Type:               nodeType,
					IsReturnSingleFunc: isReturnSingleFunc,
				})
			}

		case *ast.Ident:
			if exprInner != nil {
				nodeType, isReturnSingleFunc := GetNodeType(exprInner, f.pass.TypesInfo)
				nodeList = append(nodeList, &SelectNode{
					Name:               exprInner.Name,
					CurIdent:           exprInner,
					Type:               nodeType,
					IsReturnSingleFunc: isReturnSingleFunc})
				sort.SliceStable(nodeList, func(i, j int) bool { // 反转数组
					return i > j
				})
			}

			var pos token.Position
			GetIdentPosition(&pos, exprInner, fset)
			fieldName := buildFieldNameFromNodes(nodeList)
			//_, funcIndex := f.isExistFuncRetPtr(nodeList, false)
			isRootFromOutside := f.isRootComeFromOutside(fieldName)
			if isRootFromOutside /*|| isExistFunc*/ {
				lintErrors := f.hasSequenceDetectNode(nodeList, pos, exprInner, isRootFromOutside)
				if len(lintErrors) != 0 {
					*lintErrorList = append(*lintErrorList, lintErrors...)
				} else {
					checkedPosition := &CheckPointerPosition{
						Line:      pos.Line,
						Colum:     pos.Column,
						IsChecked: true,
						Type:      DefaultPtrType,
					}

					leafNodeType := nodeList[len(nodeList)-1].Type
					if leafNodeType == NodeTypeNonSinglePtr {
						checkedPosition = &CheckPointerPosition{
							Line:      0,
							Colum:     0,
							IsChecked: true,
							Type:      ParentPtrCurNonType,
						}
					}

					index := f.findFirstSuitablePosIndexFromStart(fieldName, pos)
					if index >= 0 {
						if f.needCheckPointerPositionMap[fieldName][index].IsChecked != true {
							f.needCheckPointerPositionMap[fieldName][index] = checkedPosition
						}
					} else {
						f.needCheckPointerPositionMap[fieldName] = []*CheckPointerPosition{checkedPosition}
					}

					if originName, ok := f.originFieldMap[fieldName]; ok {
						f.needCheckPointerPositionMap[originName] = f.needCheckPointerPositionMap[fieldName]
					}
				}
			}
		}
	})

	return nodeList
}

func TravelSelectorName(expr *ast.SelectorExpr, fset *token.FileSet) []string {
	var nodeNameList []string
	WalkSelector(expr, fset, func(node ast.Node) {
		switch exprInner := node.(type) {
		case *ast.SelectorExpr:
			if exprInner != nil && exprInner.Sel != nil {
				nodeNameList = append(nodeNameList, exprInner.Sel.Name)
			}

		case *ast.Ident:
			if exprInner != nil {
				nodeNameList = append(nodeNameList, exprInner.Name)
				sort.SliceStable(nodeNameList, func(i, j int) bool { // 反转数组
					return i > j
				})
			}
		}
	})

	return nodeNameList
}

func (f *FuncDelChecker) getPotentialNilPointerReference(
	varName string,
	filePath string,
	pos *token.Position,
	expr *ast.Ident,
	isComeFromOutSide bool,
) *LintError {
	if pos == nil || len(f.needCheckPointerPositionMap) == 0 {
		return nil
	}

	refName := RemoveVarLeafNode(varName, f.originFieldMap)
	if refName == "" {
		return nil
	}

	//needCheckInfo, ok := needCheckPointerPositionMap[refName]
	index := f.findFirstSuitablePosIndexFromEnd(refName, *pos)

	if isComeFromOutSide && index < 0 {
		f.pass.Reportf(expr.Pos(), NPEMessageTipInfo)
		return &LintError{
			Message: NPEMessageTipInfo,
			File:    filePath,
			Line:    pos.Line,
			Colum:   pos.Column,
		}
	}

	if index >= 0 {
		if lintError := f.buildLintError(f.needCheckPointerPositionMap[refName][index], filePath, pos); lintError != nil {
			f.pass.Reportf(expr.Pos(), NPEMessageTipInfo)
			return lintError
		}
	}

	return nil
}

func (f *FuncDelChecker) buildLintError(needCheckInfo *CheckPointerPosition, filePath string, pos *token.Position) *LintError {
	lintErr := &LintError{
		Message: NPEMessageTipInfo,
		File:    filePath,
		Line:    pos.Line,
		Colum:   pos.Column,
	}

	if needCheckInfo == nil {
		return lintErr
	}

	if needCheckInfo.IsChecked == false {
		return lintErr
	}

	if needCheckInfo.Line > pos.Line || (needCheckInfo.Line == pos.Line && needCheckInfo.Colum > pos.Column) {
		return lintErr
	}
	return nil
}

func Run(pass *analysis.Pass) (interface{}, error) {
	var (
		fset          = pass.Fset
		lintErrorList = make([]*LintError, 0)
	)

	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				checker := InitFuncDelChecker(pass)
				checker.preRecordNilPointerFromOutside(decl, &lintErrorList)
				checker.detectNilPointerReference(decl, fset, &lintErrorList)
			}
		}
	}
	// fmt.Println(lintErrorList)
	return nil, nil
}

type FuncDelChecker struct {
	pass *analysis.Pass

	originFieldMap              map[string]string
	needCheckPointerPositionMap map[string][]*CheckPointerPosition
}

func InitFuncDelChecker(pass *analysis.Pass) *FuncDelChecker {
	if pass == nil {
		return nil
	}

	return &FuncDelChecker{
		pass:                        pass,
		originFieldMap:              make(map[string]string),
		needCheckPointerPositionMap: make(map[string][]*CheckPointerPosition),
	}
}

func (f *FuncDelChecker) detectNilPointerReference(
	decl *ast.FuncDecl,
	fset *token.FileSet,
	lintErrorList *[]*LintError) {
	for _, stmt := range decl.Body.List {
		f.detectNPEInStatement(stmt, fset, lintErrorList)
	}
}

func (f *FuncDelChecker) detectNPEInStatement(stmt ast.Stmt, fset *token.FileSet, npeLintErrorListPtr *[]*LintError) {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		f.detectIfStatementBlock(s, fset, npeLintErrorListPtr)

	case *ast.AssignStmt:
		f.detectAssignmentStatementBlock(s, fset, npeLintErrorListPtr)

	case *ast.ExprStmt:
		f.detectExprStatementBlock(s, fset, npeLintErrorListPtr)

	case *ast.RangeStmt:
		f.detectRangeStatementBlock(s, fset, npeLintErrorListPtr)

	case *ast.SwitchStmt:
		f.detectSwitchStatementBlock(s, fset, npeLintErrorListPtr)
	}
}

func (f *FuncDelChecker) detectSwitchStatementBlock(s *ast.SwitchStmt, fset *token.FileSet, npeLintErrorListPtr *[]*LintError) {
	if s == nil {
		return
	}

	if s.Body == nil {
		return
	}

	for _, b := range s.Body.List {
		switch bStmt := b.(type) {
		case *ast.CaseClause:
			for _, expr := range bStmt.Body {
				switch expr := expr.(type) {
				case *ast.IfStmt, *ast.AssignStmt, *ast.ExprStmt, *ast.RangeStmt, *ast.SwitchStmt:
					f.detectNPEInStatement(expr, fset, npeLintErrorListPtr)
				}
			}
		}
	}
}

func (f *FuncDelChecker) detectRangeStatementBlock(s *ast.RangeStmt, fset *token.FileSet, npeLintErrorListPtr *[]*LintError) {
	if s == nil {
		return
	}

	if s.Body == nil {
		return
	}

	for _, b := range s.Body.List {
		switch bStmt := b.(type) {
		case *ast.IfStmt, *ast.AssignStmt, *ast.ExprStmt, *ast.RangeStmt, *ast.SwitchStmt:
			f.detectNPEInStatement(bStmt, fset, npeLintErrorListPtr)
		}
	}
}

func (f *FuncDelChecker) detectExprStatementBlock(s *ast.ExprStmt, fset *token.FileSet, lintErrorListPtr *[]*LintError) {
	switch callExpr := s.X.(type) {
	case *ast.CallExpr:
		if selectExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
			lintErrors := f.detectSelectorReferenceWithFunc(selectExpr, fset)
			if len(lintErrors) > 0 {
				*lintErrorListPtr = append(*lintErrorListPtr, lintErrors...)
			}
		}

		for _, expr := range callExpr.Args {
			switch expr := expr.(type) {
			case *ast.SelectorExpr:
				lintErrors := f.detectSelectorReferenceWithFunc(expr, fset)
				if len(lintErrors) > 0 {
					*lintErrorListPtr = append(*lintErrorListPtr, lintErrors...)
				}

			case *ast.CallExpr:
				if selectExpr, ok := expr.Fun.(*ast.SelectorExpr); ok {
					lintErrors := f.detectSelectorReferenceWithFunc(selectExpr, fset)
					if len(lintErrors) > 0 {
						*lintErrorListPtr = append(*lintErrorListPtr, lintErrors...)
					}
				}
			}
		}

	}
}

func (f *FuncDelChecker) detectAssignmentStatementBlock(
	s *ast.AssignStmt,
	fset *token.FileSet,
	npeLintErrorListPtr *[]*LintError) {
	for _, expr := range s.Rhs {
		switch EX := expr.(type) {
		case *ast.SelectorExpr:
			lintErrors := f.detectSelectorReferenceWithFunc(EX, fset)
			if len(lintErrors) > 0 {
				*npeLintErrorListPtr = append(*npeLintErrorListPtr, lintErrors...)
			}

		case *ast.CallExpr:
			if selectExpr, ok := EX.Fun.(*ast.SelectorExpr); ok {
				lintErrors := f.detectSelectorReferenceWithFunc(selectExpr, fset)
				if len(lintErrors) > 0 {
					*npeLintErrorListPtr = append(*npeLintErrorListPtr, lintErrors...)
				}
			}
		}
	}
}

func (f *FuncDelChecker) detectIfStatementBlock(s *ast.IfStmt, fset *token.FileSet, lintErrorListPtr *[]*LintError) {
	for _, stmt := range s.Body.List {
		switch expr := stmt.(type) {
		case *ast.ExprStmt:
			x := expr.X
			switch x := x.(type) {
			case *ast.CallExpr:
				for _, arg := range x.Args {
					switch EX := arg.(type) {
					case *ast.SelectorExpr:
						lintErrors := f.detectSelectorReferenceWithFunc(EX, fset)
						*lintErrorListPtr = append(*lintErrorListPtr, lintErrors...)
					}
				}
			}

		case *ast.IfStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.AssignStmt:
			f.detectNPEInStatement(expr, fset, lintErrorListPtr)
		}
	}
}

func (f *FuncDelChecker) detectSelectorReferenceWithFunc(
	EX *ast.SelectorExpr,
	fset *token.FileSet,
) []*LintError {
	var (
		nodeList      = make([]*SelectNode, 0)
		lintErrorList []*LintError
	)
	walkSelectorWithFunc(EX, fset, func(node ast.Node) {
		switch exprInner := node.(type) {
		case *ast.SelectorExpr:
			if exprInner != nil && exprInner.Sel != nil {
				nodeType, isReturnSingleFunc := GetNodeType(exprInner.Sel, f.pass.TypesInfo)
				nodeList = append(nodeList, &SelectNode{
					Name:               exprInner.Sel.Name,
					CurIdent:           exprInner.Sel,
					Type:               nodeType,
					IsReturnSingleFunc: isReturnSingleFunc})
			}

		case *ast.CallExpr:
			switch exFunc := exprInner.Fun.(type) {
			case *ast.SelectorExpr:
				if exFunc != nil && exFunc.Sel != nil {
					nodeType, isReturnSingleFunc := GetNodeType(exFunc.Sel, f.pass.TypesInfo)
					nodeList = append(nodeList, &SelectNode{
						Name:               exFunc.Sel.Name,
						CurIdent:           exFunc.Sel,
						Type:               nodeType,
						IsReturnSingleFunc: isReturnSingleFunc})
				}

			case *ast.Ident:
				nodeType, isReturnSingleFunc := GetNodeType(exFunc, f.pass.TypesInfo)
				nodeList = append(nodeList, &SelectNode{
					Name:               exFunc.Name,
					CurIdent:           exFunc,
					Type:               nodeType,
					IsReturnSingleFunc: isReturnSingleFunc})
			}

		case *ast.Ident:
			if exprInner != nil {
				nodeType, isReturnSingleFunc := GetNodeType(exprInner, f.pass.TypesInfo)
				nodeList = append(nodeList, &SelectNode{
					Name:               exprInner.Name,
					CurIdent:           exprInner,
					Type:               nodeType,
					IsReturnSingleFunc: isReturnSingleFunc})
				sort.SliceStable(nodeList, func(i, j int) bool {
					return i > j
				})

				var pos token.Position
				GetIdentPosition(&pos, exprInner, fset)
				fieldName := buildFieldNameFromNodes(nodeList)
				//_, funcIndex := f.isExistFuncRetPtr(nodeList, true)
				isRootFromOutside := f.isRootComeFromOutside(fieldName)
				lintErrorList = f.hasSequenceDetectNode(nodeList, pos, exprInner, isRootFromOutside)
			}
		}
	})
	return lintErrorList
}

func (f *FuncDelChecker) hasSequenceDetectNode(
	nodeList []*SelectNode,
	pos token.Position,
	expr *ast.Ident,
	// funcIndex int, //o.GetUserInfo().GetMembership() -- funcIndex: 1
	isRootFromOutside bool,
) []*LintError {
	var result = make([]*LintError, 0)
	if len(nodeList) <= 1 {
		return nil
	}

	for len(nodeList) > 0 {
		//if isRootFromOutside == false && funcIndex >= 0 && len(nodeList)-1 <= funcIndex {
		//	return result
		//}

		if len(nodeList) >= 2 && nodeList[len(nodeList)-2].Type == NodeTypeNonSinglePtr {
			nodeList = nodeList[0 : len(nodeList)-1]
			continue
		}

		fieldName := buildFieldNameFromNodes(nodeList)
		if len(nodeList) >= 2 && nodeList[len(nodeList)-2].CurIdent != nil {
			expr = nodeList[len(nodeList)-2].CurIdent
		}

		lintError := f.getPotentialNilPointerReference(fieldName, pos.Filename, &pos, expr, isRootFromOutside)
		if lintError != nil {
			result = append(result, lintError)
		}
		nodeList = nodeList[0 : len(nodeList)-1]
	}

	return result
}

func GetFuncSignature(ex *ast.CallExpr, typeInfo *types.Info) *types.Signature {
	if ex == nil {
		return nil
	}

	var (
		sig *types.Signature
		ok  bool
	)

	fn := ex.Fun
	switch fn := fn.(type) {
	case *ast.Ident:
		obj := typeInfo.ObjectOf(fn)
		sig, ok = obj.Type().(*types.Signature)
		if !ok {
			return nil
		}

	case *ast.SelectorExpr:
		obj := typeInfo.ObjectOf(fn.Sel)
		sig, ok = obj.Type().(*types.Signature)
		if !ok {
			return nil
		}
	}

	return sig
}
