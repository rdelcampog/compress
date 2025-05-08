package compress

/*
gin-compress Copyright (C) 2022 Aurora McGinnis
Modifications Copyright (C) 2025 Rubén del Campo

This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
)

// respWriter wraps the default request writer to allow for compressing the request contents. It uses an internal buffer
// until threshold is hit, at which point it switches to the compressor.
// If threshold is never hit, calling Close() will copy the buffer contents to the response writer
type respWriter struct {
	gin.ResponseWriter
	threshold  int
	encoding   string
	algo       algorithm
	buf        *bytes.Buffer
	compressor io.WriteCloser

	// bytesWritten is the size of the original data written to the response writer
	bytesWritten int
	// compressedBytesCount is the size of the compressed data written to the response writer (if compression was applied)
	compressedBytesCount int
	// metricsHandler is called after a response is written with compression metrics
	metricsHandler MetricsHandler
	// trackingWriter is used for tracking actual writes to the response
	trackingWriter *trackingResponseWriter
}

func newResponseWriter(c *gin.Context, swapSize int, encoding string, algo algorithm, metricsHandler MetricsHandler) *respWriter {
	// Create a tracking writer to count actual bytes written to the response
	tracker := newTrackingResponseWriter(c, nil)

	return &respWriter{
		ResponseWriter: tracker,
		threshold:      swapSize,
		encoding:       encoding,
		algo:           algo,
		buf:            bytes.NewBuffer(nil),
		compressor:     nil,

		bytesWritten:         0,
		compressedBytesCount: 0,
		metricsHandler:       metricsHandler,
		trackingWriter:       tracker,
	}
}

func (rw *respWriter) WriteString(s string) (int, error) {
	return rw.Write([]byte(s))
}

func (rw *respWriter) Write(b []byte) (int, error) {
	rw.Header().Del("Content-Length")

	if !rw.Swapped() && rw.buf.Len()+len(b) >= rw.threshold {
		rw.ResponseWriter.Header().Set("Content-Encoding", rw.encoding)
		rw.ResponseWriter.Header().Set("Vary", "Accept-Encoding")
		rw.compressor = rw.algo.getWriter(rw.ResponseWriter)
		if copied, err := io.Copy(rw.compressor, rw.buf); err != nil {
			return int(copied), err
		}
		rw.buf = nil
	}

	var w io.Writer
	if rw.Swapped() {
		w = rw.compressor
	} else {
		w = rw.buf
	}

	if n, err := w.Write(b); err != nil {
		return n, err
	} else {
		rw.bytesWritten += n
		return n, err
	}
}

func (rw *respWriter) Size() int {
	return rw.bytesWritten
}

func (rw *respWriter) Written() bool {
	return rw.bytesWritten > 0
}

// GetMetricsData returns the metrics data for the response writer. This includes the original size, compressed size,
// whether compression was applied, and the encoding used.
func (rw *respWriter) GetMetricsData() MetricsData {
	return MetricsData{
		OriginalSize:       rw.GetOriginalSize(),
		CompressedSize:     rw.GetCompressedSize(),
		CompressionApplied: rw.GetCompressionApplied(),
		EncodingUsed:       rw.GetEncoding(),
	}
}

// GetCompressionApplied returns true if compression was applied, otherwise false
func (rw *respWriter) GetCompressionApplied() bool {
	return rw.Swapped()
}

// GetEncoding returns the encoding used for compression if compression was applied, otherwise it returns an empty string
func (rw *respWriter) GetEncoding() string {
	if rw.GetCompressionApplied() {
		return rw.encoding
	}
	return ""
}

// GetOriginalSize returns the size of the original data written to the response writer
func (rw *respWriter) GetOriginalSize() int {
	return rw.bytesWritten
}

// GetCompressedSize returns the size of the compressed data if compression was applied, otherwise it returns the original size
func (rw *respWriter) GetCompressedSize() int {
	if rw.GetCompressionApplied() {
		return rw.trackingWriter.bytesWritten
	}
	return rw.bytesWritten
}

func (rw *respWriter) Close() error {
	defer func() {
		if rw.GetCompressionApplied() {
			rw.compressedBytesCount = rw.trackingWriter.bytesWritten
		}
		rw.metricsHandler(rw.GetMetricsData())
	}()

	if !rw.Swapped() {
		// buf was never switched...
		if _, err := io.Copy(rw.ResponseWriter, rw.buf); err != nil {
			return err
		}
	} else {
		return rw.compressor.Close()
	}

	return nil
}

func (rw *respWriter) Swapped() bool {
	return rw.buf == nil
}

func (rw *respWriter) WriteHeader(code int) {
	rw.Header().Del("Content-Length")
	rw.ResponseWriter.WriteHeader(code)
}
