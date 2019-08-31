package debug

import "github.com/prometheus/client_golang/prometheus"

// MetricRegistrar is used to update values of metrics.
type MetricRegistrar interface {
	Set(name string, value float64, labels ...string)
	Add(name string, delta float64, labels ...string)
	Inc(name string, labels ...string)
	Registry() *prometheus.Registry
}

const (
	NozzleIngressEnvelopesTotal = "metric_store_nozzle_ingress_envelopes_total"
	NozzleDroppedEnvelopesTotal = "metric_store_nozzle_dropped_envelopes_total"
	NozzleDroppedPointsTotal    = "metric_store_nozzle_dropped_points_total"
	NozzleEgressPointsTotal     = "metric_store_nozzle_egress_points_total"
	NozzleEgressErrorsTotal     = "metric_store_nozzle_egress_errors_total"
	NozzleEgressDurationSeconds = "metric_store_nozzle_egress_duration_seconds"

	AuthProxyRequestDurationSeconds     = "metric_store_auth_proxy_request_duration_seconds"
	AuthProxyCAPIRequestDurationSeconds = "metric_store_auth_proxy_capi_request_duration_seconds"

	MetricStoreIngressPointsTotal                   = "metric_store_ingress_points_total"
	MetricStoreWrittenPointsTotal                   = "metric_store_written_points_total"
	MetricStoreWriteDurationSeconds                 = "metric_store_write_duration_seconds"
	MetricStoreDiskFreeRatio                        = "metric_store_disk_free_ratio"
	MetricStoreExpiredShardsTotal                   = "metric_store_expired_shards_total"
	MetricStorePrunedShardsTotal                    = "metric_store_pruned_shards_total"
	MetricStoreStorageDays                          = "metric_store_storage_days"
	MetricStoreIndexSize                            = "metric_store_index_size_bytes"
	MetricStoreSeriesCount                          = "metric_store_series_count"
	MetricStoreMeasurementsCount                    = "metric_store_measurements_count"
	MetricStoreReadErrorsTotal                      = "metric_store_read_errors_total"
	MetricStoreTagValuesQueryDurationSeconds        = "metric_store_tag_values_query_duration_seconds"
	MetricStoreMeasurementNamesQueryDurationSeconds = "metric_store_measurement_names_query_duration_seconds"
)
