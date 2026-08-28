package immich

import "context"

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

type JobID string

const (
	JobStorageTemplateMigration JobID = "storageTemplateMigration"
)

type JobCommand string

const (
	JobCommandStart       JobCommand = "start"
	JobCommandPause       JobCommand = "pause"
	JobCommandResume      JobCommand = "resume"
	JobCommandEmpty       JobCommand = "empty"
	JobCommandClearFailed JobCommand = "clear-failed"
)

type JobName string

const (
	JobPersonCleanup JobName = "person-cleanup"
	JobTagCleanup    JobName = "tag-cleanup"
	JobUserCleanup   JobName = "user-cleanup"
)

func (ic *ImmichClient) GetJobs(ctx context.Context) (map[string]Job, error) {
	var resp map[string]Job
	err := ic.newServerCall(ctx, EndPointGetJobs).
		do(getRequest("/jobs", setAcceptJSON()), responseJSON(&resp))
	return resp, err
}

func (ic *ImmichClient) SendJobCommand(
	ctx context.Context,
	jobID string,
	command JobCommand,
	force bool,
) (resp JobCommandResponse, err error) {
	err = ic.newServerCall(ctx, EndPointSendJobCommand).do(putRequest("/jobs/"+jobID,
		setJSONBody(struct {
			Command JobCommand `json:"command"`
			Force   bool       `json:"force"`
		}{Command: command, Force: force})), responseJSON(&resp))
	return resp, err
}

func (ic *ImmichClient) CreateJob(ctx context.Context, name JobName) error {
	return ic.newServerCall(ctx, EndPointCreateJob).do(postRequest("/jobs",
		"application/json",
		setJSONBody(struct {
			Name JobName `json:"name"`
		}{Name: name})))
}
