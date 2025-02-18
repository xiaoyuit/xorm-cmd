package {{.Models}}

{{$ilen := len .Imports}}
{{if gt $ilen 0}}
import (
	{{range .Imports}}"{{.}}"{{end}}
)
{{end}}

{{range .Tables}}
type {{Mapper .Name}} struct {
{{$table := .}}
{{range .ColumnsSeq}}{{$col := $table.GetColumn .}}	{{Mapper $col.Name}}	{{Type $col}} {{Tag $table $col}} {{Annotation $table $col}}
{{end}}
}

func (m *{{Mapper .Name}}) TableName() string {
	return "{{Uamel2Case (Mapper .Name)}}"
}

{{end}}

