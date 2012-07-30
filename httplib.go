package httplib

import (
	"bytes"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

var debugprint = false

type Client struct {
	conn    *httputil.ClientConn
	lastURL *url.URL
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func getNopCloser(buf *bytes.Buffer) nopCloser {
	return nopCloser{buf}
}

func hasPort(s string) bool { return strings.LastIndex(s, ":") > strings.LastIndex(s, "]") }

func newConn(url *url.URL) (*httputil.ClientConn, error) {
	addr := url.Host
	if !hasPort(addr) {
		addr += ":" + url.Scheme
	}
	var conn net.Conn
	var err error
	if url.Scheme == "http" {
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
	} else { // https
		conn, err = tls.Dial("tcp", addr, nil)
		if err != nil {
			return nil, err
		}
		h := url.Host
		if hasPort(h) {
			h = h[0:strings.LastIndex(h, ":")]
		}
		if err := conn.(*tls.Conn).VerifyHostname(h); err != nil {
			return nil, err
		}
	}

	return httputil.NewClientConn(conn, nil), nil
}

func getResponse(rawUrl string, req *http.Request) (*httputil.ClientConn, *http.Response, error) {
	url, err := url.Parse(rawUrl)
	if err != nil {
		return nil, nil, err
	}
	req.URL = url
	if debugprint {
		dump, _ := httputil.DumpRequest(req, true)
		print(string(dump))
	}

	conn, err := newConn(url)
	if err != nil {
		return nil, nil, err
	}

	resp, err := conn.Do(req)
	if err != nil {
		if err != httputil.ErrPersistEOF {
			return nil, nil, err
		}
	}
	return conn, resp, nil
}

func Get(targeturl string) *HttpRequestBuilder {
	var req http.Request
	req.Method = "GET"
	req.Header = http.Header{}
	return &HttpRequestBuilder{url: targeturl, req: &req, params: &url.Values{}}
}

func Post(targeturl string) *HttpRequestBuilder {
	var req http.Request
	req.Method = "POST"
	req.Header = http.Header{}
	return &HttpRequestBuilder{url: targeturl, req: &req, params: &url.Values{}}
}

func Put(targeturl string) *HttpRequestBuilder {
	var req http.Request
	req.Method = "PUT"
	req.Header = http.Header{}
	return &HttpRequestBuilder{url: targeturl, req: &req, params: &url.Values{}}
}

func Delete(targeturl string) *HttpRequestBuilder {
	var req http.Request
	req.Method = "DELETE"
	req.Header = http.Header{}
	return &HttpRequestBuilder{url: targeturl, req: &req, params: &url.Values{}}
}

type HttpRequestBuilder struct {
	url        string
	req        *http.Request
	clientConn *httputil.ClientConn
	params     *url.Values
}

func (b *HttpRequestBuilder) getResponse() (*http.Response, error) {
	paramBody := b.params.Encode()
	//paramBody = paramBody[0 : len(paramBody)-1]
	if b.req.Method == "GET" && len(paramBody) > 0 {
		if strings.Index(b.url, "?") != -1 {
			b.url += "&" + paramBody
		} else {
			b.url = b.url + "?" + paramBody
		}
	} else if b.req.Method == "POST" && b.req.Body == nil && len(paramBody) > 0 {
		b.Header("Content-Type", "application/x-www-form-urlencoded")
		b.req.Body = nopCloser{bytes.NewBufferString(paramBody)}
		b.req.ContentLength = int64(len(paramBody))
	}

	conn, resp, err := getResponse(b.url, b.req)
	b.clientConn = conn
	return resp, err
}

func (b *HttpRequestBuilder) Header(key, value string) *HttpRequestBuilder {
	b.req.Header.Set(key, value)
	return b
}

func (b *HttpRequestBuilder) Param(key, value string) *HttpRequestBuilder {
	b.params.Set(key, value)
	return b
}

func (b *HttpRequestBuilder) Body(data interface{}) *HttpRequestBuilder {
	switch t := data.(type) {
	case string:
		b.req.Body = getNopCloser(bytes.NewBufferString(t))
		b.req.ContentLength = int64(len(t))
	case []byte:
		b.req.Body = getNopCloser(bytes.NewBuffer(t))
		b.req.ContentLength = int64(len(t))
	}
	return b
}

func (b *HttpRequestBuilder) AsString() (string, error) {
	resp, err := b.getResponse()
	if err != nil {
		return "", err
	}
	if resp.Body == nil {
		return "", nil
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (b *HttpRequestBuilder) AsBytes() ([]byte, error) {
	resp, err := b.getResponse()
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		return nil, nil
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (b *HttpRequestBuilder) AsFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	resp, err := b.getResponse()
	if err != nil {
		return err
	}
	if resp.Body == nil {
		return nil
	}
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func (b *HttpRequestBuilder) AsResponse() (*http.Response, error) {
	return b.getResponse()
}

func (b *HttpRequestBuilder) Close() {
	if b.clientConn != nil {
		b.clientConn.Close()
	}
}
