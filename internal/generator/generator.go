// Package generator 简易代码生成器：读 information_schema 生成 CRUD 样板代码。
//
// 输出物（一次生成一张表的全套骨架）：
//
//	entity    internal/model/entity/<name>.go      实体（自动继承 Base/BaseLite）
//	service   internal/service/<name>_gen.go       List/Detail/Create/Update/Delete
//	api       internal/api/<name>_gen.go           Gin handler
//	router    internal/router/<name>_gen.go.txt    路由注册片段（人工粘贴到 router.go）
//	页面      web/src/views/<module>/<name>/index.vue  ArtTable + 表单弹窗
//	菜单 SQL   <name>_menu.sql                     菜单/按钮/授权 INSERT
//
// 生成的是"可直接编译运行的骨架"，业务逻辑仍需人工补；命名规则见 sanitize/naming 注释。
package generator

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"unicode"
)

// ColumnMeta information_schema.columns 精简视图。
type ColumnMeta struct {
	Name       string `json:"name"`
	DataType   string `json:"dataType"`
	ColumnType string `json:"columnType"`
	Comment    string `json:"comment"`
	Nullable   bool   `json:"nullable"`
	IsPrimary  bool   `json:"isPrimary"`
	AutoIncr   bool   `json:"autoIncr"`
	MaxLen     int    `json:"maxLen"`
}

// TableMeta 表元信息。
type TableMeta struct {
	Name      string       `json:"name"`
	Comment   string       `json:"comment"`
	RowCount  int64        `json:"rowCount"`
	CreateAt  string       `json:"createAt"`
	Columns   []ColumnMeta `json:"columns"`
	Module    string       `json:"module"`    // 路由模块名（表名第一段）
	Entity    string       `json:"entity"`    // Go 结构体名
	Var       string       `json:"var"`       // Go 变量名（首字母小写）
	Perm      string       `json:"perm"`      // 权限码前缀
	RoutePath string       `json:"routePath"` // 接口路径
	ViewPath  string       `json:"viewPath"`  // 前端页面路径
}

// 系统表 / 框架表不参与生成（避免把 sys_menu 之类重新生成一遍）。
var skipTables = map[string]bool{
	"schema_migrations": true,
}

// LoadTables 读取库内所有表（含注释与行数）。
func LoadTables(ctx context.Context, db *sql.DB, schema string) ([]TableMeta, error) {
	rows, err := db.QueryContext(ctx, `
SELECT t.TABLE_NAME, IFNULL(t.TABLE_COMMENT,''), IFNULL(t.TABLE_ROWS,0), IFNULL(DATE_FORMAT(t.CREATE_TIME,'%Y-%m-%d %H:%i:%s'),'')
FROM information_schema.TABLES t
WHERE t.TABLE_SCHEMA = ? AND t.TABLE_TYPE = 'BASE TABLE'
ORDER BY t.TABLE_NAME`, schema)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]TableMeta, 0, 32)
	for rows.Next() {
		var m TableMeta
		if err := rows.Scan(&m.Name, &m.Comment, &m.RowCount, &m.CreateAt); err != nil {
			return nil, err
		}
		if skipTables[m.Name] {
			continue
		}
		m.fill()
		out = append(out, m)
	}
	return out, rows.Err()
}

// LoadTable 读取单表元信息（含字段）。
func LoadTable(ctx context.Context, db *sql.DB, schema, table string) (*TableMeta, error) {
	var m TableMeta
	err := db.QueryRowContext(ctx, `
SELECT t.TABLE_NAME, IFNULL(t.TABLE_COMMENT,''), IFNULL(t.TABLE_ROWS,0), IFNULL(DATE_FORMAT(t.CREATE_TIME,'%Y-%m-%d %H:%i:%s'),'')
FROM information_schema.TABLES t
WHERE t.TABLE_SCHEMA = ? AND t.TABLE_NAME = ?`, schema, table).
		Scan(&m.Name, &m.Comment, &m.RowCount, &m.CreateAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cols, err := loadColumns(ctx, db, schema, table)
	if err != nil {
		return nil, err
	}
	m.Columns = cols
	m.fill()
	return &m, nil
}

func loadColumns(ctx context.Context, db *sql.DB, schema, table string) ([]ColumnMeta, error) {
	rows, err := db.QueryContext(ctx, `
SELECT COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, IFNULL(COLUMN_COMMENT,''), IS_NULLABLE, COLUMN_KEY, EXTRA,
       IFNULL(CHARACTER_MAXIMUM_LENGTH, 0)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY ORDINAL_POSITION`, schema, table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]ColumnMeta, 0, 24)
	for rows.Next() {
		var c ColumnMeta
		var nullable, key, extra string
		if err := rows.Scan(&c.Name, &c.DataType, &c.ColumnType, &c.Comment, &nullable, &key, &extra, &c.MaxLen); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		c.IsPrimary = key == "PRI"
		c.AutoIncr = strings.Contains(extra, "auto_increment")
		out = append(out, c)
	}
	return out, rows.Err()
}

// fill 计算命名派生字段。
func (m *TableMeta) fill() {
	base := stripPrefix(m.Name)
	m.Entity = toCamel(base, true)
	m.Var = toCamel(base, false)
	m.Perm = strings.ReplaceAll(base, "_", "-")
	parts := strings.SplitN(base, "_", 2)
	m.Module = parts[0]
	m.RoutePath = "/" + strings.ReplaceAll(base, "_", "-")
	if len(parts) == 1 {
		// 单段表名（如 demo_goods → goods）：页面放 views/<module>/index.vue，避免 goods/goods 这种重复路径
		m.ViewPath = m.Module + "/index"
	} else {
		m.ViewPath = fmt.Sprintf("%s/%s/index", m.Module, strings.ReplaceAll(parts[1], "_", "-"))
	}
}

// 常见表前缀，生成时去掉（sys_menu → Menu）。
var prefixes = []string{"sys_", "biz_", "t_", "tb_", "app_", "demo_"}

func stripPrefix(name string) string {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) && len(name) > len(p) {
			return strings.TrimPrefix(name, p)
		}
	}
	return name
}

// toCamel snake_case → CamelCase / camelCase。
func toCamel(s string, upperFirst bool) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	var b strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		if i == 0 && !upperFirst {
			b.WriteString(string(unicode.ToLower(r[0])))
		} else {
			b.WriteString(string(unicode.ToUpper(r[0])))
		}
		b.WriteString(string(r[1:]))
	}
	return b.String()
}

// baseColumns 由 Base/BaseLite/软删除 提供的公共列，实体里不再重复声明。
var baseColumns = map[string]bool{
	"id": true, "create_by": true, "create_time": true, "update_by": true,
	"update_time": true, "deleted": true, "remark": true,
}

// 时间/软删除基础列集合（用于选择 Base 还是 BaseLite）
var baseLiteColumns = map[string]bool{"id": true, "create_by": true, "create_time": true, "update_by": true, "update_time": true, "remark": true}

// GoType 列类型 → Go 类型。
func GoType(c ColumnMeta) string {
	switch strings.ToLower(c.DataType) {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint", "year":
		return "int64"
	case "decimal", "numeric", "float", "double", "real":
		return "float64"
	case "date", "datetime", "timestamp", "time":
		return "time.Time"
	case "json":
		return "string"
	case "blob", "longblob", "mediumblob", "tinyblob", "binary", "varbinary":
		return "[]byte"
	default:
		return "string"
	}
}

// GenField 生成用的字段描述。
type GenField struct {
	Name   string // 数据库列名
	GoName string // Go 字段名
	JSON   string // JSON 字段名
	Type   string // Go 类型
	Tag    string // 结构体标签（gorm+json）
	Label  string // 中文标签（注释/表头）
	Query  bool   // 是否作为列表筛选条件
	Form   bool   // 是否出现在表单
	Input  string // 表单控件类型：input/number/textarea/select/date
}

// Fields 计算生成字段列表（跳过 Base 列）。
func (m *TableMeta) Fields() []GenField {
	out := make([]GenField, 0, len(m.Columns))
	for _, c := range m.Columns {
		if c.IsPrimary {
			continue
		}
		if baseColumns[c.Name] {
			continue
		}
		label := c.Comment
		if label == "" {
			label = c.Name
		}
		goType := GoType(c)
		tag := fmt.Sprintf("`gorm:\"column:%s\" json:\"%s\"`", c.Name, toCamel(c.Name, false))
		// bigint 主键类型（雪花 ID）以字符串输出，避免前端精度丢失
		jsonName := toCamel(c.Name, false)
		if goType == "int64" && strings.HasPrefix(strings.ToLower(c.ColumnType), "bigint") && !c.AutoIncr {
			tag = fmt.Sprintf("`gorm:\"column:%s\" json:\"%s,string\"`", c.Name, jsonName)
		}
		input := "input"
		switch {
		case goType == "int64" || goType == "float64":
			input = "number"
		case goType == "time.Time":
			input = "date"
		case strings.Contains(strings.ToLower(c.ColumnType), "text"):
			input = "textarea"
		case strings.Contains(strings.ToLower(c.ColumnType), "tinyint(1)"):
			input = "select"
		}
		// 作为列表筛选条件：字符串（非大文本）全部开放；数值只对 tinyint（状态/类型这类枚举）开放，
		// 否则 price/stock 这种精确匹配筛选对使用者毫无意义。
		lowerType := strings.ToLower(c.ColumnType)
		query := false
		if goType == "string" && input == "input" {
			query = true
		} else if strings.HasPrefix(lowerType, "tinyint") {
			query = true
		}
		out = append(out, GenField{
			Name: c.Name, GoName: toCamel(c.Name, true), JSON: jsonName, Type: goType,
			Tag: tag, Label: label, Query: query, Form: true, Input: input,
		})
	}
	return out
}

// UsesBaseLite 判断实体是否可用 BaseLite（无 deleted/remark 列时）。
func (m *TableMeta) UsesBaseLite() bool {
	has := map[string]bool{}
	for _, c := range m.Columns {
		has[c.Name] = true
	}
	for col := range has {
		if !baseLiteColumns[col] {
			return false
		}
	}
	return true
}

// TimeImport 生成文件是否需要 "time" 包。
func needsTime(fields []GenField) bool {
	for _, f := range fields {
		if f.Type == "time.Time" {
			return true
		}
	}
	return false
}

// Render 用模板渲染单份文件。
func Render(tpl string, data any) (string, error) {
	t, err := template.New("t").Funcs(template.FuncMap{
		"lower": strings.ToLower,
		"title": func(s string) string { return toCamel(s, true) },
	}).Parse(tpl)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// itoa 小工具（模板里不便直接 strconv）。
func itoa(n int) string { return strconv.Itoa(n) }
