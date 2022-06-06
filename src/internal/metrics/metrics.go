package metrics

import "github.com/prometheus/client_golang/prometheus"

// Registrar is used to update values of metrics.
type Registrar interface {
	Set(name string, value float64, labels ...string)
	Add(name string, delta float64, labels ...string)
	Inc(name string, labels ...string)
	Histogram(name string, labels ...string) prometheus.Observer
	Registerer() prometheus.Registerer
	Gatherer() prometheus.Gatherer
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
	MetricStoreSeriesCount                          = "metric_store_series_count"
	MetricStoreMeasurementsCount                    = "metric_store_measurements_count"
	MetricStoreReadErrorsTotal                      = "metric_store_read_errors_total"
	MetricStoreTagValuesQueryDurationSeconds        = "metric_store_tag_values_query_duration_seconds"
	MetricStoreMeasurementNamesQueryDurationSeconds = "metric_store_measurement_names_query_duration_seconds"
	MetricStoreReplayerDiskUsageBytes               = "metric_store_replayer_disk_usage_bytes"
	MetricStoreReplayerQueueErrorsTotal             = "metric_store_replayer_queue_errors_total"
	MetricStoreReplayerQueuedBytesTotal             = "metric_store_replayer_queued_bytes_total"
	MetricStoreReplayerReadErrorsTotal              = "metric_store_replayer_read_errors_total"
	MetricStoreReplayerReplayErrorsTotal            = "metric_store_replayer_replay_errors_total"
	MetricStoreReplayerReplayedBytesTotal           = "metric_store_replayer_replayed_bytes_total"
	MetricStoreDroppedPointsTotal                   = "metric_store_dropped_points_total"
	MetricStoreDistributedPointsTotal               = "metric_store_distributed_points_total"
	MetricStoreDistributedRequestDurationSeconds    = "metric_store_distributed_request_duration_seconds"
	MetricStoreCollectedPointsTotal                 = "metric_store_collected_points_total"
	MetricStorePendingDeletionDroppedPointsTotal    = "metric_store_pending_deletion_dropped_points_total"
)
