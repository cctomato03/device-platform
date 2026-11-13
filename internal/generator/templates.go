package generator

import (
	"fmt"
	"strings"
)

// File 生成结果文件。
type File struct {
	Path    string `json:"path"`    // 建议落盘路径
	Content string `json:"content"` // 文件内容
	Lang    string `json:"lang"`    // 高亮用：go / vue / sql / txt
}

type tplData struct {
	Table    *TableMeta
	Fields   []GenField
	NeedTime bool
}

const entityTpl = `package entity

{{if .NeedTime}}import "time"

{{end}}// {{.Table.Entity}}{{if .Table.Comment}} {{.Table.Comment}}{{end}}（表 {{.Table.Name}}）
type {{.Table.Entity}} struct {
	Base
{{- range .Fields}}
	{{.GoName}} {{.Type}} {{.Tag}}
{{- end}}
}

func ({{.Table.Entity}}) TableName() string { return "{{.Table.Name}}" }
`

const serviceTpl = `package service

// 本文件由代码生成器生成，业务逻辑请按需补充。

import (
	"context"
	"time"

{{if .NeedTime}}	"errors"
{{end}}	"device-platform/internal/model/entity"
	"device-platform/internal/model/scope"
	"device-platform/internal/principal"
	"device-platform/internal/repository"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
)

// {{.Table.Entity}}Query {{.Table.Comment}}列表查询。
type {{.Table.Entity}}Query struct {
	entity.PageQuery
{{- range .Fields}}
{{- if .Query}}
	{{.GoName}} {{if eq .Type "string"}}string{{else}}{{.Type}}{{end}} ` + "`form:\"{{.JSON}}\"`" + `
{{- end}}
{{- end}}
}

// {{.Table.Entity}}Req {{.Table.Comment}}新增/修改入参。
type {{.Table.Entity}}Req struct {
{{- range .Fields}}
{{- if .Form}}
	{{.GoName}} {{.Type}} ` + "`json:\"{{.JSON}}\"`" + `
{{- end}}
{{- end}}
}

// List{{.Table.Entity}} 分页查询{{.Table.Comment}}。
func (s *Service) List{{.Table.Entity}}(ctx context.Context, q *{{.Table.Entity}}Query, p *principal.Info) ([]entity.{{.Table.Entity}}, int64, error) {
	q.Normalize()
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.{{.Table.Entity}}{}).Scopes(scope.Tenant(p.TenantID))
{{- range .Fields}}
{{- if .Query}}
{{- if eq .Type "string"}}
	if q.{{.GoName}} != "" {
		tx = tx.Where("{{.Name}} LIKE ?", q.{{.GoName}}+"%")
	}
{{- else if eq .Type "time.Time"}}
	// {{.Label}} 时间区间筛选
{{- else}}
	if q.{{.GoName}} != 0 {
		tx = tx.Where("{{.Name}} = ?", q.{{.GoName}})
	}
{{- end}}
{{- end}}
{{- end}}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.{{.Table.Entity}}, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// Create{{.Table.Entity}} 新增{{.Table.Comment}}。
func (s *Service) Create{{.Table.Entity}}(ctx context.Context, req *{{.Table.Entity}}Req, p *principal.Info) (uint64, error) {
	now := time.Now()
	row := &entity.{{.Table.Entity}}{}
	row.ID = id.Next()
	row.CreateBy = p.UserID
	row.CreateTime = now
	row.UpdateBy = p.UserID
	row.UpdateTime = &now
{{- range .Fields}}
{{- if .Form}}
	row.{{.GoName}} = req.{{.GoName}}
{{- end}}
{{- end}}
	if err := s.Repo.DB.WithContext(ctx).Create(row).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return row.ID, nil
}

// Update{{.Table.Entity}} 修改{{.Table.Comment}}（带字段级审计差异）。
func (s *Service) Update{{.Table.Entity}}(ctx context.Context, rowID uint64, req *{{.Table.Entity}}Req, p *principal.Info) error {
	fields := map[string]any{
{{- range .Fields}}
{{- if .Form}}
		"{{.Name}}": req.{{.GoName}},
{{- end}}
{{- end}}
		"update_by":   p.UserID,
		"update_time": time.Now(),
	}
	if err := repository.UpdateByID(ctx, s.Repo.DB, &entity.{{.Table.Entity}}{}, rowID, fields); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// Delete{{.Table.Entity}} 删除{{.Table.Comment}}。
func (s *Service) Delete{{.Table.Entity}}(ctx context.Context, rowID uint64, p *principal.Info) error {
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Delete(&entity.{{.Table.Entity}}{}, rowID)); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}
`

const apiTpl = `package api

// 本文件由代码生成器生成，业务逻辑请按需补充。

import (
	"github.com/gin-gonic/gin"

	"device-platform/internal/principal"
	"device-platform/internal/service"
	"device-platform/pkg/response"
)

// List{{.Table.Entity}} {{.Table.Comment}}列表。
func (h *Handler) List{{.Table.Entity}}(c *gin.Context) {
	var q service.{{.Table.Entity}}Query
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.List{{.Table.Entity}}(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// Create{{.Table.Entity}} 新增{{.Table.Comment}}。
func (h *Handler) Create{{.Table.Entity}}(c *gin.Context) {
	var req service.{{.Table.Entity}}Req
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.Create{{.Table.Entity}}(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// Update{{.Table.Entity}} 修改{{.Table.Comment}}。
func (h *Handler) Update{{.Table.Entity}}(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.{{.Table.Entity}}Req
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.Update{{.Table.Entity}}(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// Delete{{.Table.Entity}} 删除{{.Table.Comment}}。
func (h *Handler) Delete{{.Table.Entity}}(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.Delete{{.Table.Entity}}(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}
`

const routerTpl = `		// {{.Table.Comment}}（代码生成器生成，粘贴到 internal/router/router.go 的 authed 分组内）
		g := authed.Group("{{.Table.RoutePath}}")
		g.GET("", middleware.Perm("{{.Table.Perm}}:list"), h.List{{.Table.Entity}})
		g.POST("", middleware.Perm("{{.Table.Perm}}:create"), h.Create{{.Table.Entity}})
		g.PUT("/:id", middleware.Perm("{{.Table.Perm}}:update"), h.Update{{.Table.Entity}})
		g.DELETE("/:id", middleware.Perm("{{.Table.Perm}}:delete"), h.Delete{{.Table.Entity}})
`

const menuTpl = `-- {{.Table.Comment}}菜单（代码生成器生成）
-- 注意：菜单 ID 段请按项目实际占用情况调整，避免与现有菜单冲突。
INSERT IGNORE INTO sys_menu
  (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  ({{.MenuID}}, 0, '{{.Table.Comment}}', 2, '{{.Table.Perm}}:list', '{{.Table.Perm}}', '{{.Table.ViewPath}}', 'ep:document', 10, 1, 1, 1, 1, NOW(), '代码生成器生成'),
  ({{.MenuID}}1, {{.MenuID}}, '新增', 3, '{{.Table.Perm}}:create', '', '', '', 1, 1, 1, 1, 1, NOW(), ''),
  ({{.MenuID}}2, {{.MenuID}}, '修改', 3, '{{.Table.Perm}}:update', '', '', '', 2, 1, 1, 1, 1, NOW(), ''),
  ({{.MenuID}}3, {{.MenuID}}, '删除', 3, '{{.Table.Perm}}:delete', '', '', '', 3, 1, 1, 1, 1, NOW(), '');

-- 授权给超管角色（role_id = 1）
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
VALUES (1, {{.MenuID}}), (1, {{.MenuID}}1), (1, {{.MenuID}}2), (1, {{.MenuID}}3);
`

const vueTpl = `<!-- {{.Table.Comment}}（代码生成器生成） -->
<template>
  <div class="gen-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'{{.Table.Perm}}:create'" type="primary" @click="openDialog(null)" v-ripple>
              新增
            </ElButton>
          </ElSpace>
        </template>
      </ArtTableHeader>

      <ArtTable
        :loading="loading"
        :columns="columns"
        :data="data"
        :pagination="pagination"
        @pagination:size-change="handleSizeChange"
        @pagination:current-change="handleCurrentChange"
      />
    </ElCard>

    <ElDialog v-model="dialogVisible" :title="editingId ? '编辑' : '新增'" width="560px" align-center destroy-on-close>
      <ElForm ref="formRef" :model="form" label-width="100px">
{{- range .Fields}}
{{- if eq .Input "textarea"}}
        <ElFormItem label="{{.Label}}">
          <ElInput v-model="form.{{.JSON}}" type="textarea" :rows="3" />
        </ElFormItem>
{{- else if eq .Input "number"}}
        <ElFormItem label="{{.Label}}">
          <ElInputNumber v-model="form.{{.JSON}}" :controls="false" />
        </ElFormItem>
{{- else if eq .Input "date"}}
        <ElFormItem label="{{.Label}}">
          <ElDatePicker v-model="form.{{.JSON}}" type="datetime" value-format="YYYY-MM-DD HH:mm:ss" />
        </ElFormItem>
{{- else}}
        <ElFormItem label="{{.Label}}">
          <ElInput v-model="form.{{.JSON}}" />
        </ElFormItem>
{{- end}}
{{- end}}
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTable } from '@/hooks/core/useTable'
  import request from '@/utils/http'
  import { ElMessage, ElMessageBox } from 'element-plus'

  defineOptions({ name: '{{.Table.Entity}}' })

  interface Row {
    id: string
{{- range .Fields}}
    {{.JSON}}?: any
{{- end}}
  }

  const dialogVisible = ref(false)
  const submitting = ref(false)
  const editingId = ref<string | null>(null)
  const form = reactive<Record<string, any>>({})

  const fetchList = async (params: Record<string, any>) => {
    const res = await request.get<{ list: Row[]; total: number; page: number; pageSize: number }>({
      url: '/api/v1{{.Table.RoutePath}}',
      params: { page: params.current, pageSize: params.size, ...params }
    })
    return { records: res.list || [], current: res.page, size: res.pageSize, total: res.total }
  }

  const { columns, columnChecks, data, loading, pagination, handleSizeChange, handleCurrentChange, refreshData } =
    useTable({
      core: {
        apiFn: fetchList,
        apiParams: { current: 1, size: 20 },
        columnsFactory: () => [
{{- range .Fields}}
{{- if .Query}}
          { prop: '{{.JSON}}', label: '{{.Label}}', minWidth: 140, showOverflowTooltip: true },
{{- end}}
{{- end}}
          {
            prop: 'operation',
            label: '操作',
            width: 80,
            fixed: 'right',
            formatter: (row: Row) =>
              h(ArtButtonMore, {
                list: [
                  { key: 'edit', label: '编辑', icon: 'ri:edit-2-line', auth: '{{.Table.Perm}}:update' },
                  { key: 'delete', label: '删除', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: '{{.Table.Perm}}:delete' }
                ],
                onClick: (item: ButtonMoreItem) =>
                  item.key === 'edit' ? openDialog(row) : handleDelete(row)
              })
          }
        ]
      }
    })

  const openDialog = (row: Row | null) => {
    editingId.value = row?.id ?? null
    Object.keys(form).forEach((k) => delete form[k])
{{- range .Fields}}
    form.{{.JSON}} = row?.{{.JSON}} ?? undefined
{{- end}}
    dialogVisible.value = true
  }

  const handleSubmit = async () => {
    submitting.value = true
    try {
      if (editingId.value) {
        await request.put({ url: ` + "`/api/v1{{.Table.RoutePath}}/${editingId.value}`" + `, data: form })
      } else {
        await request.post({ url: '/api/v1{{.Table.RoutePath}}', data: form })
      }
      ElMessage.success('保存成功')
      dialogVisible.value = false
      refreshData()
    } finally {
      submitting.value = false
    }
  }

  const handleDelete = async (row: Row) => {
    try {
      await ElMessageBox.confirm('确定删除该记录吗？', '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await request.del({ url: ` + "`/api/v1{{.Table.RoutePath}}/${row.id}`" + `, showSuccessMessage: true })
    refreshData()
  }
</script>
`

// MenuID 生成菜单用的起始 ID（按表名散列，避免重复；实际使用时请人工确认）。
func menuID(name string) string {
	sum := 0
	for _, r := range name {
		sum = (sum*31 + int(r)) % 900
	}
	return fmt.Sprintf("3%03d", sum)
}

// Generate 生成一张表的全套文件。
func Generate(meta *TableMeta) ([]File, error) {
	fields := meta.Fields()
	data := tplData{Table: meta, Fields: fields, NeedTime: needsTime(fields)}

	entity, err := Render(entityTpl, data)
	if err != nil {
		return nil, err
	}
	svc, err := Render(serviceTpl, data)
	if err != nil {
		return nil, err
	}
	apiCode, err := Render(apiTpl, data)
	if err != nil {
		return nil, err
	}
	router, err := Render(routerTpl, data)
	if err != nil {
		return nil, err
	}
	vue, err := Render(vueTpl, data)
	if err != nil {
		return nil, err
	}
	menu, err := Render(menuTpl, struct {
		*tplData
		MenuID string
	}{&data, menuID(meta.Name)})
	if err != nil {
		return nil, err
	}

	base := strings.ReplaceAll(meta.Entity, " ", "")
	return []File{
		{Path: fmt.Sprintf("internal/model/entity/%s.go", strings.ToLower(base)), Content: entity, Lang: "go"},
		{Path: fmt.Sprintf("internal/service/%s_gen.go", strings.ToLower(base)), Content: svc, Lang: "go"},
		{Path: fmt.Sprintf("internal/api/%s_gen.go", strings.ToLower(base)), Content: apiCode, Lang: "go"},
		{Path: fmt.Sprintf("docs/gen/%s_router.go.txt", strings.ToLower(base)), Content: router, Lang: "txt"},
		{Path: fmt.Sprintf("web/src/views/%s.vue", meta.ViewPath), Content: vue, Lang: "vue"},
		{Path: fmt.Sprintf("docs/gen/%s_menu.sql", strings.ToLower(base)), Content: menu, Lang: "sql"},
	}, nil
}
