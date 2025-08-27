package gmaps

import (
	"net/http"

	"github.com/gosom/scrapemate"
)

// NewPlaceJob cria uma nova instância de PlaceJob
func NewPlaceJob(id, langCode, url string, extractEmail, extractExtraReviews bool, opts ...PlaceJobOptions) *PlaceJob {
	job := PlaceJob{
		Job: scrapemate.Job{
			ID:     id,
			Method: http.MethodGet,
			URL:    url,
			URLParams: map[string]string{
				"hl": langCode,
			},
		},
		UsageInResultststs:  true,
		ExtractEmail:        extractEmail,
		ExtractExtraReviews: extractExtraReviews,
	}
	for _, opt := range opts {
		opt(&job)
	}
	return &job
}
