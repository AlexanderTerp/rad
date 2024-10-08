package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
)

type Requester struct {
	jsonPathsByMockedUrlRegex map[string]string
}

func NewRequester() *Requester {
	return &Requester{
		jsonPathsByMockedUrlRegex: make(map[string]string),
	}
}

func (r *Requester) AddMockedResponse(urlRegex string, jsonPath string) {
	r.jsonPathsByMockedUrlRegex[urlRegex] = jsonPath
}

func (r *Requester) Request(url string) (string, error) {
	mockJson, ok := r.resolveMockedResponse(url)
	if ok {
		return mockJson, nil
	}

	urlToQuery, err := encodeUrl(url)
	if err != nil {
		return "", err
	}

	RP.RadInfo(fmt.Sprintf("Querying url: %s\n", urlToQuery))

	resp, err := http.Get(urlToQuery)
	if err != nil {
		return "", fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading HTTP body (%v): %w", body, err)
	}

	return string(body), nil
}

func (r *Requester) RequestJson(url string) (interface{}, error) {
	body, err := r.Request(url)
	if err != nil {
		return nil, err
	}

	bodyBytes := []byte(body)
	isValidJson := json.Valid(bodyBytes)
	if !isValidJson {
		return nil, fmt.Errorf("received invalid JSON in response (truncated max 50 chars): [%s]", body[:50])
	}

	var data interface{}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON: %w", err)
	}
	return data, nil
}

// todo test this more, might need additional query param encoding
func encodeUrl(rawUrl string) (string, error) {
	parsedUrl, err := url.Parse(rawUrl)
	if err != nil {
		return "", fmt.Errorf("error parsing URL %v: %w", rawUrl, err)
	}
	return parsedUrl.String(), nil
}

func (r *Requester) resolveMockedResponse(url string) (string, bool) {
	for urlRegex, jsonPath := range r.jsonPathsByMockedUrlRegex {
		re, err := regexp.Compile(urlRegex)
		if err != nil {
			RP.ErrorExit(fmt.Sprintf("Failed to compile mock response regex %q: %v\n", urlRegex, err))
		}

		if re.MatchString(url) {
			RP.RadInfo(fmt.Sprintf("Mocking response for url (matched %q): %s\n", urlRegex, url))
			data := r.loadMockedResponse(jsonPath)
			return data, true
		} else {
			RP.RadDebug(fmt.Sprintf("No match for url %q against regex %q", url, urlRegex))
		}
	}
	return "", false
}

func (r *Requester) loadMockedResponse(path string) string {
	file, err := os.Open(path)
	if err != nil {
		RP.ErrorExit(fmt.Sprintf("Error opening file %s: %v\n", path, err))
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		RP.ErrorExit(fmt.Sprintf("Error reading file %s: %v\n", path, err))
	}
	return string(data)
}
