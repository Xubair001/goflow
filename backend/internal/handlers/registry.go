package handlers

import (
	"net/http"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// Deps are the external dependencies the built-in handlers need, gathered
// here so wiring them into a job.Registry stays a one-line call in
// cmd/worker regardless of how many handler types get added.
type Deps struct {
	Store        store.Store
	HTTPClient   *http.Client
	SMTPAddr     string
	SMTPFrom     string
	SMTPUsername string
	SMTPPassword string
}

// Register adds every built-in job type to reg.
func Register(reg *job.Registry, deps Deps) {
	client := defaultClient(deps.HTTPClient)

	reg.Register(EmailJobType, &EmailHandler{
		Addr:     deps.SMTPAddr,
		From:     deps.SMTPFrom,
		Username: deps.SMTPUsername,
		Password: deps.SMTPPassword,
	})
	reg.Register(ImageResizeJobType, &ImageResizeHandler{Client: client})
	reg.Register(CSVJobType, &CSVHandler{})
	reg.Register(HTTPRequestJobType, &HTTPRequestHandler{Client: client})
	reg.Register(ReportJobType, &ReportHandler{Store: deps.Store})
	reg.Register(ScheduledJobType, &ScheduledTaskHandler{Store: deps.Store})
}
