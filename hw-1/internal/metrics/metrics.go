package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	OrdersCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_orders_created_total",
		Help: "Total number of orders successfully created.",
	})

	ReturnsAcceptedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_returns_accepted_total",
		Help: "Total number of return requests successfully accepted.",
	})

	OrdersIssuedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_orders_issued_total",
		Help: "Total number of orders successfully marked as issued.",
	})

	OperationErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "myapp_operation_errors_total",
		Help: "Total number of errors encountered during specific operations.",
	},
		[]string{"operation"},
	)

	OrderCacheItems = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "myapp_order_cache_items",
		Help: "Current number of items in the order cache.",
	})
)
