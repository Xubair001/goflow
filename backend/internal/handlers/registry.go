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
	// UploadBaseURL lets ImageResizeHandler resolve a dashboard upload's
	// ID to a URL it can fetch -- see ImageResizeHandler.UploadBaseURL.
	UploadBaseURL string
}

// Register adds every built-in job type to reg.
func Register(reg *job.Registry, deps Deps) {
	client := defaultClient(deps.HTTPClient)
	mailer := &Mailer{
		Addr:     deps.SMTPAddr,
		From:     deps.SMTPFrom,
		Username: deps.SMTPUsername,
		Password: deps.SMTPPassword,
	}

	reg.Register(EmailJobType, &EmailHandler{Mailer: mailer})
	reg.Register(ImageResizeJobType, &ImageResizeHandler{Client: client, UploadBaseURL: deps.UploadBaseURL})
	reg.Register(CSVJobType, &CSVHandler{Mailer: mailer})
	reg.Register(HTTPRequestJobType, &HTTPRequestHandler{Client: client})
	reg.Register(ReportJobType, &ReportHandler{Store: deps.Store, Mailer: mailer})
	// ScheduledTaskHandler holds reg itself (not a specific Handler) so it
	// can look up its target type lazily at execution time -- by then every
	// handler above has been registered, regardless of registration order.
	reg.Register(ScheduledJobType, &ScheduledTaskHandler{Store: deps.Store, Registry: reg})
}
