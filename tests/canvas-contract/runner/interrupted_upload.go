package runner

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

var chunkRequestPath = regexp.MustCompile(`^/api/resources/uploads/[A-Za-z0-9_-]+/chunks/[0-9]+$`)

func (client *Client) DoInterruptedChunk(ctx context.Context, path string, declaredSize int64, prefix []byte) (Response, Envelope, error) {
	origin, err := url.Parse(client.Config.BaseURL)
	if err != nil || origin.Scheme != "http" && origin.Scheme != "https" || origin.Hostname() == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || !chunkRequestPath.MatchString(path) || len(prefix) == 0 || int64(len(prefix)) >= declaredSize {
		return Response{}, Envelope{}, fmt.Errorf("interrupted chunk requires a safe HTTP origin, chunk path and incomplete nonempty body")
	}
	timeout := client.Config.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	requestContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	port := origin.Port()
	if port == "" {
		port = "80"
		if origin.Scheme == "https" {
			port = "443"
		}
	}
	connection, err := (&net.Dialer{Timeout: timeout}).DialContext(requestContext, "tcp", net.JoinHostPort(origin.Hostname(), port))
	if err != nil {
		recordRequest(ctx, http.MethodPut, path, 0, false, true)
		return Response{}, Envelope{}, transportError{cause: err}
	}
	defer connection.Close()
	stopCancellation := context.AfterFunc(requestContext, func() { _ = connection.Close() })
	defer stopCancellation()
	deadline, _ := requestContext.Deadline()
	if err := connection.SetDeadline(deadline); err != nil {
		return Response{}, Envelope{}, transportError{cause: err}
	}
	var stream net.Conn = connection
	if origin.Scheme == "https" {
		configuration := &tls.Config{ServerName: origin.Hostname(), MinVersion: tls.VersionTLS12}
		if transport, ok := client.HTTP.Transport.(*http.Transport); ok && transport.TLSClientConfig != nil {
			configuration = transport.TLSClientConfig.Clone()
			if configuration.InsecureSkipVerify {
				return Response{}, Envelope{}, fmt.Errorf("interrupted chunk requires verified TLS")
			}
			if configuration.ServerName == "" {
				configuration.ServerName = origin.Hostname()
			}
		}
		if configuration.MinVersion < tls.VersionTLS12 {
			configuration.MinVersion = tls.VersionTLS12
		}
		secured := tls.Client(connection, configuration)
		if err := secured.HandshakeContext(requestContext); err != nil {
			recordRequest(ctx, http.MethodPut, path, 0, false, true)
			return Response{}, Envelope{}, transportError{cause: err}
		}
		stream = secured
	}
	request, err := http.NewRequestWithContext(requestContext, http.MethodPut, origin.String()+path, nil)
	if err != nil {
		return Response{}, Envelope{}, fmt.Errorf("invalid interrupted chunk request")
	}
	request.ContentLength = declaredSize
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Length", strconv.FormatInt(declaredSize, 10))
	request.Header.Set("Connection", "close")
	if client.HTTP.Jar != nil {
		for _, cookie := range client.HTTP.Jar.Cookies(request.URL) {
			request.AddCookie(cookie)
		}
	}
	_, err = fmt.Fprintf(stream, "PUT %s HTTP/1.1\r\nHost: %s\r\n", path, origin.Host)
	if err == nil {
		err = request.Header.Write(stream)
	}
	if err == nil {
		_, err = io.WriteString(stream, "\r\n")
	}
	sent := 0
	if err == nil {
		sent, err = stream.Write(prefix)
	}
	if err == nil && sent != len(prefix) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = stream.(interface{ CloseWrite() error }).CloseWrite()
	}
	if err != nil {
		recordRequest(ctx, http.MethodPut, path, 0, false, true)
		return Response{}, Envelope{}, transportError{cause: err}
	}
	faultContext := context.WithValue(ctx, faultContextKey{}, requestFault{ID: "upload-body-half-close", Declared: declaredSize, Sent: sent})
	response, err := http.ReadResponse(bufio.NewReader(stream), request)
	if err != nil {
		recordRequest(faultContext, http.MethodPut, path, 0, false, true)
		return Response{}, Envelope{}, transportError{cause: err}
	}
	result, err := readHTTPResponse(faultContext, http.MethodPut, path, response)
	if err != nil {
		return result, Envelope{}, err
	}
	envelope, err := decodeEnvelope(result)
	return result, envelope, err
}
