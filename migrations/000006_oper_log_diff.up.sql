-- 操作日志：字段级变更差异（JSON 数组：[{"field":"status","before":"1","after":"0"}]）
-- 由 service 层通过 audit 收集器写入，前端「操作日志 → 详情」以表格展示"修改前 / 修改后"。
ALTER TABLE sys_oper_log ADD COLUMN diff TEXT NULL;
