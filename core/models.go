package core

import (
	"time"

	"github.com/google/uuid"
)

// Job contains all necessary information required to execute a specific job
type Job struct {
	// the unique job id
	ID string `json:"id"`
	// when the job should start
	ScheduledAt time.Time `json:"scheduledAt"`
	// the exact command to run
	Command string `json:"command"`
	// metadata
	Meta any `json:"meta"`
	// cluster for running job
	Cluster string `json:"cluster"`
}

// NewJob creates a new scheduled job to run
func NewJob(command string, scheduledAt time.Time, meta any, cluster string) *Job {
	return &Job{ID: uuid.New().String(), ScheduledAt: scheduledAt, Command: command, Meta: meta, Cluster: cluster}
}
