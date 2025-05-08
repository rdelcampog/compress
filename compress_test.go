package compress_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"
	"github.com/klauspost/compress/zstd"
	"github.com/rdelcampog/compress"
	"github.com/stretchr/testify/assert"
)

/*
gin-compress Copyright (C) 2022 Aurora McGinnis
Modifications Copyright (C) 2025 Rubén del Campo

This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/

var smallBody = "SMALL BODY"
var largeBody = strings.Repeat("LARGE BODY", 256)

// MockMetricsHandler is a test implementation of MetricsHandler
type MockMetricsHandler struct {
	CalledWith compress.MetricsData
	Called     bool
}

func (m *MockMetricsHandler) Handle(data compress.MetricsData) {
	m.CalledWith = data
	m.Called = true
}

func setupRouter(opts ...compress.CompressOption) *gin.Engine {
	r := gin.Default()
	r.Use(compress.Compress(opts...))

	r.GET("/small", func(c *gin.Context) {
		c.String(200, smallBody)
	})
	r.GET("/large", func(c *gin.Context) {
		c.String(200, largeBody)
	})
	r.POST("/echo", func(c *gin.Context) {
		c.Header("X-Request-Content-Encoding", c.GetHeader("Content-Encoding"))

		b := bytes.NewBuffer(nil)
		if _, err := io.Copy(b, c.Request.Body); err != nil {
			panic(err)
		}

		c.Data(200, "text/plain", b.Bytes())
	})

	return r
}

func checkNoop(t *testing.T, w *httptest.ResponseRecorder) {
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "", w.Header().Get("Content-Encoding"))
	assert.Equal(t, "", w.Header().Get("Vary"))
}

func checkCompress(t *testing.T, w *httptest.ResponseRecorder, expectedAlgo string) {
	assert.Equal(t, w.Code, 200)
	assert.Equal(t, expectedAlgo, w.Header().Get("Content-Encoding"))
	assert.Equal(t, "Accept-Encoding", w.Header().Get("Vary"))
}

func TestCompressNoopSmall(t *testing.T) {
	req, _ := http.NewRequest("GET", "/small", nil)
	req.Header.Add("Accept-Encoding", "gzip, zstd, br")
	r := setupRouter()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	checkNoop(t, w)

	assert.Equal(t, w.Body.String(), smallBody)
}

func TestCompressNoopNoneAcceptable(t *testing.T) {
	req, _ := http.NewRequest("GET", "/large", nil)
	r := setupRouter()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	checkNoop(t, w)

	assert.Equal(t, w.Body.String(), largeBody)
}

func TestCompressNoopNoneAcceptable2(t *testing.T) {
	req, _ := http.NewRequest("GET", "/large", nil)
	req.Header.Set("Accept-Encoding", "doesnotexist")

	r := setupRouter()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	checkNoop(t, w)

	assert.Equal(t, w.Body.String(), largeBody)
}

func TestCompressGzip(t *testing.T) {
	req, _ := http.NewRequest("GET", "/large", nil)
	req.Header.Add("Accept-Encoding", "gzip")
	r := setupRouter(compress.WithCompressLevel("gzip", compress.GzFlateBestCompression))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	checkCompress(t, w, "gzip")

	gz, err := gzip.NewReader(w.Body)
	assert.NoError(t, err)
	defer gz.Close()

	b := bytes.NewBuffer(nil)
	_, err = gz.WriteTo(b)
	assert.NoError(t, err)
	assert.Equal(t, b.String(), largeBody)
}

func TestCompressBrotli(t *testing.T) {
	req, _ := http.NewRequest("GET", "/large", nil)
	req.Header.Add("Accept-Encoding", "br")
	r := setupRouter()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	checkCompress(t, w, "br")

	br := brotli.NewReader(w.Body)

	b := bytes.NewBuffer(nil)
	if _, err := io.Copy(b, br); err != nil {
		t.Errorf("Decompression failed: %v\n", err)
	}

	assert.Equal(t, b.String(), largeBody)
}

func TestCompressZstd(t *testing.T) {
	req, _ := http.NewRequest("GET", "/large", nil)
	req.Header.Add("Accept-Encoding", "zstd")
	r := setupRouter()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	checkCompress(t, w, "zstd")

	z, err := zstd.NewReader(w.Body)
	assert.NoError(t, err)
	defer z.Close()

	b := bytes.NewBuffer(nil)
	if _, err := io.Copy(b, z); err != nil {
		t.Errorf("Decompression failed: %v\n", err)
	}

	assert.Equal(t, b.String(), largeBody)
}

func TestCompressDeflate(t *testing.T) {
	req, _ := http.NewRequest("GET", "/large", nil)
	req.Header.Add("Accept-Encoding", "deflate")
	r := setupRouter()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	checkCompress(t, w, "deflate")

	z, err := zlib.NewReader(w.Body)
	assert.NoError(t, err)
	defer z.Close()

	b := bytes.NewBuffer(nil)
	if _, err := io.Copy(b, z); err != nil {
		t.Errorf("Decompression failed: %v\n", err)
	}

	assert.Equal(t, b.String(), largeBody)
}

func TestQ(t *testing.T) {
	req, _ := http.NewRequest("GET", "/large", nil)
	req.Header.Add("Accept-Encoding", "br;q=0.5, gzip;q=0.7, deflate;q=0.3")
	r := setupRouter()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	checkCompress(t, w, "gzip")

	gz, err := gzip.NewReader(w.Body)
	assert.NoError(t, err)
	defer gz.Close()

	b := bytes.NewBuffer(nil)
	_, err = gz.WriteTo(b)
	assert.NoError(t, err)
	assert.Equal(t, b.String(), largeBody)
}

// Test for metrics handler when compressing
func TestMetricsHandlerWithCompression(t *testing.T) {
	req, _ := http.NewRequest("GET", "/large", nil)
	req.Header.Add("Accept-Encoding", "gzip")

	mockHandler := &MockMetricsHandler{}
	metricsHandler := mockHandler.Handle

	r := setupRouter(
		compress.WithCompressLevel("gzip", compress.GzFlateBestCompression),
		compress.WithMetricsHandler(metricsHandler),
	)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	checkCompress(t, w, "gzip")

	// Verify metrics handler was called
	assert.True(t, mockHandler.Called, "Metrics handler should have been called")

	// Verify compression was applied
	assert.True(t, mockHandler.CalledWith.CompressionApplied, "Compression should have been applied")

	// Verify the encoding used
	assert.Equal(t, "gzip", mockHandler.CalledWith.EncodingUsed, "Encoding should be gzip")

	// Verify original size
	assert.Equal(t, len(largeBody), mockHandler.CalledWith.OriginalSize, "Original size should match large body length")

	// Verify compressed size is less than original
	assert.Less(t, mockHandler.CalledWith.CompressedSize, mockHandler.CalledWith.OriginalSize, "Compressed size should be less than original")
}

// Test for metrics handler when not compressing (small body)
func TestMetricsHandlerWithoutCompression(t *testing.T) {
	req, _ := http.NewRequest("GET", "/small", nil)
	req.Header.Add("Accept-Encoding", "gzip")

	mockHandler := &MockMetricsHandler{}
	metricsHandler := mockHandler.Handle

	r := setupRouter(compress.WithMetricsHandler(metricsHandler))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	checkNoop(t, w)

	// Verify metrics handler was called
	assert.True(t, mockHandler.Called, "Metrics handler should have been called")

	// Verify compression was not applied
	assert.False(t, mockHandler.CalledWith.CompressionApplied, "Compression should not have been applied")

	// Verify the encoding is empty
	assert.Equal(t, "", mockHandler.CalledWith.EncodingUsed, "Encoding should be empty")

	// Verify original size matches the small body length
	assert.Equal(t, len(smallBody), mockHandler.CalledWith.OriginalSize, "Original size should match small body length")

	// Verify compressed size equals original size when not compressed
	assert.Equal(t, mockHandler.CalledWith.OriginalSize, mockHandler.CalledWith.CompressedSize,
		"Compressed size should equal original size when not compressed")
}

// Test for metrics handler with different compression algorithms
func TestMetricsHandlerWithDifferentAlgorithms(t *testing.T) {
	testCases := []struct {
		name           string
		acceptEncoding string
		expectedAlgo   string
	}{
		{"Gzip", "gzip", "gzip"},
		{"Brotli", "br", "br"},
		{"Zstd", "zstd", "zstd"},
		{"Deflate", "deflate", "deflate"},
		{"Priority", "br, gzip, deflate, zstd", "br"}, // br has the highest priority by default
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/large", nil)
			req.Header.Add("Accept-Encoding", tc.acceptEncoding)

			mockHandler := &MockMetricsHandler{}
			metricsHandler := mockHandler.Handle

			r := setupRouter(compress.WithMetricsHandler(metricsHandler))

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			checkCompress(t, w, tc.expectedAlgo)

			// Verify metrics handler was called
			assert.True(t, mockHandler.Called, "Metrics handler should have been called")

			// Verify compression was applied
			assert.True(t, mockHandler.CalledWith.CompressionApplied, "Compression should have been applied")

			// Verify the correct encoding algorithm was used
			assert.Equal(t, tc.expectedAlgo, mockHandler.CalledWith.EncodingUsed,
				"Expected encoding %s but got %s", tc.expectedAlgo, mockHandler.CalledWith.EncodingUsed)

			// Verify original size
			assert.Equal(t, len(largeBody), mockHandler.CalledWith.OriginalSize, "Original size should match large body length")

			// Verify compressed size is less than original
			assert.Less(t, mockHandler.CalledWith.CompressedSize, mockHandler.CalledWith.OriginalSize,
				"Compressed size should be less than original")
		})
	}
}
