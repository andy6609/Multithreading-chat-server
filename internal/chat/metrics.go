package chat

import "github.com/prometheus/client_golang/prometheus"

var (
	ConnectedClients = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "chat_connected_clients",
		Help: "Number of currently connected clients",
	})

	MessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_messages_total",
		Help: "Total messages processed by type",
	}, []string{"type"})

	EventProcessingDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "chat_event_processing_seconds",
		Help:    "Time to process each event type",
		Buckets: prometheus.DefBuckets,
	}, []string{"type"})
)

func init() {
	prometheus.MustRegister(ConnectedClients)
	prometheus.MustRegister(MessagesTotal)
	prometheus.MustRegister(EventProcessingDuration)
}
