// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"go/format"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/go-xorm/core"
)

var (
	supportCommentGorm bool
	GoLangTmplGorm     LangTmpl = LangTmpl{
		template.FuncMap{"Mapper": mapper.Table2Obj,
			"Type":       typestringGorm,
			"Tag":        tagGorm,
			"UnTitle":    unTitle,
			"gt":         gtGorm,
			"getCol":     getColGorm,
			"UpperTitle": upTitle,
			"Case2Camel": case2Camel,
			"Uamel2Case": uamel2Case,
		},
		formatGoGorm,
		genGoImportsGorm,
	}
)

var (
	errBadComparisonTypeGorm = errors.New("invalid type for comparison")
	errBadComparisonGorm     = errors.New("incompatible types for comparison")
	errNoComparisonGorm      = errors.New("missing argument for comparison")
)

type kindGorm int

const (
	invalidKindGorm kindGorm = iota
	boolKindGorm
	complexKindGorm
	intKindGorm
	floatKindGorm
	integerKindGorm
	stringKindGorm
	uintKindGorm
)

func basicKindGorm(v reflect.Value) (kindGorm, error) {
	switch v.Kind() {
	case reflect.Bool:
		return boolKindGorm, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intKindGorm, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintKindGorm, nil
	case reflect.Float32, reflect.Float64:
		return floatKindGorm, nil
	case reflect.Complex64, reflect.Complex128:
		return complexKindGorm, nil
	case reflect.String:
		return stringKindGorm, nil
	}
	return invalidKindGorm, errBadComparisonTypeGorm
}

// eq evaluates the comparison a == b || a == c || ...
func eqGorm(arg1 interface{}, arg2 ...interface{}) (bool, error) {
	v1 := reflect.ValueOf(arg1)
	k1, err := basicKindGorm(v1)
	if err != nil {
		return false, err
	}
	if len(arg2) == 0 {
		return false, errNoComparisonGorm
	}
	for _, arg := range arg2 {
		v2 := reflect.ValueOf(arg)
		k2, err := basicKindGorm(v2)
		if err != nil {
			return false, err
		}
		if k1 != k2 {
			return false, errBadComparisonGorm
		}
		truth := false
		switch k1 {
		case boolKindGorm:
			truth = v1.Bool() == v2.Bool()
		case complexKindGorm:
			truth = v1.Complex() == v2.Complex()
		case floatKindGorm:
			truth = v1.Float() == v2.Float()
		case intKindGorm:
			truth = v1.Int() == v2.Int()
		case stringKindGorm:
			truth = v1.String() == v2.String()
		case uintKindGorm:
			truth = v1.Uint() == v2.Uint()
		default:
			panic("invalid kind")
		}
		if truth {
			return true, nil
		}
	}
	return false, nil
}

// lt evaluates the comparison a < b.
func ltGorm(arg1, arg2 interface{}) (bool, error) {
	v1 := reflect.ValueOf(arg1)
	k1, err := basicKindGorm(v1)
	if err != nil {
		return false, err
	}
	v2 := reflect.ValueOf(arg2)
	k2, err := basicKindGorm(v2)
	if err != nil {
		return false, err
	}
	if k1 != k2 {
		return false, errBadComparisonGorm
	}
	truth := false
	switch k1 {
	case boolKindGorm, complexKindGorm:
		return false, errBadComparisonTypeGorm
	case floatKindGorm:
		truth = v1.Float() < v2.Float()
	case intKindGorm:
		truth = v1.Int() < v2.Int()
	case stringKindGorm:
		truth = v1.String() < v2.String()
	case uintKindGorm:
		truth = v1.Uint() < v2.Uint()
	default:
		panic("invalid kind")
	}
	return truth, nil
}

// le evaluates the comparison <= b.
func leGorm(arg1, arg2 interface{}) (bool, error) {
	// <= is < or ==.
	lessThan, err := ltGorm(arg1, arg2)
	if lessThan || err != nil {
		return lessThan, err
	}
	return eqGorm(arg1, arg2)
}

// gt evaluates the comparison a > b.
func gtGorm(arg1, arg2 interface{}) (bool, error) {
	// > is the inverse of <=.
	lessOrEqual, err := leGorm(arg1, arg2)
	if err != nil {
		return false, err
	}
	return !lessOrEqual, nil
}

func getColGorm(cols map[string]*core.Column, name string) *core.Column {
	return cols[strings.ToLower(name)]
}

func formatGoGorm(src string) (string, error) {
	source, err := format.Source([]byte(src))
	if err != nil {
		return "", err
	}
	return string(source), nil
}

func genGoImportsGorm(tables []*core.Table) map[string]string {
	imports := make(map[string]string)

	for _, table := range tables {
		for _, col := range table.Columns() {
			if typestringGorm(col) == "time.Time" {
				imports["time"] = "time"
			}
		}
	}
	return imports
}

func typestringGorm(col *core.Column) string {
	st := col.SQLType
	t := core.SQLType2Type(st)
	s := t.String()
	if s == "[]uint8" {
		return "[]byte"
	}
	return s
}

func tagGorm(table *core.Table, col *core.Column) string {
	isNameId := (mapper.Table2Obj(col.Name) == "Id")
	isIdPk := isNameId && typestringGorm(col) == "int64"

	var res []string
	if !col.Nullable {
		if !isIdPk {
			res = append(res, "not null")
		}
	}
	if col.IsPrimaryKey {
		res = append(res, "pk")
	}
	if col.Default != "" {
		res = append(res, "default "+col.Default)
	}
	if col.IsAutoIncrement {
		res = append(res, "autoincr")
	}

	if col.SQLType.IsTime() && includeGorm(created, col.Name) {
		res = append(res, "created")
	}

	if col.SQLType.IsTime() && includeGorm(updated, col.Name) {
		res = append(res, "updated")
	}

	if col.SQLType.IsTime() && includeGorm(deleted, col.Name) {
		res = append(res, "deleted")
	}

	if supportCommentGorm && col.Comment != "" {
		res = append(res, fmt.Sprintf("comment('%s')", col.Comment))
	}

	names := make([]string, 0, len(col.Indexes))
	for name := range col.Indexes {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		index := table.Indexes[name]
		var uistr string
		if index.Type == core.UniqueType {
			uistr = "unique"
		} else if index.Type == core.IndexType {
			uistr = "index"
		}
		if len(index.Cols) > 1 {
			uistr += "(" + index.Name + ")"
		}
		res = append(res, uistr)
	}

	nstr := col.SQLType.Name
	if col.Length != 0 {
		if col.Length2 != 0 {
			nstr += fmt.Sprintf("(%v,%v)", col.Length, col.Length2)
		} else {
			nstr += fmt.Sprintf("(%v)", col.Length)
		}
	} else if len(col.EnumOptions) > 0 { //enum
		nstr += "("
		opts := ""

		enumOptions := make([]string, 0, len(col.EnumOptions))
		for enumOption := range col.EnumOptions {
			enumOptions = append(enumOptions, enumOption)
		}
		sort.Strings(enumOptions)

		for _, v := range enumOptions {
			opts += fmt.Sprintf(",'%v'", v)
		}
		nstr += strings.TrimLeft(opts, ",")
		nstr += ")"
	} else if len(col.SetOptions) > 0 { //enum
		nstr += "("
		opts := ""

		setOptions := make([]string, 0, len(col.SetOptions))
		for setOption := range col.SetOptions {
			setOptions = append(setOptions, setOption)
		}
		sort.Strings(setOptions)

		for _, v := range setOptions {
			opts += fmt.Sprintf(",'%v'", v)
		}
		nstr += strings.TrimLeft(opts, ",")
		nstr += ")"
	}
	res = append(res, nstr)

	var tags []string
	if len(res) > 0 {
		tags = append(tags, "gorm:\"column:"+col.Name+";"+strings.Join(res, " ")+"\"")
	}
	if genJson {
		if includeGorm(ignoreColumnsJSON, col.Name) {
			tags = append(tags, "json:\"-\"")
		} else {
			tags = append(tags, "json:\""+col.Name+"\"")
		}
	}
	if len(tags) > 0 {
		return "`" + strings.Join(tags, " ") + "`"
	} else {
		return ""
	}
}

func includeGorm(source []string, target string) bool {
	for _, s := range source {
		if s == target {
			return true
		}
	}
	return false
}
