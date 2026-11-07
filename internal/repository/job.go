package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"web-basic/internal/model/entity"
)

// ListJobs 定时任务列表。
func (r *Repo) ListJobs(ctx context.Context) ([]entity.Job, error) {
	var list []entity.Job
	err := r.DB.WithContext(ctx).Model(&entity.Job{}).Order("id ASC").Find(&list).Error
	return list, err
}

// FindJob 按 ID 查询任务。
func (r *Repo) FindJob(ctx context.Context, id uint64) (*entity.Job, error) {
	var job entity.Job
	if err := r.DB.WithContext(ctx).Where("id = ?", id).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

// SaveJob 保存任务（全量更新指定字段）。
func (r *Repo) SaveJob(ctx context.Context, job *entity.Job) error {
	return Result(r.DB.WithContext(ctx).Model(&entity.Job{}).Where("id = ?", job.ID).
		Select("name", "handler", "job_type", "target", "http_method", "cron", "params",
			"concurrent", "timeout_sec", "retry_times", "status", "next_run_at").
		Updates(job))
}

// InsertJob 新增任务。
func (r *Repo) InsertJob(ctx context.Context, job *entity.Job) error {
	return r.DB.WithContext(ctx).Create(job).Error
}

// UpdateJobStatus 启停任务。
func (r *Repo) UpdateJobStatus(ctx context.Context, id uint64, status int) error {
	return Result(r.DB.WithContext(ctx).Model(&entity.Job{}).Where("id = ?", id).Update("status", status))
}

// DeleteJob 删除任务。
func (r *Repo) DeleteJob(ctx context.Context, id uint64) error {
	return Result(r.DB.WithContext(ctx).Where("id = ?", id).Delete(&entity.Job{}))
}

// InsertJobLog 写入任务执行日志。
func (r *Repo) InsertJobLog(ctx context.Context, log *entity.JobLog) error {
	return r.DB.WithContext(ctx).Create(log).Error
}

// ListJobLogs 任务执行日志分页。
func (r *Repo) ListJobLogs(ctx context.Context, jobID uint64, page, pageSize int) ([]entity.JobLog, int64, error) {
	q := r.DB.WithContext(ctx).Model(&entity.JobLog{})
	if jobID > 0 {
		q = q.Where("job_id = ?", jobID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []entity.JobLog
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// TrimOldRecords 清理历史数据（返回删除行数）。
func (r *Repo) TrimOldRecords(ctx context.Context, table, timeCol string, before string) (int64, error) {
	res := r.DB.WithContext(ctx).Exec(
		"DELETE FROM "+table+" WHERE "+timeCol+" < ?", before)
	return res.RowsAffected, res.Error
}
