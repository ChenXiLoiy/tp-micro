package create

import (
	"bytes"
	"fmt"
	"go/ast"
	"strings"
	"text/template"

	"github.com/henrylee2cn/goutil"
	tp "github.com/henrylee2cn/teleport"
)

func (mod *Model) createMgoModel(t *TypeStruct) {
	st, ok := t.expr.(*ast.StructType)
	if !ok {
		tp.Fatalf("[micro] the type of model must be struct: %s", t.Name)
	}

	mod.NameSql = fmt.Sprintf("`%s`", mod.SnakeName)
	mod.QuerySql = [2]string{}
	mod.UpdateSql = ""
	mod.UpsertSqlSuffix = ""

	var (
		fields                            []string
		querySql1, querySql2              string
		hasId, hasCreatedAt, hasUpdatedAt bool
	)
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			tp.Fatalf("[micro] the type of model can't have anonymous field")
		}
		name := field.Names[0].Name
		if !goutil.IsExportedName(name) {
			continue
		}
		name = goutil.SnakeString(name)
		switch name {
		case "id":
			hasId = true
		case "created_at":
			hasCreatedAt = true
		case "updated_at":
			hasUpdatedAt = true
		}
		fields = append(fields, name)
	}

	if !hasId {
		t.appendMgoHeadField(`Id int64`)
		fields = append([]string{"id"}, fields...)
	}
	if !hasCreatedAt {
		t.appendMgoTailField(`CreatedAt int64`)
		fields = append(fields, "created_at")
	}
	if !hasUpdatedAt {
		t.appendMgoTailField(`UpdatedAt int64`)
		fields = append(fields, "updated_at")
	}
	mod.StructDefinition = fmt.Sprintf("\n%stype %s %s\n", t.Doc, t.Name, addTag(t.Body))

	for _, field := range fields {
		if field == "id" {
			continue
		}
		querySql1 += fmt.Sprintf("`%s`,", field)
		querySql2 += fmt.Sprintf(":%s,", field)
		if field == "created_at" {
			continue
		}
		mod.UpdateSql += fmt.Sprintf("`%s`=:%s,", field, field)
		mod.UpsertSqlSuffix += fmt.Sprintf("`%s`=VALUES(`%s`),", field, field)
	}
	mod.QuerySql = [2]string{querySql1[:len(querySql1)-1], querySql2[:len(querySql2)-1]}
	mod.UpdateSql = mod.UpdateSql[:len(mod.UpdateSql)-1]
	mod.UpsertSqlSuffix = mod.UpsertSqlSuffix[:len(mod.UpsertSqlSuffix)-1] + ";"

	m, err := template.New("").Parse(mgoModelTpl)
	if err != nil {
		panic(err)
	}
	buf := bytes.NewBuffer(nil)
	err = m.Execute(buf, mod)
	if err != nil {
		panic(err)
	}
	mod.code = buf.String()
}

func (t *TypeStruct) appendMgoHeadField(fieldLine string) {
	idx := strings.Index(t.Body, "{") + 1
	t.Body = t.Body[:idx] + "\n" + fieldLine + "\n" + t.Body[idx:]
}

func (t *TypeStruct) appendMgoTailField(fieldLine string) {
	idx := strings.LastIndex(t.Body, "}")
	t.Body = t.Body[:idx] + "\n" + fieldLine + "\n" + t.Body[idx:]
}

const mgoModelTpl = `package mgo_model

import (
	"time"

	"github.com/xiaoenai/tp-micro/model/mongo"
)

{{.StructDefinition}}

// TableName implements 'github.com/xiaoenai/tp-micro/model'.Cacheable
func (*{{.Name}}) TableName() string {
	return "{{.SnakeName}}"
}

var {{.LowerFirstName}}DB, _ = dbHandler.RegCacheableDB(new({{.Name}}), time.Hour*24)

// Get{{.Name}}DB returns the {{.Name}} DB handler.
func Get{{.Name}}DB() *mongo.CacheableDB {
	return {{.LowerFirstName}}DB
}

// Upsert{{.Name}} insert or update the {{.Name}} data by selector and updater.
// NOTE:
//  With cache layer;
//  Insert data if the primary key is specified;
//  Update data based on _updateFields if no primary key is specified;
func Upsert{{.Name}}(selector mongo.M, updater mongo.M) error {
	return {{.LowerFirstName}}DB.WitchCollection(func(col *mongo.Collection) error {
		_, err := col.Upsert(selector, updater)
		return err
	})
}

// Get{{.Name}}ByWhere query a {{.Name}} data from database by WHERE condition.
// NOTE:
//  Without cache layer;
//  If @return error!=nil, means the database error.
func Get{{.Name}}ByWhere(query mongo.M) (*{{.Name}}, bool, error) {
	var _{{.LowerFirstLetter}} = new({{.Name}})
	err := {{.LowerFirstName}}DB.WitchCollection(func(col *mongo.Collection) error {
		return col.Find(query).One(&_{{.LowerFirstLetter}})
	})
	switch err {
	case nil:
		return _{{.LowerFirstLetter}}, true, nil
	case mongo.ErrNotFound:
		return nil, false, nil
	default:
		return nil, false, err
	}
}`
