package immich

import "context"

// JobsService provides server job control operations.
type JobsService interface {
	// GetJobs returns the status of all jobs.
	GetJobs(ctx context.Context) (map[string]Job, error)

	// SendJobCommand sends a command to a server job.
	SendJobCommand(ctx context.Context, jobID string, command JobCommand, force bool) (JobCommandResponse, error)

	// CreateJob creates a new job.
	CreateJob(ctx context.Context, name JobName) error
}

// Job represents the status of a server job.
type Job struct {
	JobCounts struct {
		Active    int `json:"active"`
		Completed int `json:"completed"`
		Failed    int `json:"failed"`
		Delayed   int `json:"delayed"`
		Waiting   int `json:"waiting"`
		Paused    int `json:"paused"`
	} `json:"jobCounts"`
	QueueStatus struct {
		IsActive bool `json:"isActive"`
		IsPaused bool `json:"isPaused"`
	} `json:"queueStatus"`
}

// JobCommandResponse represents the response after sending a job command.
type JobCommandResponse struct {
	JobCounts struct {
		Active    int `json:"active"`
		Completed int `json:"completed"`
		Delayed   int `json:"delayed"`
		Failed    int `json:"failed"`
		Paused    int `json:"paused"`
		Waiting   int `json:"waiting"`
	} `json:"jobCounts"`
	QueueStatus struct {
		IsActive bool `json:"isActive"`
		IsPause  bool `json:"isPause"`
	} `json:"queueStatus"`
}

// JobCommand represents a command that can be sent to a job.
type JobCommand string

const (
	JobCommandStart       JobCommand = "start"
	JobCommandPause       JobCommand = "pause"
	JobCommandResume      JobCommand = "resume"
	JobCommandEmpty       JobCommand = "empty"
	JobCommandClearFailed JobCommand = "clear-failed"
)

// JobName represents a named job that can be created.
type JobName string

const (
	JobPersonCleanup JobName = "person-cleanup"
	JobTagCleanup    JobName = "tag-cleanup"
	JobUserCleanup   JobName = "user-cleanup"
)

// JobID represents known job identifiers.
type JobID string

const (
	JobStorageTemplateMigration JobID = "storageTemplateMigration"
)
