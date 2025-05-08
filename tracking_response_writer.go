package compress

import "github.com/gin-gonic/gin"

// trackingResponseWriter wraps a gin.ResponseWriter to track how many bytes are written
// and optionally reports metrics
type trackingResponseWriter struct {
	gin.ResponseWriter
	bytesWritten   int
	metricsHandler MetricsHandler
}

// newTrackingResponseWriter creates a new tracking writer that optionally reports metrics
func newTrackingResponseWriter(c *gin.Context, metricsHandler MetricsHandler) *trackingResponseWriter {
	if metricsHandler == nil {
		metricsHandler = noopMetricsHandler
	}

	return &trackingResponseWriter{
		ResponseWriter: c.Writer,
		bytesWritten:   0,
		metricsHandler: metricsHandler,
	}
}

func (tw *trackingResponseWriter) Write(b []byte) (int, error) {
	n, err := tw.ResponseWriter.Write(b)
	tw.bytesWritten += n
	return n, err
}

func (tw *trackingResponseWriter) WriteString(s string) (int, error) {
	return tw.Write([]byte(s))
}

// Close reports metrics if a handler is configured
func (tw *trackingResponseWriter) Close() error {
	tw.metricsHandler(MetricsData{
		OriginalSize:       tw.bytesWritten,
		CompressedSize:     tw.bytesWritten,
		CompressionApplied: false,
		EncodingUsed:       "",
	})
	return nil
}
