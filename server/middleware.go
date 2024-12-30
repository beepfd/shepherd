package server

import (
	"github.com/cen-ngc5139/shepherd/internal/cache"
	"github.com/cen-ngc5139/shepherd/internal/output"
	"github.com/gin-gonic/gin"
)

func InitPrometheusMetrics(r *gin.Engine) {
	schedMetrics := output.NewSchedMetrics(cache.SchedMetricsMap, cache.SchedPreemptedMap)
	traceMetrics := output.NewTraceMetrics(schedMetrics)
	r.GET("/metrics", traceMetrics.MetricsHandler())
}
