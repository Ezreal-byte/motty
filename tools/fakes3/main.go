// Command fakes3 is a tiny S3-compatible bucket in memory for the cloud-sync
// end-to-end script (tools/e2e/cloudsync_e2e.txt). It listens on loopback only,
// serves one path-style bucket named "e2e", requires a SigV4 Authorization
// header for the access key "AKIAE2E" (the signature itself is verified by the
// tests of internal/s3, not here), honours If-Match, and logs every request.
//
//	go run ./tools/fakes3 19000
package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

func main() {
	port := "19000"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	var (
		mu      sync.Mutex
		objects = map[string][]byte{}
	)
	etag := func(body []byte) string { sum := md5.Sum(body); return `"` + hex.EncodeToString(sum[:]) + `"` }
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		status := http.StatusOK
		defer func() { log.Printf("%s %s -> %d", r.Method, r.URL.Path, status) }()
		fail := func(code int, name string) {
			status = code
			w.WriteHeader(code)
			fmt.Fprintf(w, "<Error><Code>%s</Code><Message>%s</Message></Error>", name, name)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=AKIAE2E/") {
			fail(http.StatusForbidden, "InvalidAccessKeyId")
			return
		}
		key, ok := strings.CutPrefix(r.URL.Path, "/e2e/")
		if !ok {
			fail(http.StatusNotFound, "NoSuchBucket")
			return
		}
		body, exists := objects[key]
		switch r.Method {
		case http.MethodPut:
			if match := r.Header.Get("If-Match"); match != "" && (!exists || match != etag(body)) {
				fail(http.StatusPreconditionFailed, "PreconditionFailed")
				return
			}
			data, _ := io.ReadAll(r.Body)
			objects[key] = data
			w.Header().Set("ETag", etag(data))
		case http.MethodGet, http.MethodHead:
			if !exists {
				fail(http.StatusNotFound, "NoSuchKey")
				return
			}
			w.Header().Set("ETag", etag(body))
			if r.Method == http.MethodGet {
				w.Write(body)
			}
		}
	})
	listener, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("fake S3 on %s, bucket e2e", listener.Addr())
	log.Fatal(http.Serve(listener, handler))
}
