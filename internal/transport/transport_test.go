package transport

import (
	"net/url"
	"testing"
)

func TestSameOriginNormalizesDefaultPorts(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{name: "HTTPS default port", left: "https://ac.example:443/api/v1", right: "https://ac.example/tasks/22", want: true},
		{name: "HTTP default port", left: "http://ac.example/api/v1", right: "http://ac.example:80/tasks/22", want: true},
		{name: "hostname case", left: "https://AC.EXAMPLE/api/v1", right: "https://ac.example:443/tasks/22", want: true},
		{name: "different explicit port", left: "https://ac.example:8443/api/v1", right: "https://ac.example/tasks/22", want: false},
		{name: "different scheme", left: "https://ac.example/api/v1", right: "http://ac.example:443/tasks/22", want: false},
		{name: "different hostname", left: "https://ac.example/api/v1", right: "https://other.example/tasks/22", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left, err := url.Parse(test.left)
			if err != nil {
				t.Fatal(err)
			}
			right, err := url.Parse(test.right)
			if err != nil {
				t.Fatal(err)
			}
			if got := SameOrigin(left, right); got != test.want {
				t.Fatalf("SameOrigin(%q, %q) = %t, want %t", test.left, test.right, got, test.want)
			}
		})
	}
}
