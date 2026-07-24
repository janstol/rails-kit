package model

import "testing"

func TestUnderscore(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"S3BucketArchivePolicy", "s3_bucket_archive_policy"},
		{"OrderItem", "order_item"},
		{"APIKey", "api_key"},
		{"User", "user"},
		{"order_item", "order_item"},
		{"Admin::Dashboard", "admin/dashboard"},
	}
	for _, c := range cases {
		got := underscore(c.input)
		if got != c.want {
			t.Errorf("underscore(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
