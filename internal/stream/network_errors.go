package stream

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"syscall"
)

// isNetworkError reports dial/timeouts/connection failures that should keep retrying.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() || urlErr.Err != nil {
			return true
		}
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.ECONNABORTED,
			syscall.ETIMEDOUT, syscall.EHOSTUNREACH, syscall.ENETUNREACH:
			return true
		}
	}

	msg := strings.ToLower(err.Error())
	hints := []string{
		"connection refused", "connection reset", "connection aborted",
		"no such host", "network is unreachable", "host unreachable",
		"i/o timeout", "read tcp", "write tcp", "dial tcp", "dial udp",
		"tls handshake", "broken pipe", "unexpected eof", "eof",
		"temporary failure in name resolution", "no route to host",
		"network error", "timeout awaiting response", "client.timeout exceeded",
	}
	for _, h := range hints {
		if strings.Contains(msg, h) {
			return true
		}
	}
	return false
}
