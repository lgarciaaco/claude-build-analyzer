package shared

import (
	"cloud.google.com/go/bigquery"
	"time"
)

// BuildRecord represents a build record from BigQuery
type BuildRecord struct {
	Name              string                 `bigquery:"name" json:"name"`
	Group             string                 `bigquery:"group" json:"group"`
	Version           string                 `bigquery:"version" json:"version"`
	Release           string                 `bigquery:"release" json:"release"`
	Assembly          string                 `bigquery:"assembly" json:"assembly"`
	ElTarget          string                 `bigquery:"el_target" json:"el_target"`
	Arches            []string               `bigquery:"arches" json:"arches"`
	StartTime         time.Time              `bigquery:"start_time" json:"start_time"`
	EndTime           bigquery.NullTimestamp `bigquery:"end_time" json:"end_time,omitempty"`
	Outcome           string                 `bigquery:"outcome" json:"outcome"`
	ImagePullspec     string                 `bigquery:"image_pullspec" json:"image_pullspec,omitempty"`
	BuildPipelineURL  string                 `bigquery:"build_pipeline_url" json:"build_pipeline_url,omitempty"`
	ArtJobURL         string                 `bigquery:"art_job_url" json:"art_job_url,omitempty"`
	NVR               string                 `bigquery:"nvr" json:"nvr"`
	BuildID           string                 `bigquery:"build_id" json:"build_id"`
	RecordID          string                 `bigquery:"record_id" json:"record_id"`
	Hermetic          bool                   `bigquery:"hermetic" json:"hermetic"`
	Embargoed         bool                   `bigquery:"embargoed" json:"embargoed"`
	SourceRepo        string                 `bigquery:"source_repo" json:"source_repo,omitempty"`
	Commitish         string                 `bigquery:"commitish" json:"commitish,omitempty"`
	InstalledPackages []string               `bigquery:"installed_packages" json:"installed_packages,omitempty"`
	ParentImages      []string               `bigquery:"parent_images" json:"parent_images,omitempty"`
}

// BuildFailureQuery represents query parameters for build failures
type BuildFailureQuery struct {
	ComponentNames []string `json:"component_names,omitempty"`
	Group          string   `json:"group,omitempty"`
	Assembly       string   `json:"assembly,omitempty"`
	Outcome        string   `json:"outcome,omitempty"`
	Days           int      `json:"days,omitempty"`
	Architectures  []string `json:"architectures,omitempty"`
	Hermetic       *bool    `json:"hermetic,omitempty"`
}

// BuildComparison represents a comparison between two builds
type BuildComparison struct {
	Build1      *BuildRecord `json:"build1"`
	Build2      *BuildRecord `json:"build2"`
	Differences []string     `json:"differences"`
}

// TaskRunRecord represents a TaskRun record from BigQuery
type TaskRunRecord struct {
	CreationTime    time.Time              `bigquery:"creation_time" json:"creation_time"`
	Task            string                 `bigquery:"task" json:"task"`
	TaskRun         string                 `bigquery:"task_run" json:"task_run"`
	TaskRunUID      string                 `bigquery:"task_run_uid" json:"task_run_uid"`
	PipelineRun     string                 `bigquery:"pipeline_run" json:"pipeline_run"`
	PipelineRunUID  string                 `bigquery:"pipeline_run_uid" json:"pipeline_run_uid"`
	PodName         string                 `bigquery:"pod_name" json:"pod_name"`
	PodPhase        string                 `bigquery:"pod_phase" json:"pod_phase"`
	ScheduledTime   bigquery.NullTimestamp `bigquery:"scheduled_time" json:"scheduled_time,omitempty"`
	InitializedTime bigquery.NullTimestamp `bigquery:"initialized_time" json:"initialized_time,omitempty"`
	StartTime       bigquery.NullTimestamp `bigquery:"start_time" json:"start_time,omitempty"`
	MaxFinishedTime bigquery.NullTimestamp `bigquery:"max_finished_time" json:"max_finished_time,omitempty"`
	Containers      []ContainerInfo        `bigquery:"containers" json:"containers"`
	ContainersJSON  string                 `bigquery:"containers_json" json:"-"`
	CaptureTime     time.Time              `bigquery:"capture_time" json:"capture_time"`
	Success         bool                   `bigquery:"success" json:"success"`
	BuildID         string                 `bigquery:"build_id" json:"build_id,omitempty"`
	RecordID        string                 `bigquery:"record_id" json:"record_id,omitempty"`
}

// ContainerInfo represents container information within a TaskRun
type ContainerInfo struct {
	Name         string                 `bigquery:"name" json:"name"`
	IsInit       bool                   `bigquery:"is_init" json:"is_init"`
	Image        string                 `bigquery:"image" json:"image"`
	StartedTime  bigquery.NullTimestamp `bigquery:"started_time" json:"started_time,omitempty"`
	FinishedTime bigquery.NullTimestamp `bigquery:"finished_time" json:"finished_time,omitempty"`
	State        string                 `bigquery:"state" json:"state"`
	ExitCode     bigquery.NullInt64     `bigquery:"exit_code" json:"exit_code,omitempty"`
	Reason       string                 `bigquery:"reason" json:"reason"`
	LogOutput    bigquery.NullString    `bigquery:"log_output" json:"log_output,omitempty"`
}

// GetLogOutput returns the log output as a string, handling null values
func (c *ContainerInfo) GetLogOutput() string {
	if c.LogOutput.Valid {
		return c.LogOutput.StringVal
	}
	return ""
}

// BuildLogAnalysis represents the result of analyzing build logs
type BuildLogAnalysis struct {
	BuildInfo        BuildRecord       `json:"buildInfo"`
	LogUrls          map[string]string `json:"logUrls"`
	Architecture     []string          `json:"architecture"`
	TaskRuns         []TaskRunRecord   `json:"taskRuns,omitempty"`
	FailedContainers []ContainerInfo   `json:"failedContainers,omitempty"`
	ErrorSummary     []string          `json:"errorSummary,omitempty"`
	PossibleIssues   []string          `json:"possibleIssues,omitempty"`
}
