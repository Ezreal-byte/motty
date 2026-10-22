package agent

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	pem := "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXk\nAAAA\n-----END OPENSSH PRIVATE KEY-----"
	cases := []struct{ name, in, want string }{
		{"pem", "before\n" + pem + "\nafter", "before\n-----BEGIN OPENSSH PRIVATE KEY-----\n[REDACTED]\n[REDACTED]\n-----END OPENSSH PRIVATE KEY-----\nafter"},
		{"pem unterminated", "x\n-----BEGIN RSA PRIVATE KEY-----\nabc\ndef", "x\n-----BEGIN RSA PRIVATE KEY-----\n[REDACTED]\n[REDACTED]"},
		{"cert kept", "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----", "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"},
		{"password eq", "DB_PASSWORD=hunter2 host=a", "DB_PASSWORD=*** host=a"},
		{"passwd colon", "passwd: s3cr3t\nnext", "passwd: ***\nnext"},
		{"quoted", `{"api_key": "abc def", "n": 1}`, `{"api_key": ***, "n": 1}`},
		{"token", "GITHUB_TOKEN=ghp_abc123", "GITHUB_TOKEN=***"},
		{"secret", "client_secret = 'a b'", "client_secret = ***"},
		{"flag", "mysql --password hunter2 db", "mysql --password *** db"},
		{"bearer", "Authorization: Bearer abcdefgh12345.x", "Authorization: Bearer ***"},
		{"basic", "Authorization: Basic dXNlcjpwYXNz", "Authorization: Basic ***"},
		{"aws", "id AKIAIOSFODNN7EXAMPLE end", "id AKIA*** end"},
		{"jwt", "t=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U ok", "t=*** ok"},
		{"url credentials", "postgres://deploy:hunter2@db.internal:5432/prod", "postgres://deploy:***@db.internal:5432/prod"},
		{"url credentials https", "https://user:pass@example.com/path x", "https://user:***@example.com/path x"},
		{"url token only user", "https://token@github.com/repo.git", "https://***@github.com/repo.git"},
		{"url no credentials", "https://example.com/a?b=c and http://host:8080/x", "https://example.com/a?b=c and http://host:8080/x"},
		{"github pat", "curl -H \"Authorization: token ghp_abcdefghijklmnopqrstuvwxyz\" https://api.github.com", "curl -H \"Authorization: token ***\" https://api.github.com"},
		{"slack", "SLACK=https://hooks.slack.com/services/xoxb-123456789-987654321 rest", "SLACK=https://hooks.slack.com/services/*** rest"},
		{"google api key", "key=AIzaSyA1234567890abcdefghijklmnopqrstuv", "key=***"},
		{"stripe live", "stripe charge with sk_live_abcdefghijklmnopqrst", "stripe charge with ***"},
		{"gitlab", "git clone https://oauth2:glpat-abcdefghijklmnopqrst@gitlab.com/r.git", "git clone https://oauth2:***@gitlab.com/r.git"},
		{"flat pem", `{"key":"-----BEGIN PRIVATE KEY-----\nabc\ndef\n-----END PRIVATE KEY-----\n","n":1}`, `{"key":"***\n","n":1}`},
		{"flat pem mid-line", `-----BEGIN OPENSSH PRIVATE KEY-----\nAAAAC3\n-----END OPENSSH PRIVATE KEY----- trailing`, `*** trailing`},
		// no false positives
		{"prose", "The token is valid. Enter your password when prompted.", "The token is valid. Enter your password when prompted."},
		{"prompt", "Password:\nlogin ok", "Password:\nlogin ok"},
		{"max_tokens", "max_tokens: 100\nkeys: 3", "max_tokens: 100\nkeys: 3"},
		{"domain", "see a.b.c and www.example.com", "see a.b.c and www.example.com"},
		{"ls", "drwxr-xr-x 2 root root 4096 Jan 1 passwords_dir", "drwxr-xr-x 2 root root 4096 Jan 1 passwords_dir"},
		{"bearer word", "bearer of bad news", "bearer of bad news"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		got := Redact(c.in)
		if got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
		if strings.Count(got, "\n") != strings.Count(c.in, "\n") {
			t.Errorf("%s: line count changed", c.name)
		}
	}
}

func benchText() string {
	var b strings.Builder
	line := "Jan  1 12:00:00 host sshd[123]: Accepted publickey for root from 10.0.0.1 port 22 ssh2 key fingerprint\n"
	for b.Len() < 1<<20 {
		b.WriteString(line)
		if b.Len()%7 == 0 {
			b.WriteString("export API_KEY=abcdef123456\n")
		}
	}
	return b.String()
}

func BenchmarkRedact1MiB(bm *testing.B) {
	s := benchText()
	bm.SetBytes(int64(len(s)))
	bm.ReportAllocs()
	for i := 0; i < bm.N; i++ {
		Redact(s)
	}
}

func BenchmarkRedactClean1MiB(bm *testing.B) {
	s := strings.Repeat("total 0\ndrwxr-xr-x 2 root root 4096 Jan 1 12:00 etc\n", 21000)[:1<<20]
	bm.SetBytes(int64(len(s)))
	bm.ReportAllocs()
	for i := 0; i < bm.N; i++ {
		Redact(s)
	}
}
