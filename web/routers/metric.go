package routers

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type DataMetric struct {
}

var (
	uploadBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fa_upload_total_bytes",
		Help: "Total bytes uploaded",
	}, []string{"tenantID", "username"})
	downloadBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fa_download_total_bytes",
		Help: "Total bytes uploaded",
	}, []string{"tenantID", "username"})
)

func (dm *DataMetric) Upload(tenantID int64, username string, bytes int) {
	uploadBytes.WithLabelValues(fmt.Sprint(tenantID), username).Add(float64(bytes))
}

func (dm *DataMetric) Download(tenantID int64, username string, bytes int) {
	downloadBytes.WithLabelValues(fmt.Sprint(tenantID), username).Add(float64(bytes))
}
