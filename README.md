# gin-compress

Middleware for [Gin Gonic](https://github.com/gin-gonic/gin) for compressing HTTP responses
and decompressing HTTP requests.

Currently, this package supports Brotli, GZIP, Deflate, and ZSTD for both compressing and decompressing.

This is a fork of the original [gin-compress](https://github.com/aurowora/compress) package that adds compression metrics functionality. The original package is currently unmaintained.

### Usage

If nothing special is desired, one can add the Compress middleware with the default 
config to your Gin router like so:

```go
package main


import (
	"log"
	"github.com/gin-gonic/gin"
	limits "github.com/gin-contrib/size"
	"github.com/rdelcampog/compress"
)

func main() {
	r := gin.Default()
	r.Use(compress.Compress())
	// Limit payload to 10 MB, notice how it follows Compress() MW.
	// See the "Security" section below...
	r.Use(limits.RequestSizeLimiter(10 * 1024 * 1024)) 
	
	// Declare routes and do whatever else here...
	
	log.Fatalln(r.Run())
}
```

Despite the name, the Compress middleware handles compressing both the response body and decompressing the request body.

To configure the middleware, pass the return value of the functions beginning with `With` 
to the middleware's constructors, like so:

```go
// disables Brotli
r.Use(compress.Compress(compress.WithAlgo(compress.BROTLI, false)))
```

#### Configuration

The following configuration options are available for the Compress middleware:

| Function Signature                           | Default                              | Description                                                                                                                                                           |
|----------------------------------------------|--------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| WithAlgo(algo string, enable bool)           | All enabled                          | Allows enabling/disabling any of the supported algorithms. Valid algorithms are currently `compress.ZSTD`, `compress.BROTLI`, `compress.GZIP`, and `compress.DEFLATE` |
| WithCompressLevel(algo string, level int)    | Default for all algorithms           | Allows setting the compression level for any supported algorithm. See the Brotli*, GzFlate*, and Zstd* constants.                                                     |
| WithPriority(algo string, priority int)      | Order is Brotli, GZIP, Deflate, ZSTD | Specify the priority of an algorithm when the client will accept multiple. Higher priorities win.                                                                     |
| WithExcludeFunc(f func(c *gin.Context) bool) | Not Set                              | Specify a function to be called to determine if the compressor should run. Note that response headers/body is not available at this point.                            |
| WithMinCompressBytes(numBytes int)           | 512                                  | Do not invoke the compressor unless the response body is at least this many bytes                                                                                     |
| WithMaxDecodeSteps(steps int)                | 1                                    | Determines how many rounds of decompression to perform if Content-Encoding includes multiple decompression algorithms.                                                |
| WithDecompressBody(decompress bool)          | true                                 | Specifies whether the request body should be decompressed at all.                                                                                                     |
| WithMetricsHandler(handler MetricsHandler)   | Not Set                              | Provides a handler function to collect compression metrics such as original size, compressed size, compression algorithm used, etc.                                    |

### Security

Bugs/design flaws in the underlying compression algorithm implementations could allow for "zip bombs" that, when
decompressed, expand to massive payloads. This results in high resource usage
and potentially even denial of service. To mitigate this, applications making use of the request body decompression feature
(i.e. WithDecompressBody(true), which is the default), should also use [gin-contrib/size](https://github.com/gin-contrib/size)
(or an equivalent) to limit the size of request bodies.

It is important that any payload size limiter middleware used **come after the compress middleware** as it will be useless otherwise.

An example of correct usage is shown above.

### Tests

Tests cover most functionality in this package. The built-in tests can be run using `go test`.

### License Notice

```
gin-compress Copyright (C) 2022 Aurora McGinnis
Modifications Copyright (C) 2025 Rubén del Campo

This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at https://mozilla.org/MPL/2.0/.
```

The full terms of the Mozilla Public License 2.0 can also be 
found in the LICENSE.txt file within this repository.

The author of this package is not associated with the authors of [Gin](https://github.com/gin-gonic/gin) in any way.

## Metrics Handler

This fork adds the ability to collect metrics about compression operations. You can use the `WithMetricsHandler` option to provide a function that will be called after each response with the following data:

```go
// MetricsData contains information about compression metrics
type MetricsData struct {
    // OriginalSize is the size of the original response in bytes
    OriginalSize int
    // CompressedSize is the size of the compressed response in bytes (if compression was applied)
    CompressedSize int
    // CompressionApplied indicates whether compression was applied
    CompressionApplied bool
    // EncodingUsed is the encoding used for compression (empty if not compressed)
    EncodingUsed string
}
```

Example usage:

```go
r := gin.Default()
r.Use(compress.Compress(
    compress.WithMetricsHandler(func(data compress.MetricsData) {
        // Log or collect metrics
        if data.CompressionApplied {
            log.Printf("Compressed %d bytes to %d bytes using %s (%.2f%% reduction)",
                data.OriginalSize, data.CompressedSize, data.EncodingUsed,
                float64(data.OriginalSize-data.CompressedSize)/float64(data.OriginalSize)*100)
        }
    }),
))
```
