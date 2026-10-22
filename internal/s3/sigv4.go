package s3

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"
)

// AWS Signature Version 4, the part of it that signs one request with its
// headers (not a presigned URL, not chunked uploads). The caller sets the
// headers it wants signed before calling; every header on the request is signed
// except User-Agent and Content-Length, which proxies and the transport are free
// to change.

const (
	algorithm  = "AWS4-HMAC-SHA256"
	dateLayout = "20060102T150405Z"
)

// signV4 sets X-Amz-Date and Authorization on the request. payloadHash is the
// lower case hex SHA-256 of the body (of the empty string for none).
func signV4(request *http.Request, payloadHash, accessKey, secret, region, service string, now time.Time) {
	now = now.UTC()
	stamp := now.Format(dateLayout)
	request.Header.Set("X-Amz-Date", stamp)

	host := request.Host
	if host == "" {
		host = request.URL.Host
	}
	headers := map[string]string{"host": host}
	for name, values := range request.Header {
		lower := strings.ToLower(name)
		if lower == "authorization" || lower == "user-agent" || lower == "content-length" {
			continue
		}
		trimmed := make([]string, len(values))
		for i, value := range values {
			trimmed[i] = strings.Join(strings.Fields(value), " ")
		}
		headers[lower] = strings.Join(trimmed, ",")
	}
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	var canonicalHeaders strings.Builder
	for _, name := range names {
		canonicalHeaders.WriteString(name + ":" + headers[name] + "\n")
	}
	signed := strings.Join(names, ";")

	canonical := strings.Join([]string{
		request.Method,
		request.URL.EscapedPath(),
		canonicalQuery(request),
		canonicalHeaders.String(),
		signed,
		payloadHash,
	}, "\n")

	scope := stamp[:8] + "/" + region + "/" + service + "/aws4_request"
	digest := sha256.Sum256([]byte(canonical))
	toSign := algorithm + "\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(digest[:])

	key := hmacSHA256([]byte("AWS4"+secret), stamp[:8])
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, service)
	key = hmacSHA256(key, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(key, toSign))

	request.Header.Set("Authorization", algorithm+" Credential="+accessKey+"/"+scope+
		", SignedHeaders="+signed+", Signature="+signature)
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func canonicalQuery(request *http.Request) string {
	query := request.URL.Query()
	pairs := make([]string, 0, len(query))
	for name, values := range query {
		for _, value := range values {
			pairs = append(pairs, awsEscape(name)+"="+awsEscape(value))
		}
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "&")
}

// awsEscape percent-encodes everything but the unreserved characters, the way
// SigV4 wants names and values.
func awsEscape(text string) string {
	const hexDigits = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' {
			out.WriteByte(c)
			continue
		}
		out.WriteByte('%')
		out.WriteByte(hexDigits[c>>4])
		out.WriteByte(hexDigits[c&15])
	}
	return out.String()
}
