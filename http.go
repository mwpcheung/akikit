package akikit

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/http2"
	goproxy "golang.org/x/net/proxy"
	"golang.org/x/net/publicsuffix"
)

type Response struct {
	Status  int
	Body    []byte
	Header  *http.Header  // response headers
	Request *http.Request // the actual last submitted request
	Error   error

	Timing Timing
}

type Timing struct {
	Start        time.Time
	DNSStart     time.Time
	DNSDone      time.Time
	ConnectStart time.Time
	ConnectDone  time.Time
	TLSStart     time.Time
	TLSDone      time.Time
	FirstByte    time.Time
	Done         time.Time
}

func (t Timing) Total() time.Duration { return t.Done.Sub(t.Start) }

func (t Timing) TTFB() time.Duration { return t.FirstByte.Sub(t.Start) }

func withTrace(ctx context.Context, t *Timing) context.Context {
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		DNSStart:             func(httptrace.DNSStartInfo) { t.DNSStart = time.Now() },
		DNSDone:              func(httptrace.DNSDoneInfo) { t.DNSDone = time.Now() },
		ConnectStart:         func(string, string) { t.ConnectStart = time.Now() },
		ConnectDone:          func(string, string, error) { t.ConnectDone = time.Now() },
		TLSHandshakeStart:    func() { t.TLSStart = time.Now() },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { t.TLSDone = time.Now() },
		GotFirstResponseByte: func() { t.FirstByte = time.Now() },
	})
}

type HttpUtil struct {
	ConnectTimeout        time.Duration
	HandshakeTimeout      time.Duration
	ResponseHeaderTimeout time.Duration
	ClientCertificate     *tls.Certificate
	Proxy                 string
	Cookie                http.CookieJar
}

func NewHTTPUtil(proxy string) *HttpUtil {
	kls := new(HttpUtil)
	kls.Proxy = proxy
	kls.ConnectTimeout = time.Second * 5
	kls.HandshakeTimeout = time.Second * 30
	kls.ResponseHeaderTimeout = time.Second * 20
	kls.Cookie, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	return kls
}
func NewHTTPUtilWithTimeout(con, handshake, responseHeader time.Duration, proxy string) *HttpUtil {
	kls := new(HttpUtil)
	kls.Proxy = proxy
	kls.ConnectTimeout = con
	kls.HandshakeTimeout = handshake
	kls.ResponseHeaderTimeout = responseHeader
	kls.Cookie, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	return kls
}
func NewHTTPUtilWithClientCert(con, handshake, responseHeader time.Duration, clientCA *tls.Certificate, proxy string) *HttpUtil {
	kls := new(HttpUtil)
	kls.Proxy = proxy
	kls.ConnectTimeout = con
	kls.HandshakeTimeout = handshake
	kls.ResponseHeaderTimeout = responseHeader
	kls.ClientCertificate = clientCA
	kls.Cookie, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	return kls
}
func DefaultTransport() *http.Transport {
	return NewHTTPTransport(time.Second*5, time.Second*20, time.Second*20, nil, "")

}

func DefaultTransportWithProxy(proxy string) *http.Transport {
	return NewHTTPTransport(time.Second*5, time.Second*20, time.Second*20, nil, proxy)
}
func NewHTTPTransport(con, shake, responseHeader time.Duration, certificate *tls.Certificate, proxy string) *http.Transport {

	dial := net.Dialer{
		Timeout:   con,
		KeepAlive: 0,
	}
	config := new(tls.Config)
	config.InsecureSkipVerify = true

	if certificate != nil {
		cert := *certificate
		config.Certificates = []tls.Certificate{cert}
	}

	transport := &http.Transport{
		DialContext:           dial.DialContext,
		TLSHandshakeTimeout:   shake,
		ResponseHeaderTimeout: responseHeader,
		ExpectContinueTimeout: 5 * time.Second,
		TLSClientConfig:       config,
		DisableKeepAlives:     true,
	}
	u, _ := url.Parse(proxy)
	switch u.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(u)
	case "socks5", "socks5h":
		transport.DialContext = ParseSock5(proxy)
	}
	return transport
}
func (kls *HttpUtil) PostForm(uri string, header http.Header, body url.Values) *Response {
	if header == nil {
		header = http.Header{}
	}
	header.Set("Content-Type", "application/x-www-form-urlencoded")
	return kls.SendRequest("POST", uri, header, []byte(body.Encode()))
}

func (kls *HttpUtil) Get(uri string, header http.Header) *Response {
	return kls.SendRequest("GET", uri, header, nil)
}
func (kls *HttpUtil) SendRequestWith302Handling(method string, uri string, header http.Header, data []byte) *Response {

	maxRedirect := 10
	currentURL := uri
	currentMethod := method
	currentData := data
	auth2sv := ""
	var timing Timing
	timing.Start = time.Now()

	for range maxRedirect {
		// build request
		req, err := http.NewRequest(currentMethod, currentURL, bytes.NewBuffer(currentData))
		if err != nil {
			slog.Warn("build request failed", "err", err)
			return &Response{Error: err}
		}
		req.Close = true
		if header != nil {
			req.Header = header.Clone()
		}
		if auth2sv != "" {
			req.Header.Set("X-Apple-I-Cont-x-apple-2sv-authenticate", auth2sv)
		}
		req = req.WithContext(withTrace(req.Context(), &timing))

		// create client
		transport := NewHTTPTransport(kls.ConnectTimeout, kls.HandshakeTimeout, kls.ResponseHeaderTimeout, kls.ClientCertificate, kls.Proxy)
		http2.ConfigureTransport(transport)
		client := &http.Client{
			Transport: transport,
			Jar:       kls.Cookie,
			// disable http auto-redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		resp, err := client.Do(req)
		if err != nil {
			return &Response{Error: err}
		}
		defer resp.Body.Close()

		// handle 3xx redirect
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location := resp.Header.Get("Location")
			if location == "" {
				return &Response{Error: fmt.Errorf("302 received but no Location header")}
			}

			// extract 2sv header
			if val := resp.Header.Get("X-Apple-I-Cont-x-apple-2sv-authenticate"); val != "" {
				auth2sv = val
			}

			// switch to GET (per HTTP spec)
			currentURL = location
			currentMethod = "GET"
			currentData = nil
			continue
		}

		// normal response handling
		var body []byte
		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				return &Response{Error: err}
			}
			defer reader.Close()
			body, err = io.ReadAll(reader)
			if err != nil {
				return &Response{Error: err}
			}
		} else {
			body, err = io.ReadAll(resp.Body)
			if err != nil {
				return &Response{Error: err}
			}
		}

		timing.Done = time.Now()
		return &Response{
			Status:  resp.StatusCode,
			Body:    body,
			Header:  &resp.Header,
			Request: resp.Request,
			Timing:  timing,
		}
	}

	return &Response{Error: fmt.Errorf("too many redirects")}

}

func (kls *HttpUtil) SendRequestViaRedirect(method string, uri string, header http.Header, data []byte, checkRedirect func(req *http.Request, via []*http.Request) error) *Response {
	response := new(Response)
	response.Timing.Start = time.Now()

	transport := NewHTTPTransport(kls.ConnectTimeout, kls.HandshakeTimeout, kls.ResponseHeaderTimeout, kls.ClientCertificate, kls.Proxy)
	http2.ConfigureTransport(transport)

	client := &http.Client{
		Transport:     transport,
		Jar:           kls.Cookie,
		CheckRedirect: checkRedirect, // attach custom redirect handling
	}

	request, err := http.NewRequest(method, uri, bytes.NewBuffer(data))
	if err != nil {
		slog.Warn("build request failed", "err", err)
		return nil
	}
	request = request.WithContext(withTrace(request.Context(), &response.Timing))
	request.Close = true

	if header != nil {
		request.Header = header
	}

	hresp, err := client.Do(request)
	if err != nil {
		response.Error = err
		return response
	}
	defer hresp.Body.Close()

	response.Header = &hresp.Header
	response.Status = hresp.StatusCode
	response.Request = hresp.Request

	switch hresp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err := gzip.NewReader(hresp.Body)
		if err != nil {
			response.Error = err
			return response
		}
		defer reader.Close()
		body, err := io.ReadAll(reader)
		if err != nil {
			response.Error = err
			return response
		}
		response.Body = body
	default:
		body, err := io.ReadAll(hresp.Body)
		if err != nil {
			response.Error = err
			return response
		}
		response.Body = body
	}
	response.Timing.Done = time.Now()
	return response
}
func (kls *HttpUtil) SendRequest(method string, uri string, header http.Header, data []byte) *Response {
	response := new(Response)
	response.Timing.Start = time.Now()
	transport := NewHTTPTransport(kls.ConnectTimeout, kls.HandshakeTimeout, kls.ResponseHeaderTimeout, kls.ClientCertificate, kls.Proxy)
	client := &http.Client{
		Transport: transport,
		Jar:       kls.Cookie,
	}
	var err error
	var hresp *http.Response
	http2.ConfigureTransport(transport)
	request, err := http.NewRequest(method, uri, bytes.NewBuffer(data))
	if err != nil {
		slog.Warn("build request failed", "err", err)
		return nil
	}
	request = request.WithContext(withTrace(request.Context(), &response.Timing))
	request.Close = true
	if header != nil {
		request.Header = header
	}
	hresp, err = client.Do(request)
	if err != nil {
		response.Error = err
		return response
	}
	if hresp != nil {
		defer hresp.Body.Close()
		response.Header = &hresp.Header
		response.Status = hresp.StatusCode
		response.Request = hresp.Request
	}
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err := gzip.NewReader(hresp.Body)
		if err != nil {
			response.Error = err
			return response
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			response.Error = err
			return response
		}
		response.Body = body
	default:
		body, err := io.ReadAll(hresp.Body)
		if err != nil {
			response.Error = err
			return response
		}
		response.Body = body
	}
	response.Timing.Done = time.Now()
	return response
}

func (kls *HttpUtil) PostMultipartForm(uri string, header http.Header, form *RequestMultiForm) *Response {
	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)
	for fieldname, files := range form.File {
		for _, file := range files {
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(fieldname), escapeQuotes(file.Filename)))
			h.Set("Content-Type", file.ContentType)
			part, _ := writer.CreatePart(h)
			part.Write(file.Content)
		}
	}
	for key := range form.Value {
		v := form.Value.Get(key)
		_ = writer.WriteField(key, v)
	}
	writer.Close()
	header.Set("Content-Type", writer.FormDataContentType())
	return kls.SendRequest("POST", uri, header, buf.Bytes())
}

// ParseSock5 http://user:password@localhost:8080
// ParseSock5 socks5://user:password@localhost:1080
func ParseSock5(proxy string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	if proxy == "" {
		dial := net.Dialer{
			Timeout:   20 * time.Second,
			KeepAlive: 0,
		}
		return dial.DialContext
	}
	u, _ := url.Parse(proxy)
	auth := new(goproxy.Auth)
	var pass bool
	auth.User = u.User.Username()
	if auth.Password, pass = u.User.Password(); !pass {
		auth = nil
	}
	dialer, _ := goproxy.SOCKS5("tcp", u.Host, auth, goproxy.Direct)
	if cd, ok := dialer.(goproxy.ContextDialer); ok {
		return cd.DialContext
	}
	return func(_ context.Context, network, addr string) (net.Conn, error) {
		return dialer.Dial(network, addr)
	}
}

func POSTFrom(uri string, proxy string, header http.Header, body url.Values, jar http.CookieJar) *Response {
	if header == nil {
		header = http.Header{}
	}
	header.Set("Content-Type", "application/x-www-form-urlencoded")
	return SendHttpRequest(uri, "POST", proxy, header, []byte(body.Encode()), jar)
}

func HTTPGet(uri string, proxy string, header http.Header, jar http.CookieJar) *Response {
	return SendHttpRequest(uri, "GET", proxy, header, nil, jar)
}

func SendHttpRequest(uri, method, proxy string, header http.Header, data []byte, jar http.CookieJar) *Response {
	response := new(Response)
	response.Timing.Start = time.Now()
	client := &http.Client{
		Jar: jar,
	}
	dial := net.Dialer{
		Timeout:   20 * time.Second,
		KeepAlive: 0,
	}
	transport := &http.Transport{
		DialContext:           dial.DialContext,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives:     true,
	}
	u, _ := url.Parse(proxy)
	switch u.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(u)
	case "socks5", "socks5h":
		transport.DialContext = ParseSock5(proxy)
	}
	client.Transport = transport
	var err error
	var hresp *http.Response
	http2.ConfigureTransport(transport)
	request, err := http.NewRequest(method, uri, bytes.NewBuffer(data))
	if err != nil {
		slog.Warn("build request failed", "err", err)
		return nil
	}
	request = request.WithContext(withTrace(request.Context(), &response.Timing))
	request.Close = true
	if header != nil {
		request.Header = header
	}
	hresp, err = client.Do(request)
	if err != nil {
		response.Error = err
		return response
	}
	if hresp != nil {
		defer hresp.Body.Close()
		response.Header = &hresp.Header
		response.Status = hresp.StatusCode
		response.Request = hresp.Request
	}
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err := gzip.NewReader(hresp.Body)
		if err != nil {
			response.Error = err
			return response
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			response.Error = err
			return response
		}
		response.Body = body
	default:
		body, err := io.ReadAll(hresp.Body)
		if err != nil {
			response.Error = err
			return response
		}
		response.Body = body
	}
	response.Timing.Done = time.Now()
	return response
}

type FileHeader struct {
	Filename    string
	ContentType string
	Size        int64
	Content     []byte
}
type RequestMultiForm struct {
	Value url.Values
	File  map[string][]*FileHeader
}

func NewRequestMultiForm() *RequestMultiForm {
	h := new(RequestMultiForm)
	h.File = make(map[string][]*FileHeader)
	h.Value = url.Values{}
	return h
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func PostMultipartForm(uri, proxy string, header http.Header, jar http.CookieJar, form *RequestMultiForm) *Response {
	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)

	for fieldname, files := range form.File {
		for _, file := range files {
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(fieldname), escapeQuotes(file.Filename)))
			h.Set("Content-Type", file.ContentType)
			part, _ := writer.CreatePart(h)
			part.Write(file.Content)
		}
	}
	for key := range form.Value {
		v := form.Value.Get(key)
		_ = writer.WriteField(key, v)
	}
	writer.Close()
	header.Set("Content-Type", writer.FormDataContentType())
	return SendHttpRequest(uri, "POST", proxy, header, buf.Bytes(), jar)
}
