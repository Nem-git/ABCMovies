package keys

import (
	"testing"
)

var normalPssh = []struct {
	pssh    string
	url     string
	headers map[string]string
}{
	{"AAAAOHBzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAABgSEAAWNwaftdGsPEdH4BMi5MJI49yVmwY=", "https://lic.drmtoday.com/license-proxy-widevine/cenc/?specConform=true", map[string]string{"x-dt-auth-token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJvcHREYXRhIjoie1widXNlcklkXCI6XCJyZW50YWwxXCIsXCJzZXNzaW9uSWRcIjpcImRlZmF1bHRcIixcIm1lcmNoYW50XCI6XCJjYW5hZGlhbl9icm9hZGNhc3RpbmdcIn0iLCJjcnQiOiJbe1wicmVmXCI6W1wiM2YzMmIwYmItZDNlOC00Y2MyLTg5YmYtZTE3NGMzOGY5YjczXCIsXCJlZDdlOWZlOS01NGU3LTQwZmItYTUxNC01YTUwNjg3YTk4NjRcIl0sXCJhY2NvdW50aW5nSWRcIjpcIlRPVVRWXCIsXCJhc3NldElkXCI6XCJzcmNfbGFnZW50amVhbl9zMDNlMDFcIixcInByb2ZpbGVcIjp7XCJyZW50YWxcIjp7XCJyZWxhdGl2ZUV4cGlyYXRpb25cIjpcIlBUMTJIXCIsXCJwbGF5RHVyYXRpb25cIjo0MzIwMDAwMH19LFwic3RvcmVMaWNlbnNlXCI6ZmFsc2UsXCJvdXRwdXRQcm90ZWN0aW9uXCI6e1wiZGlnaXRhbFwiOnRydWUsXCJhbmFsb2d1ZVwiOnRydWUsXCJlbmZvcmNlXCI6dHJ1ZX19XSIsImlhdCI6MTc1ODMyODg5NiwianRpIjoiZnAvREZHbEVjMDZxc1hhQ0ZoYTE5QT09In0.UaHUdYShL-NlFechsErItfQ5muUPhkQiXwnysFMGLRr8pqgrRJ4pfnn8jcT9ysoHFuu35h0idAQscCtPjlwaSQ"}},
}

func TestPssh(t *testing.T) {
	for _, o := range normalPssh {
		keys, err := Get(o.pssh, o.url, o.headers)
		if err != nil {
			t.Errorf(`Get(%v, %v, %v) = %q, %v`, o.pssh, o.url, o.headers, err, keys)
		}
	}
}
