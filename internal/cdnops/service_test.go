package cdnops

import (
	"testing"

	sdk "github.com/alibabacloud-go/cdn-20180510/v5/client"
	"github.com/alibabacloud-go/tea/tea"
)

func TestConfigured(t *testing.T) {
	if NewService("", "").Configured() {
		t.Fatal("empty AK/SK must be unconfigured")
	}
	if NewService("ak", "").Configured() {
		t.Fatal("missing SK must be unconfigured")
	}
	if !NewService("ak", "sk").Configured() {
		t.Fatal("both set must be configured")
	}
}

func TestPageToView(t *testing.T) {
	d := &sdk.DescribeUserDomainsResponseBodyDomainsPageData{
		DomainName:   tea.String("cdn.example.com"),
		Cname:        tea.String("cdn.example.com.w.kunlunsl.com"),
		DomainStatus: tea.String("online"),
		SslProtocol:  tea.String("on"),
		GmtCreated:   tea.String("2024-01-01T00:00:00Z"),
		Sources: &sdk.DescribeUserDomainsResponseBodyDomainsPageDataSources{
			Source: []*sdk.DescribeUserDomainsResponseBodyDomainsPageDataSourcesSource{
				{Type: tea.String("oss"), Content: tea.String("bucket.oss-cn-hangzhou.aliyuncs.com"), Port: tea.Int32(443)},
			},
		},
	}
	certNames := map[string]string{"cdn.example.com": "my-cert"}
	v := pageToView(d, certNames)
	if v.DomainName != "cdn.example.com" || v.CertName != "my-cert" {
		t.Fatalf("view = %+v", v)
	}
	if v.SourceType != "oss" || v.SourceContent != "bucket.oss-cn-hangzhou.aliyuncs.com" || v.SourcePort != 443 {
		t.Fatalf("source = %+v", v)
	}
}

func TestPageToViewNilSources(t *testing.T) {
	// Sources 为 nil / 空列表都不能 panic,源站字段留空
	for _, srcs := range []*sdk.DescribeUserDomainsResponseBodyDomainsPageDataSources{
		nil,
		{Source: nil},
	} {
		d := &sdk.DescribeUserDomainsResponseBodyDomainsPageData{
			DomainName: tea.String("d.example.com"),
			Sources:    srcs,
		}
		v := pageToView(d, map[string]string{})
		if v.SourceType != "" || v.SourceContent != "" || v.SourcePort != 0 {
			t.Fatalf("empty sources must yield zero fields: %+v", v)
		}
		if v.CertName != "" {
			t.Fatalf("no cert mapping must yield empty certName: %+v", v)
		}
	}
}
