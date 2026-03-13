package scanner

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

// DetectHTTP attempts to detect if a port is serving plain HTTP
func DetectHTTP(ip string, port int) bool {
	target := fmt.Sprintf("%s:%d", ip, port)

	// Set a short timeout for detection
	conn, err := net.DialTimeout("tcp", target, 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Set deadline for the entire operation
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	// Send a simple HTTP GET request
	request := fmt.Sprintf("GET / HTTP/1.0\r\nHost: %s\r\n\r\n", ip)
	_, err = conn.Write([]byte(request))
	if err != nil {
		return false
	}

	// Read the response
	reader := bufio.NewReader(conn)

	// Try to read the first line
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	// Check if it looks like an HTTP response
	// Valid HTTP responses start with "HTTP/1.0", "HTTP/1.1", "HTTP/2.0", etc.
	if strings.HasPrefix(firstLine, "HTTP/") {
		return true
	}

	// Also check for common HTTP-like responses
	if strings.Contains(firstLine, "200 OK") ||
	   strings.Contains(firstLine, "404 Not Found") ||
	   strings.Contains(firstLine, "400 Bad Request") ||
	   strings.Contains(firstLine, "403 Forbidden") ||
	   strings.Contains(firstLine, "301 Moved") ||
	   strings.Contains(firstLine, "302 Found") {
		return true
	}

	return false
}
