package s3

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// The expected signatures below are the published ones: the AWS Signature
// Version 4 test suite (get-vanilla, post-vanilla, get-header-value-trim; the
// .authz files of aws-sig-v4-test-suite, as mirrored in botocore) and the worked
// example of the S3 documentation ("Signature Calculations for the
// Authorization Header: GET Object"). They keep the signer honest without
// relying on our own server.

func newSigned(t *testing.T, method, rawURL string, headers map[string]string) *http.Request {
	t.Helper()
	address, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	request := &http.Request{Method: method, URL: address, Header: http.Header{}}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	return request
}

const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestSignerMatchesTheAWSTestSuite(t *testing.T) {
	const (
		key    = "AKIDEXAMPLE"
		secret = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	)
	at := time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)
	cases := []struct {
		name, method string
		headers      map[string]string
		want         string
	}{
		{"get-vanilla", "GET", nil,
			"AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31"},
		{"post-vanilla", "POST", nil,
			"AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5da7c1a2acd57cee7505fc6676e4e544621c30862966e37dddb68e92efbe5d6b"},
		{"get-header-value-trim", "GET", map[string]string{"My-Header1": " value1", "My-Header2": `"a   b   c"`},
			"AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;my-header1;my-header2;x-amz-date, Signature=acc3ed3afb60bb290fc8d2dd0098b9911fcaa05412b367055dee359757a9c736"},
	}
	for _, c := range cases {
		request := newSigned(t, c.method, "https://example.amazonaws.com/", c.headers)
		signV4(request, emptyHash, key, secret, "us-east-1", "service", at)
		if got := request.Header.Get("Authorization"); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

func TestSignerMatchesTheS3DocumentationExample(t *testing.T) {
	request := newSigned(t, "GET", "https://examplebucket.s3.amazonaws.com/test.txt", map[string]string{
		"Range":                "bytes=0-9",
		"X-Amz-Content-Sha256": emptyHash,
	})
	signV4(request, emptyHash, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", "s3",
		time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))
	const want = "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request," +
		" SignedHeaders=host;range;x-amz-content-sha256;x-amz-date," +
		" Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got := request.Header.Get("Authorization"); got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
	if !strings.HasPrefix(request.Header.Get("X-Amz-Date"), "20130524T") {
		t.Fatal("no date header")
	}
}
